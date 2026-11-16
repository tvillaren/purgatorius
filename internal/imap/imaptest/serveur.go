// Package imaptest est un faux serveur Gmail pour les tests : une boite en
// memoire (« Tous les messages »), qui repond aux commandes du client imap
// sur une connexion net.Pipe et enregistre chaque commande recue. Importe
// seulement par des tests : il n'entre pas dans l'application.
package imaptest

import (
	"bufio"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/louis49/purgatorius/internal/imap"
)

// Message : un message de la fausse boite.
type Message struct {
	UID      uint32
	GmMsgID  uint64
	Taille   int64
	Date     time.Time
	Lu       bool
	Libelles []string // "\\Inbox", "\\Sent", libelles perso
	De       string   // ex. "Nom <a@x.example>"
	Objet    string
	ModSeq   uint64 // derniere modification (CONDSTORE)
}

// Boite : etat de la fausse boite et reglages du serveur.
type Boite struct {
	mu          sync.Mutex
	UIDValidity uint32
	Messages    []Message
	// Categories : requete X-GM-RAW (ex. "category:promotions") -> UID.
	Categories map[string][]uint32
	// CoupureApres : coupe la connexion au debut du n-ieme UID FETCH complet
	// (1 = le premier) ; 0 : jamais. Simule une synchro interrompue.
	CoupureApres int
	// Intrus : reponse non sollicitee glissee apres le premier message de
	// chaque UID FETCH complet (ex. "* 9 FETCH (UID 8 FLAGS (\\Seen))").
	Intrus string
	// MotDePasse : si non vide, LOGIN n'accepte que lui (sinon tout).
	MotDePasse string
	// Lenteur : attente avant de repondre a chaque UID FETCH complet (une
	// synchro qu'on peut interrompre en cours de lot).
	Lenteur time.Duration
	// Omis : UID annonces par SEARCH mais absents des reponses aux UID FETCH
	// complets (lot incomplet, accepte avec OK).
	Omis map[uint32]bool
	// Corbeille : messages deplaces (UID MOVE) vers la corbeille.
	Corbeille []Message
	// CoupureMoveApres : coupe la connexion au n-ieme UID MOVE (1 = le
	// premier), sans rien deplacer ; 0 : jamais.
	CoupureMoveApres int
	moves            int

	fetchs    int
	commandes []string
}

// Commandes : commandes recues (sans etiquette), dans l'ordre.
func (b *Boite) Commandes() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.commandes...)
}

// OublierCommandes efface les commandes recues (par exemple avant de
// verifier qu'une synchro, apres une mise a la corbeille, est en lecture
// seule).
func (b *Boite) OublierCommandes() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.commandes = nil
}

// HighestModSeq : plus grand ModSeq de la boite.
func (b *Boite) HighestModSeq() uint64 {
	var m uint64 = 1
	for _, x := range b.Messages {
		if x.ModSeq > m {
			m = x.ModSeq
		}
	}
	return m
}

// Modifier change un message (drapeaux, libelles) et avance son ModSeq,
// comme une action de l'utilisateur dans Gmail.
func (b *Boite) Modifier(uid uint32, f func(*Message)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	h := b.HighestModSeq()
	for i := range b.Messages {
		if b.Messages[i].UID == uid {
			f(&b.Messages[i])
			b.Messages[i].ModSeq = h + 1
		}
	}
}

// Connecter ouvre une session sur la fausse boite (accueil lu).
func (b *Boite) Connecter(t testing.TB) *imap.Client {
	t.Helper()
	cli, srv := net.Pipe()
	go b.servir(srv)
	c, err := imap.SurConnexion(cli)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Fermer() })
	return c
}

// verbesAutorises : tout ce qu'une session en lecture seule peut envoyer.
var verbesAutorises = map[string]bool{
	"LOGIN": true, "CAPABILITY": true, "ENABLE": true, "LIST": true, "EXAMINE": true,
	"STATUS": true, "UID SEARCH": true, "UID FETCH": true, "NOOP": true, "LOGOUT": true,
}

// VerifierLectureSeule echoue si une commande recue peut modifier la boite :
// verbe hors liste (SELECT, STORE, EXPUNGE, COPY, MOVE...), ou FETCH d'un
// contenu qui pose le drapeau \Seen : BODY[...] ou BINARY[...] sans PEEK,
// RFC822 et RFC822.TEXT (RFC822.SIZE et RFC822.HEADER ne le posent pas).
func (b *Boite) VerifierLectureSeule(t testing.TB) {
	t.Helper()
	for _, cmd := range b.Commandes() {
		if !verbesAutorises[verbe(cmd)] {
			t.Errorf("commande hors lecture seule : %q", cmd)
		}
		if reMarqueLu.MatchString(cmd) {
			t.Errorf("contenu lu sans PEEK (marque le message lu) : %q", cmd)
		}
	}
}

