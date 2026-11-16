package synchro

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/louis49/purgatorius/internal/cache"
	"github.com/louis49/purgatorius/internal/imap"
	"github.com/louis49/purgatorius/internal/imap/imaptest"
)

// boiteTest : cinq messages, UID 1 a 8, compteurs CONDSTORE 10 a 14.
func boiteTest() *imaptest.Boite {
	j := func(jour int) time.Time { return time.Date(2024, 3, jour, 10, 0, 0, 0, time.UTC) }
	return &imaptest.Boite{
		UIDValidity: 7,
		Messages: []imaptest.Message{
			{UID: 1, GmMsgID: 0x101, Taille: 1000, Date: j(1), Lu: true, Libelles: []string{`\Inbox`}, De: "Alice <alice@x.example>", Objet: "un", ModSeq: 10},
			{UID: 2, GmMsgID: 0x102, Taille: 2000, Date: j(2), Libelles: []string{`\Inbox`, "Factures"}, De: "promo@y.example", Objet: "deux", ModSeq: 11},
			{UID: 3, GmMsgID: 0x103, Taille: 3000, Date: j(3), Libelles: []string{`\Inbox`}, De: "bob@x.example", Objet: "trois", ModSeq: 12},
			{UID: 5, GmMsgID: 0x105, Taille: 5000, Date: j(5), Lu: true, Libelles: []string{`\Sent`}, De: "moi@example.invalid", Objet: "cinq", ModSeq: 13},
			{UID: 8, GmMsgID: 0x108, Taille: 8000, Date: j(8), Libelles: []string{`\Inbox`}, De: "carl@z.example", Objet: "huit", ModSeq: 14},
		},
		Categories: map[string][]uint32{"category:promotions": {2}},
	}
}

func cacheTest(t *testing.T) *cache.Cache {
	t.Helper()
	c, err := cache.OuvrirChemin(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Fermer() })
	return c
}

// lancer : une synchro complete contre la fausse boite (lots de 2), puis
// verification que rien n'a pu modifier la boite.
func lancer(t *testing.T, c *cache.Cache, b *imaptest.Boite, opt Options) (Bilan, error) {
	t.Helper()
	opt.Adresse, opt.MotDePasse, opt.Lot = "moi@example.invalid", "x", 2
	opt.Connecter = func(context.Context) (*imap.Client, error) { return b.Connecter(t), nil }
	bilan, err := Lancer(context.Background(), c, opt, nil)
	b.VerifierLectureSeule(t)
	return bilan, err
}

func nombre(t *testing.T, c *cache.Cache) int {
	t.Helper()
	r, err := c.Resumer()
	if err != nil {
		t.Fatal(err)
	}
	return r.Messages
}

func TestSynchroComplete(t *testing.T) {
	b, c := boiteTest(), cacheTest(t)
	bilan, err := lancer(t, c, b, Options{})
	if err != nil || bilan.Lus != 5 || bilan.Total != 5 {
		t.Fatalf("bilan : %+v %v", bilan, err)
	}
	if n := nombre(t, c); n != 5 {
		t.Fatalf("messages en cache : %d", n)
	}
	n, err := c.Carte(cache.RequeteCarte{Axes: []string{"categorie"}, Metrique: "nombre"})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range n.Noeuds {
		if x.Cle == "promotions" && x.Nombre != 1 {
			t.Fatalf("categorie promotions : %+v", x)
		}
	}
	if m, _ := c.Meta("highestmodseq"); m != "14" {
		t.Fatalf("compteur CONDSTORE enregistre : %q", m)
	}
	// Une seconde synchro sans changement ne relit rien.
	if bilan, err = lancer(t, c, b, Options{}); err != nil || bilan.Lus != 0 || bilan.Actualises != 0 {
		t.Fatalf("seconde synchro : %+v %v", bilan, err)
	}
}

