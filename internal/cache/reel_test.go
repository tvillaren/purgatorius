//go:build reel

package cache

// Temps de reponse du treemap sur le cache reel du compte actif (lecture
// seule). N'affiche que des durees et des comptes.
//
//	PURGATORIUS_CACHE=<chemin de cache.db> go test -tags reel -run TestCarteReelle -v ./internal/cache

import (
	"os"
	"testing"
	"time"
)

func TestCarteReelle(t *testing.T) {
	p := os.Getenv("PURGATORIUS_CACHE")
	if p == "" {
		t.Skip("PURGATORIUS_CACHE non defini")
	}
	c, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Fermer()
	for _, r := range []RequeteCarte{
		{Axes: []string{"domaine", "expediteur"}, Metrique: "taille"},
		{Axes: []string{"domaine", "expediteur"}, Metrique: "nombre"},
		{Axes: []string{"expediteur"}, Metrique: "taille"},
		{Axes: []string{"titre"}, Metrique: "nombre"},
		{Axes: []string{"tranche", "domaine"}, Metrique: "taille"},
	} {
		debut := time.Now()
		n, err := c.Carte(r)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%-24v %-7s %3d blocs (%5d dans Autres)  %v", r.Axes, r.Metrique, len(n.Noeuds), n.Noeuds[len(n.Noeuds)-1].Autres, time.Since(debut).Round(time.Millisecond))
		if r.Axes[0] == "domaine" && r.Metrique == "taille" {
			debut = time.Now()
			if _, err := c.DetailCarte(RequeteDetail{Chemin: []Filtre{{Axe: "domaine", Valeur: n.Noeuds[0].Cle}}, Metrique: "taille"}); err != nil {
				t.Fatal(err)
			}
			t.Logf("detail du premier domaine : %v", time.Since(debut).Round(time.Millisecond))
			debut = time.Now()
			if _, err := c.DetailCarte(RequeteDetail{Metrique: "taille"}); err != nil {
				t.Fatal(err)
			}
			t.Logf("detail de toute la boite : %v", time.Since(debut).Round(time.Millisecond))
		}
	}
}

// Liste des messages : pire cas (tous les messages, sans filtre) puis un lot
// de lignes.
//
//	PURGATORIUS_CACHE=<chemin de cache.db> go test -tags reel -run TestListeReelle -v ./internal/cache
func TestListeReelle(t *testing.T) {
	p := os.Getenv("PURGATORIUS_CACHE")
	if p == "" {
		t.Skip("PURGATORIUS_CACHE non defini")
	}
	c, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Fermer()
	for _, tri := range []string{"taille", "date", "expediteur", "pj"} {
		debut := time.Now()
		idx, err := c.IndexListe(RequeteListe{Tri: tri, Desc: true})
		if err != nil {
			t.Fatal(err)
		}
		d := time.Since(debut).Round(time.Millisecond)
		debut = time.Now()
		n := min(100, len(idx.UIDs))
		l, err := c.Lignes(idx.UIDs[:n])
		if err != nil || len(l) != n {
			t.Fatalf("lignes : %d sur %d (%v)", len(l), n, err)
		}
		t.Logf("index %-10s %6d messages  %v ; 100 lignes %v", tri, len(idx.UIDs), d, time.Since(debut).Round(time.Millisecond))
	}
}
