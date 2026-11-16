package secrets

import (
	"errors"
	"os"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/louis49/purgatorius/internal/store"
)

// fauxCoffre : un coffre en memoire, qu'on peut mettre en panne.
type fauxCoffre struct {
	secrets map[string]string
	panne   bool
}

var errPanne = errors.New("D-Bus : coffre verrouille")

func (f *fauxCoffre) ecrire(c Cle, s string) error {
	if f.panne {
		return errPanne
	}
	f.secrets[c.Adresse] = s
	return nil
}

func (f *fauxCoffre) lire(c Cle) (string, error) {
	if f.panne {
		return "", errPanne
	}
	s, ok := f.secrets[c.Adresse]
	if !ok {
		return "", errIntrouvable
	}
	return s, nil
}

func (f *fauxCoffre) supprimer(c Cle) error {
	if f.panne {
		return errPanne
	}
	delete(f.secrets, c.Adresse)
	return nil
}

func nouveauRepli() (*fauxCoffre, avecRepli) {
	f := &fauxCoffre{secrets: map[string]string{}}
	return f, avecRepli{coffre: f, repli: true, actif: &atomic.Bool{}}
}

// Un coffre en panne n'est pas un secret absent.
func TestRepliCoffreIndisponibleNEstPasAbsent(t *testing.T) {
	f, a := nouveauRepli()
	c := Cle{ID: "repli1", Adresse: "a@example.invalid"}
	if _, err := a.lire(c); !errors.Is(err, ErrAbsent) {
		t.Fatalf("coffre disponible, secret absent : %v", err)
	}
	f.panne = true
	_, err := a.lire(c)
	if errors.Is(err, ErrAbsent) || !errors.Is(err, errPanne) {
		t.Fatalf("coffre en panne signale comme : %v", err)
	}
}

// Un mot de passe change pendant une panne du coffre (donc
// range dans le fichier) l'emporte sur l'ancien quand le coffre revient.
func TestRepliNouveauSecretLEmporte(t *testing.T) {
	f, a := nouveauRepli()
	c := Cle{ID: "repli2", Adresse: "b@example.invalid"}
	if err := a.ecrire(c, "ancien"); err != nil {
		t.Fatal(err)
	}
	f.panne = true
	if err := a.ecrire(c, "nouveau"); err != nil || !a.actif.Load() {
		t.Fatalf("ecriture de repli : %v (repli actif %v)", err, a.actif.Load())
	}
	f.panne = false
	if s, err := a.lire(c); err != nil || s != "nouveau" {
		t.Fatalf("apres retour du coffre : %q %v", s, err)
	}
	// Prochaine ecriture reussie dans le coffre : le fichier disparait, le
	// coffre fait de nouveau foi.
	if err := a.ecrire(c, "troisieme"); err != nil {
		t.Fatal(err)
	}
	if _, err := lireFichier(c); !errors.Is(err, ErrAbsent) {
		t.Fatalf("fichier de repli encore present : %v", err)
	}
	if s, _ := a.lire(c); s != "troisieme" {
		t.Fatalf("lecture : %q", s)
	}
	if err := a.supprimer(c); err != nil {
		t.Fatal(err)
	}
	if _, err := a.lire(c); !errors.Is(err, ErrAbsent) {
		t.Fatalf("apres suppression : %v", err)
	}
}

// Sans repli (Windows) : l'erreur du coffre remonte telle quelle.
func TestSansRepli(t *testing.T) {
	f := &fauxCoffre{secrets: map[string]string{}, panne: true}
	a := avecRepli{coffre: f, actif: &atomic.Bool{}}
	c := Cle{ID: "repli3", Adresse: "c@example.invalid"}
	if err := a.ecrire(c, "x"); !errors.Is(err, errPanne) {
		t.Fatalf("ecriture : %v", err)
	}
	if _, err := lireFichier(c); !errors.Is(err, ErrAbsent) {
		t.Fatalf("fichier ecrit sans repli : %v", err)
	}
}

// Avec repli, un coffre indisponible n'empeche pas la suppression
// (sinon, sous Linux sans coffre, aucun compte ne pourrait etre supprime) ;
// le fichier de repli est bien efface. Sans repli, l'erreur remonte.
func TestRepliSuppressionCoffreIndisponible(t *testing.T) {
	f, a := nouveauRepli()
	c := Cle{ID: "repli4", Adresse: "e@example.invalid"}
	f.panne = true
	if err := a.ecrire(c, "secret"); err != nil {
		t.Fatal(err)
	}
	if err := a.supprimer(c); err != nil {
		t.Fatalf("suppression avec coffre indisponible : %v", err)
	}
	if _, err := lireFichier(c); !errors.Is(err, ErrAbsent) {
		t.Fatalf("fichier de repli encore present : %v", err)
	}
	sans := avecRepli{coffre: f, actif: &atomic.Bool{}}
	if err := sans.supprimer(c); !errors.Is(err, errPanne) {
		t.Fatalf("sans repli : %v", err)
	}
}

// Coffre revenu, mais l'ancien fichier de repli ne peut pas
// etre efface. Il serait lu avant le coffre : l'ancien secret l'emporterait
// en silence. L'ecriture echoue donc.
func TestRepliAncienFichierNonEfface(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("droits Unix d'un utilisateur ordinaire requis")
	}
	f, a := nouveauRepli()
	c := Cle{ID: "repli5", Adresse: "d@example.invalid"}
	f.panne = true
	if err := a.ecrire(c, "ancien"); err != nil {
		t.Fatal(err)
	}
	f.panne = false
	d, err := store.CheminAbsolu("secrets")
	if err != nil {
		t.Fatal(err)
	}
	os.Chmod(d, 0o500)
	defer os.Chmod(d, 0o700)
	if err := a.ecrire(c, "nouveau"); err == nil {
		s, _ := a.lire(c)
		t.Fatalf("ecriture acceptee, alors que la lecture rend %q", s)
	}
	os.Chmod(d, 0o700)
	if err := a.ecrire(c, "nouveau"); err != nil {
		t.Fatal(err)
	}
	if s, _ := a.lire(c); s != "nouveau" {
		t.Fatalf("lecture : %q", s)
	}
	a.supprimer(c)
}