// Coupure au milieu : ce qui est lu reste, la reprise lit le reste.
func TestSynchroRepriseApresCoupure(t *testing.T) {
	b, c := boiteTest(), cacheTest(t)
	b.CoupureApres = 2 // le second lot
	if _, err := lancer(t, c, b, Options{}); err == nil {
		t.Fatal("coupure non signalee")
	}
	if n := nombre(t, c); n != 2 {
		t.Fatalf("messages lus avant la coupure : %d", n)
	}
	// Pendant la coupure, un message deja lu (8, le plus recent) change.
	b.Modifier(8, func(m *imaptest.Message) { m.Lu = true })
	b.CoupureApres = 0
	bilan, err := lancer(t, c, b, Options{})
	if err != nil || bilan.Lus != 3 || bilan.Actualises != 1 {
		t.Fatalf("reprise : %+v %v", bilan, err)
	}
	l, _ := c.Lignes([]uint32{8})
	if len(l) != 1 || !l[0].Lu {
		t.Fatalf("drapeau rattrape apres reprise : %+v", l)
	}
}

// Un message lu, relabellise ou deplace dans Gmail est rattrape, sans
// relire les en-tetes.
func TestSynchroRattrapeDrapeaux(t *testing.T) {
	b, c := boiteTest(), cacheTest(t)
	if _, err := lancer(t, c, b, Options{}); err != nil {
		t.Fatal(err)
	}
	b.Modifier(3, func(m *imaptest.Message) { m.Lu = true; m.Libelles = append(m.Libelles, "Perso") })
	b.Modifier(1, func(m *imaptest.Message) { m.Libelles = []string{`\Sent`} })
	avant := len(b.Commandes())
	bilan, err := lancer(t, c, b, Options{})
	if err != nil || bilan.Lus != 0 || bilan.Actualises != 2 {
		t.Fatalf("bilan : %+v %v", bilan, err)
	}
	session := strings.Join(b.Commandes()[avant:], "\n")
	if !strings.Contains(session, "(CHANGEDSINCE 14)") || strings.Contains(session, "RFC822.SIZE") {
		t.Fatalf("rattrapage attendu par CHANGEDSINCE seulement :\n%s", session)
	}
	l, err := c.Lignes([]uint32{1, 3})
	if err != nil || len(l) != 2 {
		t.Fatalf("lignes : %+v %v", l, err)
	}
	if l[0].Boite != "envoye" || !l[1].Lu || !slices.Contains(l[1].Libelles, "Perso") {
		t.Fatalf("drapeaux, boite ou libelles non rattrapes : %+v", l)
	}
	if m, _ := c.Meta("highestmodseq"); m != "16" {
		t.Fatalf("compteur apres rattrapage : %q", m)
	}
}

// UIDVALIDITY changee. AvantVidage est appele avant de vider ; le cache
// est relu sous les nouveaux UID ; le Purgatoire (par X-GM-MSGID) reste.
func TestSynchroUIDValidityChangee(t *testing.T) {
	b, c := boiteTest(), cacheTest(t)
	if _, err := lancer(t, c, b, Options{}); err != nil {
		t.Fatal(err)
	}
	if lot, err := c.EnvoyerPurgatoire([]uint32{2}); err != nil || lot.Nombre != 1 {
		t.Fatalf("envoi : %+v %v", lot, err)
	}
	b.UIDValidity = 8
	for i := range b.Messages {
		b.Messages[i].UID += 100
	}
	b.Categories = nil

	// AvantVidage en echec : rien n'est vide.
	refus := errors.New("registre illisible")
	if _, err := lancer(t, c, b, Options{AvantVidage: func() error { return refus }}); !errors.Is(err, refus) {
		t.Fatalf("erreur d'AvantVidage : %v", err)
	}
	if n := nombre(t, c); n != 5 {
		t.Fatalf("cache vide malgre l'echec : %d", n)
	}

	appels := 0
	bilan, err := lancer(t, c, b, Options{AvantVidage: func() error { appels++; return nil }})
	if err != nil || appels != 1 || bilan.Lus != 5 {
		t.Fatalf("relecture : appels=%d %+v %v", appels, bilan, err)
	}
	idx, err := c.IndexListe(cache.RequeteListe{Purgatoire: true, Tri: "date"})
	if err != nil || len(idx.UIDs) != 1 || idx.UIDs[0] != 102 {
		t.Fatalf("Purgatoire apres renumerotation : %+v %v", idx, err)
	}
}

