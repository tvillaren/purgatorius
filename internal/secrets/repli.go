package secrets

// Coffre du systeme avec repli fichier (Windows et Linux, via go-keyring ; voir
// secrets_keyring.go). La logique est ici, compilee partout, pour etre testee
// sur tous les systemes avec un faux coffre.

import (
	"errors"
	"fmt"
	"log"
	"sync/atomic"
)

// errIntrouvable : le coffre repond, et n'a pas ce secret.
var errIntrouvable = errors.New("secret introuvable dans le coffre")

// coffre : un coffre du systeme. lire renvoie errIntrouvable si le secret
// n'y est pas ; toute autre erreur signifie un coffre indisponible
// (verrouille, D-Bus absent...). supprimer ne signale pas un secret absent.
type coffre interface {
	ecrire(c Cle, secret string) error
	lire(c Cle) (string, error)
	supprimer(c Cle) error
}

// avecRepli : un coffre, et le fichier de repli si repli est vrai (Linux ;
// Windows a toujours son coffre).
type avecRepli struct {
	coffre coffre
	repli  bool
	actif  *atomic.Bool // le repli sert (Repli())
}

func (a avecRepli) ecrire(c Cle, secret string) error {
	err := a.coffre.ecrire(c, secret)
	if err == nil {
		// Un ancien repli devient inutile. Reste, il serait lu avant le
		// coffre (voir lire) : l'ancien secret l'emporterait sur le nouveau.
		if errF := supprimerFichier(c); errF != nil {
			return fmt.Errorf("ancien secret de repli non efface : %w", errF)
		}
		return nil
	}
	if !a.repli {
		return err
	}
	a.actif.Store(true)
	if err := ecrireFichier(c, secret); err != nil {
		return err
	}
	// L'ancien secret du coffre ne doit pas ressortir quand le coffre revient :
	// lire consulte le fichier d'abord ; on tente aussi de l'effacer (le coffre
	// est sans doute encore indisponible, d'ou l'erreur ignoree).
	_ = a.coffre.supprimer(c)
	return nil
}

// lire : le fichier de repli d'abord, s'il existe. Il est forcement plus
// recent que le coffre, puisqu'une ecriture reussie dans le coffre le
// supprime. Sinon le coffre : « absent » seulement si le coffre l'affirme ; un
// coffre indisponible est signale comme tel (sinon l'utilisateur ressaisirait
// un mot de passe pour rien).
func (a avecRepli) lire(c Cle) (string, error) {
	if a.repli {
		s, err := lireFichier(c)
		if err == nil {
			a.actif.Store(true)
			return s, nil
		}
		if !errors.Is(err, ErrAbsent) {
			return "", err
		}
	}
	s, err := a.coffre.lire(c)
	switch {
	case err == nil:
		return s, nil
	case errors.Is(err, errIntrouvable):
		return "", ErrAbsent
	}
	return "", fmt.Errorf("coffre du systeme indisponible : %w", err)
}

// supprimer : avec repli, le fichier fait foi. Un coffre indisponible (le
// cas meme du repli) n'empeche pas la suppression : un ancien secret peut y
// rester, ce qui est journalise (sans le secret).
func (a avecRepli) supprimer(c Cle) error {
	err := a.coffre.supprimer(c)
	if !a.repli {
		return err
	}
	if errF := supprimerFichier(c); errF != nil {
		return errF
	}
	if err != nil {
		log.Printf("secret %s : coffre indisponible a la suppression (%v), fichier de repli efface", c.ID, err)
	}
	return nil
}
