package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/louis49/purgatorius/internal/cache"
	"github.com/louis49/purgatorius/internal/comptes"
	"github.com/louis49/purgatorius/internal/store"
)

// compteComplet ajoute un compte marque comme entierement synchronise.
func compteComplet(t *testing.T, a *App, adresse string) string {
	t.Helper()
	cv, err := a.AjouterCompte(NouveauCompte{Nom: "Test", Adresse: adresse, MotDePasse: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := comptes.Modifier(func(r *comptes.Registre) error {
		c, _ := r.Compte(cv.ID)
		c.SynchroComplete = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return cv.ID
}

func estComplet(t *testing.T, id string) bool {
	t.Helper()
	r, err := comptes.Lire()
	if err != nil {
		t.Fatal(err)
	}
	c, ok := r.Compte(id)
	return ok && c.SynchroComplete
}

// Cache efface a la main (dossier cache/ presente comme purgeable) :
// le compte repasse par la synchro au lieu d'afficher une carte vide comme
// complete. Un cache synchronise (UIDVALIDITY connue) n'est pas touche.
func TestCacheEffaceALaMain(t *testing.T) {
	a := NewApp()
	efface := compteComplet(t, a, "efface@example.invalid")
	garde := compteComplet(t, a, "garde@example.invalid")
	c, err := cache.Ouvrir(garde)
	if err != nil {
		t.Fatal(err)
	}
	c.EcrireMeta("uidvalidity", "7")
	c.Fermer()
	t.Cleanup(func() { a.fermerLecteur(efface); a.fermerLecteur(garde) })

	a.verifierCaches()
	if estComplet(t, efface) {
		t.Error("cache absent : le compte est reste marque complet")
	}
	if !estComplet(t, garde) {
		t.Error("cache synchronise : le compte a ete repasse en synchro")
	}
}

// Un Purgatoire illisible est mis de cote a l'ouverture ; l'avis est
// emis et retenu pour le front (une seule fois), la carte reste utilisable.
func TestPurgatoireMisDeCoteAnnonce(t *testing.T) {
	a := NewApp()
	var mu sync.Mutex
	var avis []AvisMisDeCote
	a.emettre = func(nom string, d any) {
		if x, ok := d.(AvisMisDeCote); ok && nom == EvPurgatoireMisDeCote {
			mu.Lock()
			avis = append(avis, x)
			mu.Unlock()
		}
	}
	id := compteComplet(t, a, "abime@example.invalid")
	p, err := store.CheminAbsolu(cache.CheminPurgatoire(id))
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(p), 0o700)
	if err := os.WriteFile(p, []byte(strings.Repeat("pas une base SQLite ", 300)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.fermerLecteur(id) })

	a.verifierCaches()
	if _, err := a.EtatPurgatoire(id); err != nil {
		t.Fatalf("carte bloquee : %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(avis) != 1 || avis[0].Compte != id || !strings.HasPrefix(avis[0].Fichier, "purgatoire.db.illisible-") {
		t.Fatalf("avis emis : %+v", avis)
	}
	if f := a.PurgatoireMisDeCote(id); f != avis[0].Fichier {
		t.Fatalf("avis retenu : %q", f)
	}
	if f := a.PurgatoireMisDeCote(id); f != "" {
		t.Fatalf("avis rendu deux fois : %q", f)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(p), avis[0].Fichier)); err != nil {
		t.Fatalf("fichier garde : %v", err)
	}
}
