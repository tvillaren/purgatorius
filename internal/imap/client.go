// Package imap est un client IMAP minimal, taille pour Gmail (extensions
// X-GM-EXT-1 : X-GM-LABELS, X-GM-MSGID, X-GM-THRID, X-GM-RAW). Bibliotheque
// standard uniquement : go-imap v2 ne gere pas ces extensions.
//
// La lecture ne modifie pas la boite : EXAMINE (et non SELECT) et BODY.PEEK
// (le drapeau \Seen n'est pas pose). Seules les commandes de ecriture.go
// (SELECT, UID MOVE) la modifient, pour la mise a la corbeille.
package imap

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// HoteGmail : serveur IMAP de Gmail (port 993, TLS implicite).
const HoteGmail = "imap.gmail.com"

// ErrAuthentification : identifiants refuses par le serveur.
var ErrAuthentification = errors.New("identifiants refuses par le serveur")

// Client : une connexion IMAP authentifiable. Non concurrent : une commande a
// la fois.
type Client struct {
	conn    net.Conn
	cpt     *compteur
	l       *lecteur
	w       *bufio.Writer
	n       int
	delai   time.Duration
	arret   func() bool
	caps    map[string]bool
	utf8    bool
	Accueil string // ligne d'accueil du serveur
}

// Connecter ouvre une connexion TLS vers hote:993 et lit l'accueil. La
// connexion est fermee si ctx est annule.
func Connecter(ctx context.Context, hote string) (*Client, error) {
	d := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 20 * time.Second}, Config: &tls.Config{ServerName: hote, MinVersion: tls.VersionTLS12}}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(hote, "993"))
	if err != nil {
		return nil, fmt.Errorf("connexion a %s : %w", hote, err)
	}
	c, err := nouveauClient(conn)
	if err != nil {
		return nil, err
	}
	c.arret = context.AfterFunc(ctx, func() { conn.Close() })
	return c, nil
}

// SurConnexion enveloppe une connexion deja etablie, sans TLS : reserve aux
// tests (faux serveur sur net.Pipe, paquet imaptest). L'application passe
// toujours par Connecter.
func SurConnexion(conn net.Conn) (*Client, error) { return nouveauClient(conn) }

func nouveauClient(conn net.Conn) (*Client, error) {
	cpt := &compteur{r: conn}
	c := &Client{
		conn:  conn,
		cpt:   cpt,
		l:     &lecteur{bufio.NewReaderSize(cpt, 1<<20)},
		w:     bufio.NewWriter(conn),
		delai: 10 * time.Minute,
		caps:  map[string]bool{},
	}
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	_, elems, err := c.l.reponse()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("accueil du serveur : %w", err)
	}
	if len(elems) >= 2 {
		c.Accueil = elems[1].s
		if strings.EqualFold(elems[0].s, "BYE") {
			conn.Close()
			return nil, fmt.Errorf("serveur indisponible : %s", c.Accueil)
		}
	}
	return c, nil
}

// Octets : volume recu depuis la connexion.
func (c *Client) Octets() int64 { return c.cpt.n }

// Fermer coupe la connexion sans LOGOUT.
func (c *Client) Fermer() error {
	if c.arret != nil {
		c.arret()
	}
	return c.conn.Close()
}

// Deconnecter envoie LOGOUT puis ferme.
func (c *Client) Deconnecter() error {
	_, _ = c.commande("LOGOUT", nil)
	return c.Fermer()
}

