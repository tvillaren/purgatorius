package app

import (
	"testing"

	"github.com/louis49/purgatorius/internal/comptes"
	"github.com/louis49/purgatorius/internal/secrets"
	"github.com/louis49/purgatorius/internal/store"
)

func TestCycleDeVieCompte(t *testing.T) {
	a := NewApp()
	if e := a.Etat(); len(e.Comptes) != 0 || e.Actif != "" || e.Erreur != "" {
		t.Fatalf("etat initial : %+v", e)
	}
	if _, err := a.AjouterCompte(NouveauCompte{Adresse: "pas une adresse", MotDePasse: "x"}); err == nil {
		t.Fatal("adresse invalide acceptee")
	}
	if _, err := a.AjouterCompte(NouveauCompte{Adresse: "a@b.example"}); err == nil {
		t.Fatal("mot de passe vide accepte")
	}
	cv, err := a.AjouterCompte(NouveauCompte{Adresse: " Moi@Example.Invalid ", MotDePasse: "abcd efgh ijkl mnop", Couleur: "pas une couleur"})
	if err != nil {
		t.Fatal(err)
	}
	if cv.Adresse != "moi@example.invalid" || cv.Nom != "moi@example.invalid" || cv.Couleur != "#60A5FA" || cv.ID != comptes.ID("moi@example.invalid") {
		t.Fatalf("compte cree : %+v", cv)
	}
	if s, err := secrets.Lire(secrets.Cle{ID: cv.ID, Adresse: cv.Adresse}); err != nil || s != "abcdefghijklmnop" {
		t.Fatalf("secret : %q %v (espaces retires attendus)", s, err)
	}
	// Meme adresse : mise a jour, pas de doublon.
	if _, err := a.AjouterCompte(NouveauCompte{Nom: "Perso", Adresse: "moi@example.invalid", MotDePasse: "x", Couleur: "#187A63"}); err != nil {
		t.Fatal(err)
	}
	cv2, _ := a.AjouterCompte(NouveauCompte{Nom: "Asso", Adresse: "asso@example.invalid", MotDePasse: "y"})
	e := a.Etat()
	if len(e.Comptes) != 2 || e.Comptes[0].Nom != "Perso" || e.Comptes[0].Couleur != "#187A63" || e.Actif != cv2.ID {
		t.Fatalf("etat apres ajouts : %+v", e)
	}
	if err := a.ChoisirCompte(cv.ID); err != nil || a.Etat().Actif != cv.ID {
		t.Fatalf("choix du compte : %v", err)
	}
	if err := a.ChoisirCompte("inconnu"); err == nil {
		t.Fatal("compte inconnu accepte")
	}
	if err := a.DefinirLangue("en"); err != nil || a.Etat().Langue != "en" {
		t.Fatalf("langue : %v", err)
	}
	if err := a.DefinirLangue("de"); err == nil {
		t.Fatal("langue non prise en charge acceptee")
	}

	store.Save("cache/comptes/"+cv.ID+"/cache.db", []byte("x"))
	if err := a.SupprimerCompte(cv.ID); err != nil {
		t.Fatal(err)
	}
	e = a.Etat()
	if len(e.Comptes) != 1 || e.Actif != cv2.ID {
		t.Fatalf("etat apres suppression : %+v", e)
	}
	if store.Has("cache/comptes/" + cv.ID + "/cache.db") {
		t.Fatal("cache du compte supprime encore present")
	}
	if _, err := secrets.Lire(secrets.Cle{ID: cv.ID, Adresse: cv.Adresse}); err != secrets.ErrAbsent {
		t.Fatalf("secret du compte supprime : %v", err)
	}
}

func TestTesterConnexionAdresseInvalide(t *testing.T) {
	if r := NewApp().TesterConnexion("nope", "x"); r.OK || r.Code != "adresse_invalide" {
		t.Fatalf("resultat : %+v", r)
	}
}

func TestModifierCompte(t *testing.T) {
	a := NewApp()
	cv, err := a.AjouterCompte(NouveauCompte{Nom: "Ancien", Adresse: "renommer@example.invalid", MotDePasse: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ModifierCompte(cv.ID, "Nouveau", "rouge"); err == nil {
		t.Fatal("couleur invalide acceptee")
	}
	if _, err := a.ModifierCompte("inconnu", "x", "#F87171"); err == nil {
		t.Fatal("compte inconnu accepte")
	}
	m, err := a.ModifierCompte(cv.ID, "  Nouveau  ", "#F87171")
	if err != nil || m.Nom != "Nouveau" || m.Couleur != "#F87171" {
		t.Fatalf("modification : %+v %v", m, err)
	}
	m, _ = a.ModifierCompte(cv.ID, " ", "#F87171")
	if m.Nom != "renommer@example.invalid" {
		t.Fatalf("nom vide : %q, attendu l'adresse", m.Nom)
	}
	a.SupprimerCompte(cv.ID)
}
