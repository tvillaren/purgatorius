// Package synchro remplit le cache d'un compte depuis Gmail par IMAP, en
// lecture seule, et met a la corbeille le Purgatoire (corbeille.go, seule
// action qui modifie la boite). Une synchro interrompue reprend la ou elle s'est arretee :
// seuls les UID absents du cache sont lus, du plus recent au plus ancien. Les
// messages deja en cache ne sont pas relus : leurs drapeaux (lu) et libelles
// sont rattrapes par CONDSTORE (ce qui a change depuis la synchro precedente).
package synchro

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"time"

	"github.com/louis49/purgatorius/internal/cache"
	"github.com/louis49/purgatorius/internal/imap"
)

// Etapes de la synchro, dans l'ordre.
const (
	EtapeConnexion  = "connexion"
	EtapeListe      = "liste"
	EtapeEntetes    = "entetes"
	EtapeCategories = "categories"
	EtapeTerminee   = "terminee"
)

// ErrTousLesMessagesAbsent : le dossier \All n'est pas expose en IMAP
// (reglage Gmail « Afficher dans IMAP » decoche pour Tous les messages).
var ErrTousLesMessagesAbsent = errors.New(`dossier "Tous les messages" introuvable en IMAP`)

// Options d'une synchro.
type Options struct {
	Adresse    string
	MotDePasse string
	Hote       string // imap.HoteGmail par defaut
	Lot        int    // messages par FETCH (1000 par defaut)
	// Connecter remplace la connexion TLS a Hote (tests : faux serveur).
	Connecter func(ctx context.Context) (*imap.Client, error)
	// AvantVidage est appele juste avant de vider le cache (UIDVALIDITY
	// changee) : le compte n'est plus complet tant que la relecture n'est pas
	// terminee. Une erreur arrete la synchro sans rien vider.
	AvantVidage func() error
}

// Progression : etat transmis a l'IHM apres chaque etape et chaque lot.
type Progression struct {
	Etape              string
	Faits, Total       int           // messages lus / a lire pendant cette synchro
	DejaEnCache        int           // messages deja presents avant cette synchro
	Volume             int64         // somme des tailles des messages lus (octets)
	Octets             int64         // octets recus du serveur
	Debit              float64       // messages par seconde
	Restant            time.Duration // estimation, 0 si inconnue
	AnneeLaPlusVieille int           // annee du plus ancien message lu
}

// Bilan d'une synchro terminee.
type Bilan struct {
	Lus, Supprimes, Total int
	Actualises            int // messages en cache dont drapeaux ou libelles ont change
	Duree                 time.Duration
	Octets                int64
}

// DuLot ne garde que les messages demandes dans le lot, une fois chacun : le
// client ecarte deja les reponses incompletes, ceci protege le cache d'une
// reponse complete mais hors lot (filet de securite, voir imap.Recuperer).
func DuLot(lot []uint32, msgs []imap.Message) []imap.Message {
	attendus := make(map[uint32]bool, len(lot))
	for _, u := range lot {
		attendus[u] = true
	}
	out := msgs[:0]
	for _, m := range msgs {
		if attendus[m.UID] && m.GmMsgID != 0 {
			attendus[m.UID] = false
			out = append(out, m)
		}
	}
	return out
}

// rattraper met a jour les drapeaux et libelles des messages en cache modifies
// dans Gmail depuis la synchro precedente (compteur CONDSTORE enregistre a sa
// fin). Sans compteur (premiere synchro, cache vide) ou sans CONDSTORE, rien
// a faire : les messages lus ensuite le sont en entier. Les reponses pour des
// UID absents du cache (pas encore lus) sont ignorees par le cache.
func rattraper(c *cache.Cache, cli *imap.Client, etat imap.Etat) (int, error) {
	brut, err := c.Meta("highestmodseq")
	if err != nil || brut == "" || etat.HighestModSeq == 0 {
		return 0, err
	}
	depuis, err := strconv.ParseUint(brut, 10, 64)
	if err != nil || depuis >= etat.HighestModSeq {
		return 0, nil
	}
	var changes []imap.Message
	if err := cli.Changements(depuis, func(m imap.Message) { changes = append(changes, m) }); err != nil {
		return 0, fmt.Errorf("changements depuis la derniere synchro : %w", err)
	}
	return c.ActualiserDrapeaux(changes)
}

