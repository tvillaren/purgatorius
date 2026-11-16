//go:build reel

package synchro

// Synchro reelle sur la boite du .env (lecture seule). Lancement explicite :
//
//	go test -tags reel -run TestSynchroReelle -v -timeout 40m ./internal/synchro
//
// N'affiche que des agregats. Le cache est cree dans un dossier temporaire.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/louis49/purgatorius/internal/cache"
	"github.com/louis49/purgatorius/internal/imap"
)

func lireEnv(t *testing.T) (string, string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", ".env"))
	if err != nil {
		t.Skipf(".env absent : %v", err)
	}
	env := map[string]string{}
	for _, l := range strings.Split(string(data), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(l), "="); ok && !strings.HasPrefix(k, "#") {
			env[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return env["GMAIL_ADRESSE"], env["GMAIL_MOT_DE_PASSE_APP"]
}

func TestSynchroReelle(t *testing.T) {
	adresse, mdp := lireEnv(t)
	c, err := cache.OuvrirChemin(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Fermer()
	opt := Options{Adresse: adresse, MotDePasse: mdp}
	journal := func(p Progression) {
		if p.Etape != EtapeEntetes || p.Faits%10000 == 0 || p.Faits == p.Total {
			t.Logf("%-10s %6d/%-6d deja %6d  %6.0f Mo  %5.0f msg/s  reste %v  annee %d",
				p.Etape, p.Faits, p.Total, p.DejaEnCache, float64(p.Volume)/1e6, p.Debit, p.Restant.Round(time.Second), p.AnneeLaPlusVieille)
		}
	}

	// 1. Interruption volontaire apres environ 3 000 messages.
	ctx, annuler := context.WithCancel(context.Background())
	_, err = Lancer(ctx, c, opt, func(p Progression) {
		journal(p)
		if p.Etape == EtapeEntetes && p.Faits >= 3000 {
			annuler()
		}
	})
	t.Logf("interruption : %v", err)
	r, _ := c.Resumer()
	t.Logf("apres interruption : %d messages en cache", r.Messages)

	// 2. Reprise jusqu'au bout.
	b, err := Lancer(context.Background(), c, opt, journal)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("reprise : %d lus, %d supprimes, %d sur le serveur, %v, %.1f Mo recus", b.Lus, b.Supprimes, b.Total, b.Duree.Round(time.Second), float64(b.Octets)/1e6)

	// 3. Relance immediate : rien a lire.
	b2, err := Lancer(context.Background(), c, opt, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("relance : %d lus en %v", b2.Lus, b2.Duree.Round(time.Second))

	r, _ = c.Resumer()
	t.Logf("cache : %d messages, %.2f Go, %d expediteurs, %d domaines, %d fils, %s -> %s", r.Messages, float64(r.Taille)/1e9,
		r.Expediteurs, r.Domaines, r.Fils, time.Unix(r.PremiereDate, 0).Format("2006-01"), time.Unix(r.DerniereDate, 0).Format("2006-01"))
	if r.Messages != b.Total || b2.Lus != 0 {
		t.Fatalf("cache incoherent : %d en cache, %d sur le serveur, relance %d", r.Messages, b.Total, b2.Lus)
	}
}

// Rattrapage CONDSTORE reel : Gmail accepte-t-il UID FETCH ... (CHANGEDSINCE)
// sur une session EXAMINE ? Lecture seule, agregats seulement :
//
//	go test -tags reel -run TestChangementsReels -v ./internal/synchro
func TestChangementsReels(t *testing.T) {
	adresse, mdp := lireEnv(t)
	ctx, annuler := context.WithTimeout(context.Background(), 2*time.Minute)
	defer annuler()
	cli, err := imap.Connecter(ctx, imap.HoteGmail)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Fermer()
	if err := cli.Authentifier(adresse, mdp); err != nil {
		t.Fatal(err)
	}
	ds, err := cli.Dossiers()
	if err != nil {
		t.Fatal(err)
	}
	tous := ""
	for _, d := range ds {
		if d.Special(`\All`) {
			tous = d.Nom
		}
	}
	et, err := cli.Examiner(tous)
	if err != nil || et.HighestModSeq == 0 {
		t.Fatalf("EXAMINE (CONDSTORE) : %+v %v", et, err)
	}
	for _, recul := range []uint64{0, 1000, 100000} {
		depuis := et.HighestModSeq - min(recul, et.HighestModSeq-1)
		n, lus := 0, 0
		debut := time.Now()
		if err := cli.Changements(depuis, func(m imap.Message) {
			n++
			if m.Lu {
				lus++
			}
		}); err != nil {
			t.Fatalf("CHANGEDSINCE %d : %v", depuis, err)
		}
		t.Logf("recul %d : %d messages changes (%d lus) en %v", recul, n, lus, time.Since(debut).Round(time.Millisecond))
	}
	_ = cli.Deconnecter()
}