// reMarqueLu : elements de FETCH qui marquent le message lu.
var reMarqueLu = regexp.MustCompile(`(?i)\b(BODY\[|BINARY\[|RFC822(\.TEXT)?([\s)]|$))`)

func verbe(cmd string) string {
	v, reste, _ := strings.Cut(cmd, " ")
	v = strings.ToUpper(v)
	if v == "UID" {
		v2, _, _ := strings.Cut(reste, " ")
		v += " " + strings.ToUpper(v2)
	}
	return v
}

var (
	reFetchComplet = regexp.MustCompile(`^UID FETCH (\S+) \(UID RFC822\.SIZE`)
	reChangements  = regexp.MustCompile(`^UID FETCH 1:\* \(UID FLAGS X-GM-LABELS\) \(CHANGEDSINCE (\d+)\)$`)
	reRaw          = regexp.MustCompile(`^UID SEARCH X-GM-RAW "(.*)"$`)
	reLogin        = regexp.MustCompile(`^LOGIN "[^"]*" "(.*)"$`)
	reGmMsgIDs     = regexp.MustCompile(`^UID FETCH (\S+) \(UID X-GM-MSGID\)$`)
	reChercherGm   = regexp.MustCompile(`^UID SEARCH X-GM-MSGID (\d+)$`)
	reDeplacer     = regexp.MustCompile(`^UID MOVE (\S+) "(.*)"$`)
)

// NomCorbeille : nom du dossier \Trash annonce par LIST.
const NomCorbeille = "[Gmail]/Corbeille"

func (b *Boite) servir(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	ecrire := func(s string) { w.WriteString(s); w.Flush() }
	ecrire("* OK Gimap ready\r\n")
	for {
		ligne, err := r.ReadString('\n')
		if err != nil {
			return
		}
		tag, cmd, _ := strings.Cut(strings.TrimRight(ligne, "\r\n"), " ")
		b.mu.Lock()
		b.commandes = append(b.commandes, cmd)
		rep, couper := b.repondre(tag, cmd)
		b.mu.Unlock()
		if couper {
			return
		}
		if b.Lenteur > 0 && reFetchComplet.MatchString(cmd) {
			time.Sleep(b.Lenteur)
		}
		ecrire(rep)
		if verbe(cmd) == "LOGOUT" {
			return
		}
	}
}

