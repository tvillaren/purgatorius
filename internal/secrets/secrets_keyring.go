//go:build !darwin

package secrets

import (
	"errors"
	"runtime"
	"sync/atomic"

	"github.com/zalando/go-keyring"
)

// repliActif : le coffre a echoue sous Linux et le fichier de repli sert.
var repliActif atomic.Bool

// systeme : go-keyring (wincred sous Windows, Secret Service sous Linux),
// avec repli fichier sous Linux seulement.
var systeme = avecRepli{coffre: coffreKeyring{}, repli: runtime.GOOS != "windows", actif: &repliActif}

type coffreKeyring struct{}

func (coffreKeyring) ecrire(c Cle, secret string) error {
	return keyring.Set(service, c.Adresse, secret)
}

func (coffreKeyring) lire(c Cle) (string, error) {
	s, err := keyring.Get(service, c.Adresse)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", errIntrouvable
	}
	return s, err
}

func (coffreKeyring) supprimer(c Cle) error {
	if err := keyring.Delete(service, c.Adresse); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}

func ecrire(c Cle, secret string) error { return systeme.ecrire(c, secret) }
func lire(c Cle) (string, error)        { return systeme.lire(c) }
func supprimer(c Cle) error             { return systeme.supprimer(c) }

// Repli indique que le coffre est indisponible et que les secrets sont dans
// des fichiers 0600 (Linux sans Secret Service) : l'IHM l'affiche.
func Repli() bool { return repliActif.Load() }
