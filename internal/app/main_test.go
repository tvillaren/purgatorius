package app

import (
	"os"
	"testing"

	"github.com/louis49/purgatorius/internal/secrets"
	"github.com/louis49/purgatorius/internal/store"
)

// Les tests n'ecrivent jamais dans les vraies donnees ni dans le vrai coffre.
func TestMain(m *testing.M) {
	d, err := os.MkdirTemp("", "purgatorius-app-*")
	if err != nil {
		panic(err)
	}
	store.DefinirRacinePourTests(d)
	secrets.DefinirMemoirePourTests()
	code := m.Run()
	os.RemoveAll(d)
	os.Exit(code)
}
