//go:build darwin

package secrets

import (
	"errors"

	"github.com/keybase/go-keychain"
)

func requete(c Cle) keychain.Item {
	it := keychain.NewItem()
	it.SetSecClass(keychain.SecClassGenericPassword)
	it.SetService(service)
	it.SetAccount(c.Adresse)
	return it
}

func ecrire(c Cle, secret string) error {
	it := requete(c)
	it.SetLabel(service + " (" + c.Adresse + ")")
	it.SetData([]byte(secret))
	err := keychain.AddItem(it)
	if errors.Is(err, keychain.ErrorDuplicateItem) {
		maj := keychain.NewItem()
		maj.SetData([]byte(secret))
		err = keychain.UpdateItem(requete(c), maj)
	}
	return err
}

func lire(c Cle) (string, error) {
	q := requete(c)
	q.SetMatchLimit(keychain.MatchLimitOne)
	q.SetReturnData(true)
	res, err := keychain.QueryItem(q)
	if err != nil {
		return "", err
	}
	if len(res) == 0 {
		return "", ErrAbsent
	}
	return string(res[0].Data), nil
}

func supprimer(c Cle) error {
	err := keychain.DeleteItem(requete(c))
	if errors.Is(err, keychain.ErrorItemNotFound) {
		return nil
	}
	return err
}

// Repli : jamais sur macOS (le Trousseau est toujours present).
func Repli() bool { return false }
