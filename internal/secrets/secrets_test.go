package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/louis49/purgatorius/internal/store"
)

func TestMain(m *testing.M) {
	d, err := os.MkdirTemp("", "purgatorius-secrets-*")
	if err != nil {
		panic(err)
	}
	store.DefinirRacinePourTests(d)
	code := m.Run()
	os.RemoveAll(d)
	os.Exit(code)
}

func TestCleIncomplete(t *testing.T) {
	if err := Ecrire(Cle{ID: "x"}, "s"); err == nil {
		t.Fatal("cle sans adresse acceptee")
	}
}

// Repli fichier : portable, teste sur tous les systemes.
func TestRepliFichier(t *testing.T) {
	c := Cle{ID: "0a1b2c3d4e5f", Adresse: "test@example.invalid"}
	if _, err := lireFichier(c); !errors.Is(err, ErrAbsent) {
		t.Fatalf("lecture d'un repli absent : %v", err)
	}
	if err := ecrireFichier(c, "abcd efgh\n"); err != nil {
		t.Fatal(err)
	}
	if s, err := lireFichier(c); err != nil || s != "abcd efgh" {
		t.Fatalf("relu %q, %v", s, err)
	}
	p, _ := store.CheminAbsolu(cheminFichier(c))
	// Droits Unix : sous Windows, Go les rapporte toujours en 0666 ; le fichier
	// y est protege par les ACL du profil de l'utilisateur.
	if st, err := os.Stat(p); err != nil || (runtime.GOOS != "windows" && st.Mode().Perm() != 0o600) {
		t.Fatalf("droits du fichier de repli : %v %v", st.Mode().Perm(), err)
	}
	if filepath.Base(p) != c.ID+".secret" {
		t.Fatalf("nom de fichier %q : l'adresse ne doit pas y figurer", filepath.Base(p))
	}
	if err := supprimerFichier(c); err != nil {
		t.Fatal(err)
	}
	if err := supprimerFichier(c); err != nil {
		t.Fatalf("suppression d'un repli absent : %v", err)
	}
}

// Coffre reel : ecrit, relit et supprime un element jetable. Desactive par
// defaut (CI sans trousseau deverrouille) : PURGATORIUS_TEST_COFFRE=1.
func TestCoffreReel(t *testing.T) {
	if os.Getenv("PURGATORIUS_TEST_COFFRE") != "1" {
		t.Skip("PURGATORIUS_TEST_COFFRE=1 pour tester le coffre du systeme")
	}
	c := Cle{ID: "test00000000", Adresse: "test-coffre@example.invalid"}
	t.Cleanup(func() { _ = Supprimer(c) })
	if _, err := Lire(c); !errors.Is(err, ErrAbsent) {
		t.Fatalf("avant ecriture : %v", err)
	}
	for _, v := range []string{"premier secret", "second secret"} { // creation puis mise a jour
		if err := Ecrire(c, v); err != nil {
			t.Fatal(err)
		}
		if s, err := Lire(c); err != nil || s != v {
			t.Fatalf("relu %q, %v ; attendu %q", s, err, v)
		}
	}
	if err := Supprimer(c); err != nil {
		t.Fatal(err)
	}
	if _, err := Lire(c); !errors.Is(err, ErrAbsent) {
		t.Fatalf("apres suppression : %v", err)
	}
}
