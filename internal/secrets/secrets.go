// Package secrets range les mots de passe d'application des comptes Gmail dans
// le coffre du systeme :
//
//   - macOS   : Trousseau, via l'API Security (cgo). L'element appartient a
//     Purgatorius ; une autre application declenche une demande d'acces. On
//     n'utilise PAS /usr/bin/security (go-keyring) : un element cree par cet
//     outil est lisible sans dialogue par tout programme de la session
//     (constate a l'essai) ;
//   - Windows : Gestionnaire d'identification (go-keyring, wincred) ;
//   - Linux   : Secret Service via D-Bus (go-keyring). Sans coffre disponible,
//     repli sur un fichier 0600 du dossier de donnees (Repli() le signale).
//
// Un secret n'est jamais journalise ni renvoye dans un message d'erreur.
package secrets

import (
	"errors"
	"sync"
)

// service : nom du service dans le coffre (visible dans Trousseaux d'acces).
const service = "Purgatorius"

// ErrAbsent : aucun secret enregistre pour ce compte.
var ErrAbsent = errors.New("secret absent du coffre")

// Cle identifie le secret d'un compte : l'adresse sert de nom de compte dans
// le coffre (lisible par l'utilisateur), l'identifiant nomme le fichier de
// repli (jamais l'adresse dans un nom de fichier).
type Cle struct {
	ID      string
	Adresse string
}

func (c Cle) valide() error {
	if c.ID == "" || c.Adresse == "" {
		return errors.New("cle de secret incomplete")
	}
	return nil
}

// memoire : coffre en memoire des tests des paquets appelants (nil en
// fonctionnement normal), pour ne jamais ecrire dans le vrai coffre.
var (
	muMemoire sync.Mutex
	memoire   map[string]string
)

// DefinirMemoirePourTests remplace le coffre du systeme par un coffre en
// memoire. A appeler depuis un TestMain. Jamais appele par l'application.
func DefinirMemoirePourTests() {
	muMemoire.Lock()
	defer muMemoire.Unlock()
	memoire = map[string]string{}
}

// Ecrire enregistre (ou remplace) le secret du compte.
func Ecrire(c Cle, secret string) error {
	if err := c.valide(); err != nil {
		return err
	}
	if m, ok := enMemoire(); ok {
		muMemoire.Lock()
		defer muMemoire.Unlock()
		m[c.Adresse] = secret
		return nil
	}
	return ecrire(c, secret)
}

// Lire renvoie le secret du compte, ou ErrAbsent.
func Lire(c Cle) (string, error) {
	if err := c.valide(); err != nil {
		return "", err
	}
	if m, ok := enMemoire(); ok {
		muMemoire.Lock()
		defer muMemoire.Unlock()
		if s, ok := m[c.Adresse]; ok {
			return s, nil
		}
		return "", ErrAbsent
	}
	return lire(c)
}

// Supprimer efface le secret du compte (sans erreur s'il est absent).
func Supprimer(c Cle) error {
	if err := c.valide(); err != nil {
		return err
	}
	if m, ok := enMemoire(); ok {
		muMemoire.Lock()
		defer muMemoire.Unlock()
		delete(m, c.Adresse)
		return nil
	}
	return supprimer(c)
}

func enMemoire() (map[string]string, bool) {
	muMemoire.Lock()
	defer muMemoire.Unlock()
	return memoire, memoire != nil
}
