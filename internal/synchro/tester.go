package synchro

import (
	"context"
	"errors"
	"net"

	"github.com/louis49/purgatorius/internal/imap"
)

// Tester verifie les identifiants et renvoie le nombre de messages de « Tous
// les messages », sans rien modifier dans la boite.
func Tester(ctx context.Context, opt Options) (int, error) {
	if opt.Hote == "" {
		opt.Hote = imap.HoteGmail
	}
	if opt.Connecter == nil {
		opt.Connecter = func(ctx context.Context) (*imap.Client, error) { return imap.Connecter(ctx, opt.Hote) }
	}
	cli, err := opt.Connecter(ctx)
	if err != nil {
		return 0, err
	}
	defer cli.Fermer()
	if err := cli.Authentifier(opt.Adresse, opt.MotDePasse); err != nil {
		return 0, err
	}
	dossiers, err := cli.Dossiers()
	if err != nil {
		return 0, err
	}
	for _, d := range dossiers {
		if d.Special(`\All`) {
			et, err := cli.Examiner(d.Nom)
			if err != nil {
				return 0, err
			}
			_ = cli.Deconnecter()
			return et.Existe, nil
		}
	}
	return 0, ErrTousLesMessagesAbsent
}

// Codes d'erreur transmis a l'IHM (traduits cote front).
const (
	ErreurAuthentification = "authentification"
	ErreurTousAbsent       = "tous_les_messages_absent"
	ErreurReseau           = "reseau"
	ErreurAnnulee          = "annulee"
	ErreurAutre            = "autre"
)

// CodeErreur classe une erreur de connexion ou de synchro pour l'IHM.
func CodeErreur(err error) string {
	var ne net.Error
	var oe *net.OpError
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return ErreurAnnulee
	case errors.Is(err, imap.ErrAuthentification):
		return ErreurAuthentification
	case errors.Is(err, ErrTousLesMessagesAbsent):
		return ErreurTousAbsent
	case errors.As(err, &oe), errors.As(err, &ne), errors.Is(err, context.DeadlineExceeded):
		return ErreurReseau
	}
	return ErreurAutre
}