// repondre construit la reponse a une commande (sous verrou).
func (b *Boite) repondre(tag, cmd string) (string, bool) {
	ok := tag + " OK Success\r\n"
	switch v := verbe(cmd); {
	case v == "LOGIN":
		if m := reLogin.FindStringSubmatch(cmd); b.MotDePasse != "" && (m == nil || m[1] != b.MotDePasse) {
			return tag + " NO [AUTHENTICATIONFAILED] Invalid credentials (Failure)\r\n", false
		}
		return ok, false
	case v == "CAPABILITY":
		return "* CAPABILITY IMAP4rev1 X-GM-EXT-1 CONDSTORE ENABLE UTF8=ACCEPT MOVE UIDPLUS\r\n" + ok, false
	case v == "ENABLE":
		return "* ENABLED UTF8=ACCEPT\r\n" + ok, false
	case v == "LIST":
		return "* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n* LIST (\\All \\HasNoChildren) \"/\" \"[Gmail]/Tous les messages\"\r\n" +
			"* LIST (\\Trash \\HasNoChildren) \"/\" \"" + NomCorbeille + "\"\r\n" + ok, false
	case v == "SELECT":
		return fmt.Sprintf("* %d EXISTS\r\n* OK [UIDVALIDITY %d] UIDs valid.\r\n%s OK [READ-WRITE] Success\r\n",
			len(b.Messages), b.UIDValidity, tag), false
	case reGmMsgIDs.MatchString(cmd):
		voulus := ensemble(reGmMsgIDs.FindStringSubmatch(cmd)[1])
		var s strings.Builder
		for i, m := range b.Messages {
			if voulus(m.UID) {
				fmt.Fprintf(&s, "* %d FETCH (UID %d X-GM-MSGID %d)\r\n", i+1, m.UID, m.GmMsgID)
			}
		}
		return s.String() + ok, false
	case reChercherGm.MatchString(cmd):
		gm, _ := strconv.ParseUint(reChercherGm.FindStringSubmatch(cmd)[1], 10, 64)
		var uids []string
		for _, m := range b.Messages {
			if m.GmMsgID == gm {
				uids = append(uids, strconv.FormatUint(uint64(m.UID), 10))
			}
		}
		return "* SEARCH " + strings.Join(uids, " ") + "\r\n" + ok, false
	case reDeplacer.MatchString(cmd):
		m := reDeplacer.FindStringSubmatch(cmd)
		b.moves++
		if b.CoupureMoveApres > 0 && b.moves >= b.CoupureMoveApres {
			return "", true
		}
		if m[2] != NomCorbeille {
			return tag + " NO [TRYCREATE] dossier inconnu\r\n", false
		}
		voulus := ensemble(m[1])
		var s strings.Builder
		reste := b.Messages[:0]
		retires := 0
		for i, x := range b.Messages {
			if voulus(x.UID) {
				b.Corbeille = append(b.Corbeille, x)
				fmt.Fprintf(&s, "* %d EXPUNGE\r\n", i+1-retires) // numeros decales par les precedents
				retires++
				continue
			}
			reste = append(reste, x)
		}
		b.Messages = reste
		return "* OK [COPYUID 1 1:2 1:2]\r\n" + s.String() + ok, false
	case v == "EXAMINE":
		return fmt.Sprintf("* %d EXISTS\r\n* OK [UIDVALIDITY %d] UIDs valid.\r\n* OK [HIGHESTMODSEQ %d] Highest\r\n%s OK [READ-ONLY] Success\r\n",
			len(b.Messages), b.UIDValidity, b.HighestModSeq(), tag), false
	case cmd == "UID SEARCH ALL":
		var uids []string
		for _, m := range b.Messages {
			uids = append(uids, strconv.FormatUint(uint64(m.UID), 10))
		}
		return "* SEARCH " + strings.Join(uids, " ") + "\r\n" + ok, false
	case reRaw.MatchString(cmd):
		var uids []string
		for _, u := range b.Categories[reRaw.FindStringSubmatch(cmd)[1]] {
			uids = append(uids, strconv.FormatUint(uint64(u), 10))
		}
		return "* SEARCH " + strings.Join(uids, " ") + "\r\n" + ok, false
	case reChangements.MatchString(cmd):
		depuis, _ := strconv.ParseUint(reChangements.FindStringSubmatch(cmd)[1], 10, 64)
		var s strings.Builder
		for i, m := range b.Messages {
			if m.ModSeq > depuis {
				fmt.Fprintf(&s, "* %d FETCH (UID %d FLAGS (%s) X-GM-LABELS (%s) MODSEQ (%d))\r\n", i+1, m.UID, drapeaux(m), libelles(m), m.ModSeq)
			}
		}
		return s.String() + ok, false
	case reFetchComplet.MatchString(cmd):
		b.fetchs++
		if b.CoupureApres > 0 && b.fetchs >= b.CoupureApres {
			return "", true
		}
		voulus := ensemble(reFetchComplet.FindStringSubmatch(cmd)[1])
		var s strings.Builder
		premier := true
		for i, m := range b.Messages {
			if !voulus(m.UID) || b.Omis[m.UID] {
				continue
			}
			s.WriteString(fetchComplet(i+1, m))
			if premier && b.Intrus != "" {
				s.WriteString(b.Intrus + "\r\n")
			}
			premier = false
		}
		return s.String() + ok, false
	case v == "LOGOUT":
		return "* BYE LOGOUT Requested\r\n" + ok, false
	}
	return tag + " BAD commande inconnue du faux serveur\r\n", false
}

func drapeaux(m Message) string {
	if m.Lu {
		return `\Seen`
	}
	return ""
}

func libelles(m Message) string {
	var l []string
	for _, x := range m.Libelles {
		if strings.HasPrefix(x, `\`) {
			l = append(l, x)
		} else {
			l = append(l, strconv.Quote(x))
		}
	}
	return strings.Join(l, " ")
}

func fetchComplet(seq int, m Message) string {
	entetes := "From: " + m.De + "\r\nSubject: " + m.Objet + "\r\n\r\n"
	return fmt.Sprintf("* %d FETCH (UID %d RFC822.SIZE %d INTERNALDATE \"%s\" FLAGS (%s) X-GM-MSGID %d X-GM-THRID %d X-GM-LABELS (%s) "+
		"BODYSTRUCTURE (\"text\" \"plain\" (\"charset\" \"utf-8\") NIL NIL \"7bit\" 10 1 NIL NIL NIL) "+
		"BODY[HEADER.FIELDS (FROM SUBJECT TO CC)] {%d}\r\n%s)\r\n",
		seq, m.UID, m.Taille, m.Date.Format("_2-Jan-2006 15:04:05 -0700"), drapeaux(m), m.GmMsgID, m.GmMsgID, libelles(m), len(entetes), entetes)
}

// ensemble interprete un ensemble d'UID IMAP ("1:3,7,9:10").
func ensemble(s string) func(uint32) bool {
	type plage struct{ a, b uint32 }
	var plages []plage
	for _, p := range strings.Split(s, ",") {
		a, b, ok := strings.Cut(p, ":")
		x, _ := strconv.ParseUint(a, 10, 32)
		y := x
		if ok {
			y, _ = strconv.ParseUint(b, 10, 32)
		}
		if x > y {
			x, y = y, x
		}
		plages = append(plages, plage{uint32(x), uint32(y)})
	}
	sort.Slice(plages, func(i, j int) bool { return plages[i].a < plages[j].a })
	return func(u uint32) bool {
		for _, p := range plages {
			if u >= p.a && u <= p.b {
				return true
			}
		}
		return false
	}
}