// De bout en bout : des reponses non sollicitees glissees dans les lots
// n'abiment pas le cache.
func TestSynchroIgnoreLesFetchNonSollicites(t *testing.T) {
	b, c := boiteTest(), cacheTest(t)
	b.Intrus = "* 1 FETCH (UID 1 FLAGS () MODSEQ (40))\r\n* 4 FETCH (FLAGS (\\Seen))"
	if _, err := lancer(t, c, b, Options{}); err != nil {
		t.Fatal(err)
	}
	l, err := c.Lignes([]uint32{1})
	if err != nil || len(l) != 1 || l[0].Taille != 1000 || l[0].GmMsgID != "101" || !l[0].Lu {
		t.Fatalf("message 1 abime : %+v %v", l, err)
	}
	if n := nombre(t, c); n != 5 {
		t.Fatalf("messages en cache : %d", n)
	}
}

// Un message du Purgatoire supprime dans Gmail en sort a la synchro
// suivante : s'il revenait (restauration), il ne serait plus au Purgatoire.
func TestSynchroNettoieLePurgatoire(t *testing.T) {
	b, c := boiteTest(), cacheTest(t)
	if _, err := lancer(t, c, b, Options{}); err != nil {
		t.Fatal(err)
	}
	c.EnvoyerPurgatoire([]uint32{5})
	cinq := b.Messages[3]
	b.Messages = slices.Delete(b.Messages, 3, 4)
	if _, err := lancer(t, c, b, Options{}); err != nil {
		t.Fatal(err)
	}
	cinq.UID = 50
	b.Messages = append(b.Messages, cinq)
	if _, err := lancer(t, c, b, Options{}); err != nil {
		t.Fatal(err)
	}
	if idx, _ := c.IndexListe(cache.RequeteListe{Purgatoire: true, Tri: "date"}); len(idx.UIDs) != 0 {
		t.Fatalf("message revenu encore au Purgatoire : %+v", idx)
	}
}

// Un lot incomplet (le serveur omet un message demande) ne doit pas
// faire perdre son entree du Purgatoire : le nettoyage n'a lieu que si le
// cache contient tous les messages du serveur. Le message, relu ensuite, y
// est toujours.
func TestSynchroIncompleteNeNettoiePasLePurgatoire(t *testing.T) {
	b, c := boiteTest(), cacheTest(t)
	if _, err := lancer(t, c, b, Options{}); err != nil {
		t.Fatal(err)
	}
	if lot, err := c.EnvoyerPurgatoire([]uint32{2}); err != nil || lot.Nombre != 1 {
		t.Fatalf("envoi : %+v %v", lot, err)
	}
	// Cache sans le message 2 (comme apres une recreation), que le serveur
	// omet a la relecture.
	if err := c.Supprimer([]uint32{2}); err != nil {
		t.Fatal(err)
	}
	b.Omis = map[uint32]bool{2: true}
	if _, err := lancer(t, c, b, Options{}); err != nil {
		t.Fatal(err)
	}
	b.Omis = nil
	if _, err := lancer(t, c, b, Options{}); err != nil {
		t.Fatal(err)
	}
	idx, err := c.IndexListe(cache.RequeteListe{Purgatoire: true, Tri: "date"})
	if err != nil || len(idx.UIDs) != 1 || idx.UIDs[0] != 2 {
		t.Fatalf("Purgatoire apres relecture : %+v %v", idx, err)
	}
}