// absents : nombre d'UID du serveur qui ne sont pas en cache.
func absents(c *cache.Cache, serveur []uint32) (int, error) {
	enCache, err := c.UIDs()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, u := range serveur {
		if !enCache[u] {
			n++
		}
	}
	return n, nil
}

// Planifier compare les UID du serveur a ceux du cache : messages a lire (du
// plus recent au plus ancien) et messages a retirer du cache.
func Planifier(serveur []uint32, enCache map[uint32]bool) (aLire, aSupprimer []uint32) {
	surServeur := make(map[uint32]bool, len(serveur))
	for _, u := range serveur {
		surServeur[u] = true
		if !enCache[u] {
			aLire = append(aLire, u)
		}
	}
	for u := range enCache {
		if !surServeur[u] {
			aSupprimer = append(aSupprimer, u)
		}
	}
	slices.Sort(aLire)
	slices.Reverse(aLire)
	slices.Sort(aSupprimer)
	return aLire, aSupprimer
}

// Lancer synchronise le cache avec la boite. rappel peut etre nil.
func Lancer(ctx context.Context, c *cache.Cache, opt Options, rappel func(Progression)) (Bilan, error) {
	debut := time.Now()
	if opt.Hote == "" {
		opt.Hote = imap.HoteGmail
	}
	if opt.Lot <= 0 {
		opt.Lot = 1000
	}
	signaler := func(p Progression) {
		if rappel != nil {
			rappel(p)
		}
	}
	echec := func(err error) (Bilan, error) {
		if ctx.Err() != nil {
			return Bilan{}, ctx.Err()
		}
		return Bilan{}, err
	}

	if opt.Connecter == nil {
		opt.Connecter = func(ctx context.Context) (*imap.Client, error) { return imap.Connecter(ctx, opt.Hote) }
	}

	signaler(Progression{Etape: EtapeConnexion})
	cli, err := opt.Connecter(ctx)
	if err != nil {
		return echec(err)
	}
	defer cli.Fermer()
	if err := cli.Authentifier(opt.Adresse, opt.MotDePasse); err != nil {
		return echec(err)
	}
	dossiers, err := cli.Dossiers()
	if err != nil {
		return echec(err)
	}
	tous := ""
	for _, d := range dossiers {
		if d.Special(`\All`) {
			tous = d.Nom
		}
	}
	if tous == "" {
		return Bilan{}, ErrTousLesMessagesAbsent
	}
	etat, err := cli.Examiner(tous)
	if err != nil {
		return echec(err)
	}

	// UIDVALIDITY changee : les UID du cache ne designent plus les memes
	// messages, on repart de zero.
	validite := strconv.FormatUint(uint64(etat.UIDValidity), 10)
	vide := false
	if ancienne, err := c.Meta("uidvalidity"); err != nil {
		return echec(err)
	} else if ancienne != "" && ancienne != validite {
		if opt.AvantVidage != nil {
			if err := opt.AvantVidage(); err != nil {
				return echec(err)
			}
		}
		if err := c.Vider(); err != nil {
			return echec(err)
		}
		// Le compteur CONDSTORE appartenait a l'ancienne numerotation.
		if err := c.EcrireMeta("highestmodseq", ""); err != nil {
			return echec(err)
		}
		vide = true
	}
	if err := c.EcrireMeta("uidvalidity", validite); err != nil {
		return echec(err)
	}
	// Sans compteur (premiere synchro, cache vide) : on note celui du debut de
	// cette synchro. Si elle est interrompue, la suivante rattrapera depuis
	// ce point les messages deja lus. Le compteur enregistre est ainsi
	// toujours anterieur a la lecture de chaque message du cache.
	if anc, err := c.Meta("highestmodseq"); err != nil {
		return echec(err)
	} else if anc == "" && etat.HighestModSeq > 0 {
		if err := c.EcrireMeta("highestmodseq", strconv.FormatUint(etat.HighestModSeq, 10)); err != nil {
			return echec(err)
		}
	}

	signaler(Progression{Etape: EtapeListe})
	serveur, err := cli.RechercherUID("ALL")
	if err != nil {
		return echec(err)
	}
	enCache, err := c.UIDs()
	if err != nil {
		return echec(err)
	}
	aLire, aSupprimer := Planifier(serveur, enCache)
	if err := c.Supprimer(aSupprimer); err != nil {
		return echec(err)
	}
	actualises := 0
	if !vide {
		if actualises, err = rattraper(c, cli, etat); err != nil {
			return echec(err)
		}
	}

	p := Progression{Etape: EtapeEntetes, Total: len(aLire), DejaEnCache: len(enCache) - len(aSupprimer)}
	signaler(p)
	debutLecture := time.Now()
	octets0 := cli.Octets()
	for i := 0; i < len(aLire); i += opt.Lot {
		lot := slices.Clone(aLire[i:min(i+opt.Lot, len(aLire))])
		slices.Sort(lot)
		msgs := make([]imap.Message, 0, len(lot))
		if err := cli.Recuperer(imap.Ensemble(lot), func(m imap.Message) { msgs = append(msgs, m) }); err != nil {
			return echec(err)
		}
		msgs = DuLot(lot, msgs)
		if err := c.Enregistrer(msgs); err != nil {
			return echec(err)
		}
		for _, m := range msgs {
			p.Volume += m.Taille
			if a := m.Date.Year(); m.Date.Unix() > 0 && (p.AnneeLaPlusVieille == 0 || a < p.AnneeLaPlusVieille) {
				p.AnneeLaPlusVieille = a
			}
		}
		p.Faits += len(lot)
		p.Octets = cli.Octets() - octets0
		if ecoule := time.Since(debutLecture).Seconds(); ecoule > 0 {
			p.Debit = float64(p.Faits) / ecoule
			p.Restant = time.Duration(float64(p.Total-p.Faits) / p.Debit * float64(time.Second))
		}
		signaler(p)
	}

	signaler(Progression{Etape: EtapeCategories, Faits: p.Faits, Total: p.Total, Volume: p.Volume})
	cats := map[uint32]string{}
	for nom, requete := range cache.Categories {
		uids, err := cli.RechercherGmail(requete)
		if err != nil {
			return echec(fmt.Errorf("categorie %s : %w", nom, err))
		}
		for _, u := range uids {
			cats[u] = nom
		}
	}
	if err := c.DefinirCategories(cats); err != nil {
		return echec(err)
	}
	// Les messages du Purgatoire supprimes depuis dans Gmail peuvent en
	// sortir, mais seulement si le cache reflete toute la boite : un lot
	// incomplet (message omis par le serveur, ou ecarte par Recuperer) laisse
	// des absents, dont les entrees seraient perdues alors qu'ils sont
	// toujours chez Gmail (ils seront relus a la synchro suivante).
	if manquants, err := absents(c, serveur); err != nil {
		return echec(err)
	} else if manquants > 0 {
		log.Printf("synchro : %d messages du serveur absents du cache, Purgatoire non nettoye", manquants)
	} else if _, err := c.NettoyerPurgatoire(); err != nil {
		return echec(err)
	}
	if etat.HighestModSeq > 0 {
		if err := c.EcrireMeta("highestmodseq", strconv.FormatUint(etat.HighestModSeq, 10)); err != nil {
			return echec(err)
		}
	}
	if err := c.EcrireMeta("derniere_synchro", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return echec(err)
	}
	_ = cli.Deconnecter()

	b := Bilan{Lus: p.Faits, Supprimes: len(aSupprimer), Total: len(serveur), Actualises: actualises, Duree: time.Since(debut), Octets: cli.Octets()}
	signaler(Progression{Etape: EtapeTerminee, Faits: p.Faits, Total: p.Total, Volume: p.Volume, Octets: p.Octets, Debit: p.Debit})
	return b, nil
}
