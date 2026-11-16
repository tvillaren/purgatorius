package comptes

import (
	"errors"
	"os"
	"testing"

	"github.com/louis49/purgatorius/internal/store"
)

func TestMain(m *testing.M) {
	d, err := os.MkdirTemp("", "purgatorius-comptes-*")
	if err != nil {
		panic(err)
	}
	store.DefinirRacinePourTests(d)
	code := m.Run()
	os.RemoveAll(d)
	os.Exit(code)
}

func TestID(t *testing.T) {
	if ID(" Moi@Example.Invalid ") != ID("moi@example.invalid") || len(ID("a@b.example")) != 12 {
		t.Fatal("ID non deterministe ou de mauvaise longueur")
	}
	if ID("a@b.example") == ID("c@b.example") {
		t.Fatal("collision triviale")
	}
}

func TestValidations(t *testing.T) {
	if !AdresseValide("prenom.nom@example.invalid") || AdresseValide("pas une adresse") || AdresseValide("a@b") {
		t.Fatal("AdresseValide")
	}
	if !CouleurValide("#2A4FB8") || CouleurValide("bleu") || CouleurValide("#12345") {
		t.Fatal("CouleurValide")
	}
}

func TestModifier(t *testing.T) {
	r, err := Lire()
	if err != nil || len(r.Comptes) != 0 {
		t.Fatalf("registre initial : %+v %v", r, err)
	}
	_, err = Modifier(func(r *Registre) error {
		r.Comptes = append(r.Comptes, Compte{ID: ID("a@b.example"), Adresse: "a@b.example"})
		r.Actif = ID("a@b.example")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Une erreur dans fn n'ecrit rien.
	_, err = Modifier(func(r *Registre) error {
		r.Comptes = nil
		return errors.New("refus")
	})
	if err == nil {
		t.Fatal("erreur de fn perdue")
	}
	r, _ = Lire()
	if c, ok := r.Compte(ID("a@b.example")); !ok || c.Adresse != "a@b.example" || r.Actif != ID("a@b.example") {
		t.Fatalf("registre relu : %+v", r)
	}
	// Registre corrompu : jamais reecrit.
	store.Save(fichier, []byte("{corrompu"))
	if _, err := Modifier(func(r *Registre) error { return nil }); err == nil {
		t.Fatal("registre corrompu accepte")
	}
	if b, _ := store.Load(fichier); string(b) != "{corrompu" {
		t.Fatal("registre corrompu reecrit")
	}
}