// commande envoie cmd et passe chaque reponse non etiquetee a rappel. Les
// erreurs ne citent que le verbe de la commande : LOGIN porte le mot de passe.
func (c *Client) commande(cmd string, rappel func([]noeud)) (string, error) {
	c.n++
	tag := "p" + strconv.Itoa(c.n)
	verbe, _, _ := strings.Cut(cmd, " ")
	if verbe == "UID" {
		v2, _, _ := strings.Cut(strings.TrimPrefix(cmd, "UID "), " ")
		verbe += " " + v2
	}
	c.conn.SetDeadline(time.Now().Add(c.delai))
	fmt.Fprintf(c.w, "%s %s\r\n", tag, cmd)
	if err := c.w.Flush(); err != nil {
		return "", fmt.Errorf("%s : envoi : %w", verbe, err)
	}
	for {
		t, elems, err := c.l.reponse()
		if err != nil {
			return "", fmt.Errorf("%s : lecture : %w", verbe, err)
		}
		if t == "*" {
			if len(elems) >= 2 && strings.EqualFold(elems[0].s, "CAPABILITY") {
				c.notercapacites(elems[1:])
			}
			if rappel != nil {
				rappel(elems)
			}
			continue
		}
		if t != tag || len(elems) < 2 {
			continue
		}
		if !strings.EqualFold(elems[0].s, "OK") {
			return elems[1].s, &ErreurServeur{Commande: verbe, Statut: strings.ToUpper(elems[0].s), Texte: elems[1].s}
		}
		return elems[1].s, nil
	}
}

// ErreurServeur : reponse NO ou BAD du serveur.
type ErreurServeur struct {
	Commande, Statut, Texte string
}

func (e *ErreurServeur) Error() string {
	return e.Commande + " : " + e.Statut + " " + e.Texte
}

func (c *Client) notercapacites(l []noeud) {
	c.caps = map[string]bool{}
	for _, x := range l {
		c.caps[strings.ToUpper(x.s)] = true
	}
}

// Capacite indique si le serveur annonce cap (apres Authentifier, la liste
// est celle de la session authentifiee).
func (c *Client) Capacite(cap string) bool { return c.caps[strings.ToUpper(cap)] }

