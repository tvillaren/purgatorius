package synchro

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"

	"github.com/louis49/purgatorius/internal/cache"
	"github.com/louis49/purgatorius/internal/imap"
)

// Mise a la corbeille du Purgatoire : la seule action qui modifie la boite.
//
// Ce qui part est exactement ce que la revue affiche (cache.CiblesCorbeille).
// Les UID du cache ne sont que des candidats : dans la session, apres controle
// d'UIDVALIDITY, chacun est verifie par son X-GM-MSGID ; un message qui ne
// correspond pas est cherche par son X-GM-MSGID (UID SEARCH). Les messages
// sont deplaces (UID MOVE) de « Tous les messages » vers la corbeille, par
// lots ; chaque lot deplace sort aussitot du cache et du Purgatoire, si bien
// qu'une interruption laisse un etat coherent et qu'une relance reprend ce qui
// reste. Dans Gmail, la corbeille garde les messages 30 jours.

var (
	// ErrCorbeilleAbsente : pas de dossier special-use \Trash en IMAP.
	ErrCorbeilleAbsente = errors.New(`dossier "Corbeille" introuvable en IMAP`)
	// ErrEcartCorbeille : le Purgatoire ne correspond plus a ce qui etait
	// affiche ; rien n'est envoye.
	ErrEcartCorbeille = errors.New("le Purgatoire a change depuis son affichage")
	// ErrNumerotation : UIDVALIDITY de la boite differente de celle du cache
	// (ou cache jamais synchronise) : une synchro doit d'abord le remettre a jour.
	ErrNumerotation = errors.New("numerotation de la boite changee depuis la derniere synchro")
)

// lotCorbeille : messages par commande (FETCH de verification, MOVE).
// Variable pour les tests.
var lotCorbeille = 500

// BilanCorbeille : issue d'une mise a la corbeille.
type BilanCorbeille struct {
	Deplaces int   // messages mis a la corbeille
	Taille   int64 // leur taille
	Absents  int   // deja hors de « Tous les messages » : retires du Purgatoire
	Ambigus  int   // X-GM-MSGID porte par plusieurs UID : laisses au Purgatoire
}

// MettreALaCorbeille met a la corbeille de Gmail les messages du Purgatoire.
// attendu : nombre affiche par la revue (0 : pas de controle). rappel (peut
// etre nil) recoit l'avancement (deplaces, total).
func MettreALaCorbeille(ctx context.Context, c *cache.Cache, opt Options, attendu int, rappel func(faits, total int)) (BilanCorbeille, error) {
	var b BilanCorbeille
	echec := func(err error) (BilanCorbeille, error) {
		if ctx.Err() != nil {
			return b, ctx.Err()
		}
		return b, err
	}
	cibles, err := c.CiblesCorbeille()
	if err != nil {
		return b, err
	}
	if attendu > 0 && len(cibles) != attendu {
		return b, ErrEcartCorbeille
	}
	if len(cibles) == 0 {
		return b, nil
	}
	validiteCache, err := c.Meta("uidvalidity")
	if err != nil {
		return b, err
	}

	if opt.Hote == "" {
		opt.Hote = imap.HoteGmail
	}
	if opt.Connecter == nil {
		opt.Connecter = func(ctx context.Context) (*imap.Client, error) { return imap.Connecter(ctx, opt.Hote) }
	}
	cli, err := opt.Connecter(ctx)
	if err != nil {
		return echec(err)
	}
	defer cli.Fermer()
	if err := cli.Authentifier(opt.Adresse, opt.MotDePasse); err != nil {
		return echec(err)
	}
	if !cli.Capacite("MOVE") || !cli.Capacite("X-GM-EXT-1") {
		return b, errors.New("le serveur n'annonce pas MOVE et X-GM-EXT-1")
	}
	dossiers, err := cli.Dossiers()
	if err != nil {
		return echec(err)
	}
	tous, corbeille := "", ""
	for _, d := range dossiers {
		switch {
		case d.Special(`\All`):
			tous = d.Nom
		case d.Special(`\Trash`):
			corbeille = d.Nom
		}
	}
	if tous == "" {
		return b, ErrTousLesMessagesAbsent
	}
	if corbeille == "" {
		return b, ErrCorbeilleAbsente
	}
	etat, err := cli.Selectionner(tous)
	if err != nil {
		return echec(err)
	}
	if validiteCache == "" || validiteCache != strconv.FormatUint(uint64(etat.UIDValidity), 10) {
		return b, ErrNumerotation
	}

	// Verification des candidats dans la session.
	type sur struct {
		uid   uint32
		cible cache.Cible
	}
	var surs []sur
	var aChercher []cache.Cible
	for i := 0; i < len(cibles); i += lotCorbeille {
		part := cibles[i:min(i+lotCorbeille, len(cibles))]
		uids := make([]uint32, len(part))
		for k, x := range part {
			uids[k] = x.UID
		}
		lus, err := cli.GmMsgIDs(imap.Ensemble(uids))
		if err != nil {
			return echec(err)
		}
		for _, x := range part {
			if lus[x.UID] == x.GmMsgID {
				surs = append(surs, sur{x.UID, x})
			} else {
				aChercher = append(aChercher, x)
			}
		}
	}
	var absents []uint64
	for _, x := range aChercher {
		uids, err := cli.ChercherGmMsgID(x.GmMsgID)
		if err != nil {
			return echec(err)
		}
		switch len(uids) {
		case 0:
			absents = append(absents, x.GmMsgID)
		case 1:
			surs = append(surs, sur{uids[0], x})
		default:
			b.Ambigus++
		}
	}
	if len(absents) > 0 {
		if err := c.SortirDeLaBoite(absents); err != nil {
			return b, err
		}
		b.Absents = len(absents)
	}

	// Deplacement, lot par lot ; chaque lot deplace sort du cache.
	slices.SortFunc(surs, func(x, y sur) int { return int(int64(x.uid) - int64(y.uid)) })
	total := len(surs)
	if rappel != nil {
		rappel(0, total)
	}
	for i := 0; i < total; i += lotCorbeille {
		if err := ctx.Err(); err != nil {
			return b, err
		}
		part := surs[i:min(i+lotCorbeille, total)]
		uids := make([]uint32, len(part))
		gms := make([]uint64, len(part))
		var taille int64
		for k, x := range part {
			uids[k], gms[k] = x.uid, x.cible.GmMsgID
			taille += x.cible.Taille
		}
		if err := cli.Deplacer(imap.Ensemble(uids), corbeille); err != nil {
			return echec(fmt.Errorf("mise a la corbeille : %w", err))
		}
		if err := c.SortirDeLaBoite(gms); err != nil {
			// Deplaces chez Gmail mais encore en cache : la prochaine synchro
			// les retirera (ils ne sont plus dans « Tous les messages »).
			return b, err
		}
		b.Deplaces += len(part)
		b.Taille += taille
		if rappel != nil {
			rappel(b.Deplaces, total)
		}
	}
	if b.Ambigus > 0 {
		log.Printf("corbeille : %d messages ambigus laisses au Purgatoire", b.Ambigus)
	}
	_ = cli.Deconnecter()
	return b, nil
}
