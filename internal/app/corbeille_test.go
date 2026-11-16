package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/louis49/purgatorius/internal/imap"
	"github.com/louis49/purgatorius/internal/imap/imaptest"
)

// De bout en bout : synchro, envoi au Purgatoire, mise a la corbeille (avec
// son avancement) ; pendant, la synchro est refusee ; ensuite la carte ne
// montre plus ces messages.
func TestMettreALaCorbeille(t *testing.T) {
	b := &imaptest.Boite{UIDValidity: 3}
	for i := uint32(1); i <= 4; i++ {
		b.Messages = append(b.Messages, imaptest.Message{UID: i, GmMsgID: uint64(300 + i), Taille: 100 * int64(i), Date: time.Date(2024, 1, int(i), 0, 0, 0, 0, time.UTC),
			Libelles: []string{`\Inbox`}, De: "a@x.example", Objet: "objet", ModSeq: 1})
	}
	a := NewApp()
	a.connecterIMAP = func(ctx context.Context) (*imap.Client, error) { return b.Connecter(t), nil }
	var mu sync.Mutex
	var avancement []ProgressionCorbeille
	fin := make(chan FinSynchro, 1)
	a.emettre = func(nom string, d any) {
		switch x := d.(type) {
		case ProgressionCorbeille:
			mu.Lock()
			avancement = append(avancement, x)
			mu.Unlock()
		case FinSynchro:
			if nom == EvTerminee || nom == EvErreur {
				fin <- x
			}
		}
	}
	cv, err := a.AjouterCompte(NouveauCompte{Nom: "Corbeille", Adresse: "corbeille@example.invalid", MotDePasse: "x"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.fermerLecteur(cv.ID) })
	if err := a.LancerSynchro(cv.ID); err != nil {
		t.Fatal(err)
	}
	if f := <-fin; f.Code != "" {
		t.Fatalf("synchro : %+v", f)
	}
	if lot, err := a.EnvoyerPurgatoire(cv.ID, []uint32{1, 3}); err != nil || lot.Nombre != 2 {
		t.Fatalf("envoi : %+v %v", lot, err)
	}

	if r := a.MettreALaCorbeille(cv.ID, 3); r.Code != "ecart" || r.Deplaces != 0 {
		t.Fatalf("ecart : %+v", r)
	}
	r := a.MettreALaCorbeille(cv.ID, 2)
	if r.Code != "" || r.Deplaces != 2 || r.Taille != 400 {
		t.Fatalf("corbeille : %+v", r)
	}
	mu.Lock()
	if n := len(avancement); n == 0 || avancement[n-1] != (ProgressionCorbeille{Compte: cv.ID, Faits: 2, Total: 2}) {
		t.Errorf("avancement : %+v", avancement)
	}
	mu.Unlock()
	if len(b.Corbeille) != 2 {
		t.Fatalf("dans la corbeille : %+v", b.Corbeille)
	}
	e, err := a.EtatPurgatoire(cv.ID)
	if err != nil || e.Nombre != 0 || e.TotalNombre != 2 {
		t.Fatalf("carte apres : %+v %v", e, err)
	}

	// Pendant une mise a la corbeille, synchro et suppression attendent.
	a.mu.Lock()
	a.corbeilles[cv.ID] = true
	a.mu.Unlock()
	if err := a.LancerSynchro(cv.ID); err == nil {
		t.Error("synchro acceptee pendant la mise a la corbeille")
	}
	if err := a.SupprimerCompte(cv.ID); err == nil {
		t.Error("suppression acceptee pendant la mise a la corbeille")
	}
	a.mu.Lock()
	delete(a.corbeilles, cv.ID)
	a.mu.Unlock()
}
