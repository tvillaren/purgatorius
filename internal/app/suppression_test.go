package app

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/louis49/purgatorius/internal/comptes"
	"github.com/louis49/purgatorius/internal/imap"
	"github.com/louis49/purgatorius/internal/imap/imaptest"
	"github.com/louis49/purgatorius/internal/secrets"
	"github.com/louis49/purgatorius/internal/store"
)

// Supprimer un compte en pleine synchro attend la fin de celle-ci ;
// ensuite plus rien ne recree son cache, et secret et registre sont effaces.
func TestSupprimerCompteEnPleineSynchro(t *testing.T) {
	b := &imaptest.Boite{UIDValidity: 1, Lenteur: 2 * time.Second}
	for i := uint32(1); i <= 6; i++ {
		b.Messages = append(b.Messages, imaptest.Message{UID: i, GmMsgID: uint64(100 + i), Taille: 100, Date: time.Date(2024, 1, int(i), 0, 0, 0, 0, time.UTC),
			Libelles: []string{`\Inbox`}, De: "a@x.example", Objet: "objet", ModSeq: 1})
	}
	a := NewApp()
	entetes := make(chan struct{}, 1)
	a.emettre = func(nom string, d any) {
		if p, ok := d.(Progression); ok && nom == EvProgression && p.Etape == "entetes" {
			select {
			case entetes <- struct{}{}:
			default:
			}
		}
	}
	a.connecterIMAP = func(ctx context.Context) (*imap.Client, error) {
		c := b.Connecter(t)
		// Comme imap.Connecter, mais l'arret prend du temps (reseau lent) : sans
		// attente, SupprimerCompte effacerait sous une synchro encore vivante.
		context.AfterFunc(ctx, func() { time.Sleep(300 * time.Millisecond); c.Fermer() })
		return c, nil
	}
	cv, err := a.AjouterCompte(NouveauCompte{Nom: "Suppr", Adresse: "suppr@example.invalid", MotDePasse: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.LancerSynchro(cv.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entetes:
	case <-time.After(5 * time.Second):
		t.Fatal("la synchro n'a pas atteint la lecture des en-tetes")
	}

	if err := a.SupprimerCompte(cv.ID); err != nil {
		t.Fatal(err)
	}
	if a.SynchroEnCours(cv.ID) {
		t.Fatal("SupprimerCompte est revenu avant la fin de la synchro")
	}
	time.Sleep(400 * time.Millisecond) // une goroutine encore vivante aurait le temps d'ecrire
	for _, d := range []string{"cache/comptes/" + cv.ID, "travail/comptes/" + cv.ID} {
		p, _ := store.CheminAbsolu(d)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s encore present : %v", d, err)
		}
	}
	if _, err := secrets.Lire(secrets.Cle{ID: cv.ID, Adresse: cv.Adresse}); !errors.Is(err, secrets.ErrAbsent) {
		t.Errorf("secret : %v", err)
	}
	if r, _ := comptes.Lire(); func() bool { _, ok := r.Compte(cv.ID); return ok }() {
		t.Errorf("compte encore au registre")
	}
}

// Changer le mot de passe d'application. Refuse par Gmail : rien n'est
// remplace. Accepte : il remplace l'ancien.
func TestChangerMotDePasse(t *testing.T) {
	b := &imaptest.Boite{UIDValidity: 1, MotDePasse: "nouveaumotdepass"}
	a := NewApp()
	a.connecterIMAP = func(ctx context.Context) (*imap.Client, error) { return b.Connecter(t), nil }
	cv, err := a.AjouterCompte(NouveauCompte{Adresse: "mdp@example.invalid", MotDePasse: "ancienmotdepasse"})
	if err != nil {
		t.Fatal(err)
	}
	cle := secrets.Cle{ID: cv.ID, Adresse: cv.Adresse}
	if r := a.ChangerMotDePasse(cv.ID, "mauvaismotdepass"); r.OK || r.Code != "authentification" {
		t.Fatalf("mot de passe refuse : %+v", r)
	}
	if s, _ := secrets.Lire(cle); s != "ancienmotdepasse" {
		t.Fatalf("ancien mot de passe remplace malgre le refus : %q", s)
	}
	if r := a.ChangerMotDePasse(cv.ID, "nouv eaum otde pass"); !r.OK {
		t.Fatalf("mot de passe accepte : %+v", r)
	}
	if s, _ := secrets.Lire(cle); s != "nouveaumotdepass" {
		t.Fatalf("secret apres changement : %q", s)
	}
	if r := a.ChangerMotDePasse("inconnu", "x"); r.OK {
		t.Fatal("compte inconnu accepte")
	}
}

// present : le chemin relatif du dossier de donnees existe.
func present(t *testing.T, rel string) bool {
	t.Helper()
	p, err := store.CheminAbsolu(rel)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(p)
	return err == nil
}

func auRegistre(t *testing.T, id string) bool {
	t.Helper()
	r, err := comptes.Lire()
	if err != nil {
		t.Fatal(err)
	}
	_, ok := r.Compte(id)
	return ok
}

// Le secret ne peut pas etre efface. Rien d'autre n'est detruit (le
// Purgatoire surtout), le compte reste ; un nouvel essai aboutit.
func TestSupprimerCompteSecretEnEchec(t *testing.T) {
	a := NewApp()
	cv, err := a.AjouterCompte(NouveauCompte{Nom: "Coffre", Adresse: "coffre@example.invalid", MotDePasse: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.EtatPurgatoire(cv.ID); err != nil { // cree cache et Purgatoire
		t.Fatal(err)
	}
	a.supprimerSecret = func(secrets.Cle) error { return errors.New("trousseau verrouille") }
	if err := a.SupprimerCompte(cv.ID); err == nil {
		t.Fatal("suppression annoncee reussie")
	}
	if !present(t, "travail/comptes/"+cv.ID+"/purgatoire.db") || !present(t, "cache/comptes/"+cv.ID) {
		t.Fatal("donnees detruites alors que la suppression a echoue")
	}
	if !auRegistre(t, cv.ID) {
		t.Fatal("compte retire du registre")
	}
	a.supprimerSecret = secrets.Supprimer
	if err := a.SupprimerCompte(cv.ID); err != nil {
		t.Fatalf("nouvel essai : %v", err)
	}
	if present(t, "travail/comptes/"+cv.ID) || present(t, "cache/comptes/"+cv.ID) || auRegistre(t, cv.ID) {
		t.Fatal("compte incompletement supprime")
	}
}

// Une synchro demandee pendant une suppression (ici bloquee sur le
// coffre) est refusee, comme la carte : rien ne recree les fichiers du compte.
func TestSynchroRefuseePendantSuppression(t *testing.T) {
	b := &imaptest.Boite{UIDValidity: 1}
	for i := uint32(1); i <= 3; i++ {
		b.Messages = append(b.Messages, imaptest.Message{UID: i, GmMsgID: uint64(200 + i), Taille: 100, Date: time.Date(2024, 1, int(i), 0, 0, 0, 0, time.UTC),
			Libelles: []string{`\Inbox`}, De: "a@x.example", Objet: "objet", ModSeq: 1})
	}
	a := NewApp()
	a.connecterIMAP = func(ctx context.Context) (*imap.Client, error) { return b.Connecter(t), nil }
	cv, err := a.AjouterCompte(NouveauCompte{Nom: "Course", Adresse: "course@example.invalid", MotDePasse: "x"})
	if err != nil {
		t.Fatal(err)
	}
	dans, reprendre := make(chan struct{}), make(chan struct{})
	a.supprimerSecret = func(k secrets.Cle) error {
		close(dans)
		<-reprendre
		return secrets.Supprimer(k)
	}
	fin := make(chan error, 1)
	go func() { fin <- a.SupprimerCompte(cv.ID) }()
	<-dans
	errSynchro := a.LancerSynchro(cv.ID)
	_, errCarte := a.EtatPurgatoire(cv.ID)
	close(reprendre)
	if err := <-fin; err != nil {
		t.Fatal(err)
	}
	if errSynchro == nil || errCarte == nil {
		t.Fatalf("acceptes pendant la suppression : synchro %v, carte %v", errSynchro, errCarte)
	}
	time.Sleep(300 * time.Millisecond) // une synchro lancee aurait le temps d'ecrire
	if present(t, "cache/comptes/"+cv.ID) || present(t, "travail/comptes/"+cv.ID) || a.SynchroEnCours(cv.ID) {
		t.Fatal("fichiers du compte recrees apres sa suppression")
	}
}

// Un identifiant hors registre est refuse, sans creer de fichiers.
func TestIdentifiantInconnuRefuse(t *testing.T) {
	a := NewApp()
	id := "0123456789abcdef"
	if _, err := a.EtatPurgatoire(id); err == nil {
		t.Error("carte d'un compte inconnu acceptee")
	}
	if _, err := a.ResumeCompte(id); err == nil {
		t.Error("resume d'un compte inconnu accepte")
	}
	if present(t, "cache/comptes/"+id) || present(t, "travail/comptes/"+id) {
		t.Error("fichiers crees pour un compte inconnu")
	}
}
