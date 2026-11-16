package app

import (
	"testing"

	"github.com/louis49/purgatorius/internal/store"
)

func TestBorner(t *testing.T) {
	cas := []struct {
		nom      string
		e, att   EtatFenetre
		ecL, ecH int
	}{
		{"deja visible", EtatFenetre{X: 100, Y: 80, Largeur: 1200, Hauteur: 800, Position: true}, EtatFenetre{X: 100, Y: 80, Largeur: 1200, Hauteur: 800, Position: true}, 1920, 1080},
		{"ecran plus petit", EtatFenetre{X: 1500, Y: 900, Largeur: 2400, Hauteur: 1400, Position: true}, EtatFenetre{X: 1160, Y: 668, Largeur: 1280, Hauteur: 788, Position: true}, 1280, 788},
		{"hors ecran a gauche et en haut", EtatFenetre{X: -5000, Y: -300, Largeur: 1000, Hauteur: 700, Position: true}, EtatFenetre{X: -880, Y: 0, Largeur: 1000, Hauteur: 700, Position: true}, 1920, 1080},
		{"trop petite", EtatFenetre{X: 10, Y: 10, Largeur: 300, Hauteur: 200, Position: true}, EtatFenetre{X: 10, Y: 10, Largeur: 900, Hauteur: 600, Position: true}, 1920, 1080},
		// Ecran inconnu : position abandonnee, taille bornee aux minimums.
		{"ecran inconnu", EtatFenetre{X: 9999, Y: 9999, Largeur: 50, Hauteur: 50, Position: true}, EtatFenetre{X: 9999, Y: 9999, Largeur: 900, Hauteur: 600, Position: false}, 0, 0},
	}
	for _, c := range cas {
		if got := borner(c.e, c.ecL, c.ecH); got != c.att {
			t.Errorf("%s : %+v, attendu %+v", c.nom, got, c.att)
		}
	}
}

func TestFenetreAllerRetour(t *testing.T) {
	if _, ok := lireFenetre(); ok {
		t.Fatal("etat present avant toute sauvegarde")
	}
	if l, h := taillesDemarrage(EtatFenetre{}, false); l != largeurDefaut || h != hauteurDefaut {
		t.Fatalf("taille par defaut : %d x %d", l, h)
	}
	e := EtatFenetre{X: 40, Y: 60, Largeur: 1400, Hauteur: 900, Position: true}
	ecrireFenetre(e)
	if lu, ok := lireFenetre(); !ok || lu != e {
		t.Fatalf("relu %+v (%v), attendu %+v", lu, ok, e)
	}
	if l, h := taillesDemarrage(EtatFenetre{Largeur: 500, Hauteur: 400}, true); l != largeurMin || h != hauteurMin {
		t.Fatalf("minimums non appliques : %d x %d", l, h)
	}
}

// Un etat aberrant (fichier abime ou modifie a la main) est ignore : la
// fenetre repart a sa taille par defaut au lieu d'etre demandee geante.
func TestFenetreAberranteIgnoree(t *testing.T) {
	for _, brut := range []string{
		`{"largeur":2000000000,"hauteur":2000000000,"maximisee":true}`,
		`{"x":-900000,"y":10,"largeur":1200,"hauteur":800,"position":true}`,
		`{"largeur":0,"hauteur":800}`,
		`pas du json`,
	} {
		if err := store.Save(ficFenetre, []byte(brut)); err != nil {
			t.Fatal(err)
		}
		if e, ok := lireFenetre(); ok {
			t.Errorf("%s : etat accepte %+v", brut, e)
		}
	}
	if err := store.Save(ficFenetre, []byte(`{"x":-50,"y":0,"largeur":10000,"hauteur":600,"position":true}`)); err != nil {
		t.Fatal(err)
	}
	if _, ok := lireFenetre(); !ok {
		t.Error("etat aux bornes refuse")
	}
}