// citer produit une chaine IMAP entre guillemets.
func citer(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// Authentifier envoie LOGIN (mot de passe d'application), recharge les
// capacites et active UTF8=ACCEPT quand le serveur le permet (noms de dossiers
// et de libelles en UTF-8 plutot qu'en UTF-7 modifie).
func (c *Client) Authentifier(adresse, motDePasse string) error {
	motDePasse = strings.ReplaceAll(motDePasse, " ", "")
	if strings.ContainsAny(adresse+motDePasse, "\r\n") {
		return errors.New("identifiants invalides")
	}
	if _, err := c.commande("LOGIN "+citer(adresse)+" "+citer(motDePasse), nil); err != nil {
		var es *ErreurServeur
		if errors.As(err, &es) {
			// Le texte vient du serveur : s'il recopiait la commande, il
			// porterait le mot de passe, affiche ensuite dans l'interface.
			es.Texte = masquer(es.Texte, motDePasse)
			if es.Statut == "NO" {
				return fmt.Errorf("%w (%s)", ErrAuthentification, es.Texte)
			}
		}
		return err
	}
	if _, err := c.commande("CAPABILITY", nil); err != nil {
		return err
	}
	if c.Capacite("ENABLE") && c.Capacite("UTF8=ACCEPT") {
		if _, err := c.commande("ENABLE UTF8=ACCEPT", nil); err == nil {
			c.utf8 = true
		}
	}
	return nil
}

// masquer remplace le secret (et ses groupes de 4, forme affichee par Google)
// dans un texte venu du serveur.
func masquer(texte, secret string) string {
	if secret == "" {
		return texte
	}
	texte = strings.ReplaceAll(texte, secret, "***")
	for i := 0; i+4 <= len(secret); i += 4 {
		texte = strings.ReplaceAll(texte, secret[i:i+4], "***")
	}
	return texte
}

// Dossier : un dossier (libelle Gmail) et ses attributs (\All, \Trash...).
type Dossier struct {
	Nom       string
	Attributs []string
}

// Special indique si le dossier porte l'attribut special-use attr (\All...).
func (d Dossier) Special(attr string) bool {
	for _, a := range d.Attributs {
		if strings.EqualFold(a, attr) {
			return true
		}
	}
	return false
}

// Dossiers liste tous les dossiers.
func (c *Client) Dossiers() ([]Dossier, error) {
	var out []Dossier
	_, err := c.commande(`LIST "" "*"`, func(e []noeud) {
		if len(e) < 4 || !strings.EqualFold(e[0].s, "LIST") {
			return
		}
		d := Dossier{Nom: c.nomDossier(e[3].s)}
		for _, a := range e[1].l {
			d.Attributs = append(d.Attributs, a.s)
		}
		out = append(out, d)
	})
	return out, err
}

func (c *Client) nomDossier(s string) string {
	if c.utf8 {
		return s
	}
	return decoderUTF7Modifie(s)
}

// nomVersServeur reencode un nom de dossier pour une commande.
func (c *Client) nomVersServeur(s string) string {
	if c.utf8 {
		return s
	}
	return encoderUTF7Modifie(s)
}

// Etat : etat d'un dossier ouvert.
type Etat struct {
	Existe        int
	UIDValidity   uint32
	HighestModSeq uint64 // 0 si CONDSTORE indisponible
}

var (
	reUIDValidity = regexp.MustCompile(`\[UIDVALIDITY (\d+)\]`)
	reModSeq      = regexp.MustCompile(`\[HIGHESTMODSEQ (\d+)\]`)
)

// Examiner ouvre un dossier en LECTURE SEULE (EXAMINE), avec CONDSTORE quand
// le serveur le permet.
func (c *Client) Examiner(nom string) (Etat, error) {
	var et Etat
	cmd := "EXAMINE " + citer(c.nomVersServeur(nom))
	if c.Capacite("CONDSTORE") {
		cmd += " (CONDSTORE)"
	}
	_, err := c.commande(cmd, func(e []noeud) {
		if len(e) >= 2 && strings.EqualFold(e[1].s, "EXISTS") {
			et.Existe, _ = strconv.Atoi(e[0].s)
		}
		if len(e) >= 2 && estStatut(e[0].s) {
			if m := reUIDValidity.FindStringSubmatch(e[1].s); m != nil {
				v, _ := strconv.ParseUint(m[1], 10, 32)
				et.UIDValidity = uint32(v)
			}
			if m := reModSeq.FindStringSubmatch(e[1].s); m != nil {
				et.HighestModSeq, _ = strconv.ParseUint(m[1], 10, 64)
			}
		}
	})
	return et, err
}

// NombreMessages : STATUS (MESSAGES) d'un dossier, sans l'ouvrir.
func (c *Client) NombreMessages(nom string) (int, error) {
	n := -1
	_, err := c.commande("STATUS "+citer(c.nomVersServeur(nom))+" (MESSAGES)", func(e []noeud) {
		if len(e) >= 3 && strings.EqualFold(e[0].s, "STATUS") {
			for i := 0; i+1 < len(e[2].l); i += 2 {
				if strings.EqualFold(e[2].l[i].s, "MESSAGES") {
					n, _ = strconv.Atoi(e[2].l[i+1].s)
				}
			}
		}
	})
	return n, err
}

// RechercherUID renvoie les UID du dossier ouvert qui satisfont critere
// (syntaxe IMAP, ex. "ALL").
func (c *Client) RechercherUID(critere string) ([]uint32, error) {
	var uids []uint32
	_, err := c.commande("UID SEARCH "+critere, func(e []noeud) {
		if len(e) > 0 && strings.EqualFold(e[0].s, "SEARCH") {
			for _, x := range e[1:] {
				if v, err := strconv.ParseUint(x.s, 10, 32); err == nil {
					uids = append(uids, uint32(v))
				}
			}
		}
	})
	return uids, err
}

// RechercherGmail : recherche avec la syntaxe de Gmail (X-GM-RAW), ex.
// "category:promotions".
func (c *Client) RechercherGmail(requete string) ([]uint32, error) {
	if !c.Capacite("X-GM-EXT-1") {
		return nil, errors.New("extension Gmail X-GM-EXT-1 absente")
	}
	return c.RechercherUID("X-GM-RAW " + citer(requete))
}

// elementsFetch : ce qu'on lit de chaque message. BODY.PEEK ne marque pas le
// message comme lu.
const elementsFetch = "(UID RFC822.SIZE INTERNALDATE FLAGS X-GM-MSGID X-GM-THRID X-GM-LABELS BODYSTRUCTURE BODY.PEEK[HEADER.FIELDS (FROM SUBJECT TO CC)])"

// Recuperer lit les metadonnees des messages d'UID dans ensemble (syntaxe IMAP,
// ex. "1:500" ou "3,7,9") et appelle rappel pour chacun.
//
// Le serveur peut glisser parmi les reponses des FETCH non sollicites (un
// message marque lu ailleurs : « * 4 FETCH (UID 8 FLAGS (\Seen)) »). Seules
// les reponses completes (UID, taille et X-GM-MSGID non nul) sont transmises :
// une reponse partielle ecraserait le message en cache avec des champs vides.
func (c *Client) Recuperer(ensemble string, rappel func(Message)) error {
	_, err := c.commande("UID FETCH "+ensemble+" "+elementsFetch, func(e []noeud) {
		if len(e) >= 3 && strings.EqualFold(e[1].s, "FETCH") && e[2].genre == 'l' && fetchComplet(e[2].l) {
			if m := analyserFetch(e[2].l, c.utf8); m.UID != 0 && m.GmMsgID != 0 {
				rappel(m)
			}
		}
	})
	return err
}

// fetchComplet : la reponse porte les elements sans lesquels un message ne
// peut pas etre enregistre.
func fetchComplet(paires []noeud) bool { return porte(paires, "UID", "RFC822.SIZE", "X-GM-MSGID") }

// porte : la reponse FETCH contient chacun des elements cles.
func porte(paires []noeud, cles ...string) bool {
	vus := 0
	for _, cle := range cles {
		for i := 0; i+1 < len(paires); i += 2 {
			if strings.EqualFold(paires[i].s, cle) {
				vus++
				break
			}
		}
	}
	return vus == len(cles)
}

// Changements lit les drapeaux et libelles des messages modifies depuis le
// compteur CONDSTORE depuis (HIGHESTMODSEQ d'une synchro precedente) et
// appelle rappel pour chacun (Message partiel : UID, Lu, Libelles). Le dossier
// doit avoir ete ouvert par Examiner avec CONDSTORE. Lecture seule : FETCH ne
// lit que FLAGS et X-GM-LABELS.
func (c *Client) Changements(depuis uint64, rappel func(Message)) error {
	if !c.Capacite("CONDSTORE") {
		return errors.New("extension CONDSTORE absente")
	}
	cmd := "UID FETCH 1:* (UID FLAGS X-GM-LABELS) (CHANGEDSINCE " + strconv.FormatUint(depuis, 10) + ")"
	_, err := c.commande(cmd, func(e []noeud) {
		if len(e) >= 3 && strings.EqualFold(e[1].s, "FETCH") && e[2].genre == 'l' && porte(e[2].l, "UID", "FLAGS", "X-GM-LABELS") {
			if m := analyserFetch(e[2].l, c.utf8); m.UID != 0 {
				rappel(m)
			}
		}
	})
	return err
}

// Ensemble formate une liste d'UID en ensemble IMAP compact (plages).
func Ensemble(uids []uint32) string {
	var b strings.Builder
	for i := 0; i < len(uids); {
		j := i
		for j+1 < len(uids) && uids[j+1] == uids[j]+1 {
			j++
		}
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatUint(uint64(uids[i]), 10))
		if j > i {
			b.WriteByte(':')
			b.WriteString(strconv.FormatUint(uint64(uids[j]), 10))
		}
		i = j + 1
	}
	return b.String()
}
