package secrets

import (
	"errors"
	"io/fs"
	"strings"

	"github.com/louis49/purgatorius/internal/store"
)

// Repli fichier (Linux sans coffre) : secrets/<id>.secret, ecrit en 0600 par
// le store (ecriture atomique), dans un dossier de donnees en 0700. Protege
// par les droits du fichier et le chiffrement du disque, pas davantage.

func cheminFichier(c Cle) string { return "secrets/" + c.ID + ".secret" }

func ecrireFichier(c Cle, secret string) error {
	return store.Save(cheminFichier(c), []byte(secret))
}

func lireFichier(c Cle) (string, error) {
	b, err := store.Load(cheminFichier(c))
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrAbsent
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func supprimerFichier(c Cle) error { return store.Remove(cheminFichier(c)) }
