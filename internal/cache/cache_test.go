package cache

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/louis49/purgatorius/internal/imap"
)

func ouvrirTest(t *testing.T) *Cache {
	t.Helper()
	p := filepath.Join(t.TempDir(), "dossier avec espace", "cache.db")
	c, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Fermer() })
	// Droits Unix : sous Windows, Go les rapporte toujours en 0666 ; le fichier
	// y est protege par les ACL du profil de l'utilisateur.
	if st, err := os.Stat(p); err != nil || (runtime.GOOS != "windows" && st.Mode().Perm() != 0o600) {
		t.Fatalf("droits de la base : %v %v", st.Mode().Perm(), err)
	}
	return c
}

func msg(uid uint32, de string, taille int64) imap.Message {
	return imap.Message{UID: uid, Taille: taille, Date: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC), De: de, Objet: "RE: Facture 2024-03", GmThrID: uint64(uid),
		Libelles: []string{`\Inbox`, "Perso"}, Pieces: []imap.Piece{{Type: "pdf", Taille: 300}, {Type: "image", Taille: 50, Inline: true}}}
}

func TestEnregistrerEtResumer(t *testing.T) {
	c := ouvrirTest(t)
	if err := c.Enregistrer([]imap.Message{msg(1, "a@news.marque.fr", 1000), msg(2, "b@marque.fr", 500), msg(3, "c@webmail.example", 10)}); err != nil {
		t.Fatal(err)
	}
	// Reecriture du meme UID : remplace, ne duplique pas.
	if err := c.Enregistrer([]imap.Message{msg(3, "c@webmail.example", 20)}); err != nil {
		t.Fatal(err)
	}
	r, err := c.Resumer()
	if err != nil || r.Messages != 3 || r.Taille != 1520 || r.Domaines != 2 || r.Expediteurs != 3 {
		t.Fatalf("resume : %+v %v", r, err)
	}
	var n int
	c.db.QueryRow("SELECT COUNT(*) FROM libelles").Scan(&n)
	if n != 6 {
		t.Fatalf("libelles : %d, attendu 6", n)
	}
	var pj int64
	var norm string
	c.db.QueryRow("SELECT pj_taille, objet_norm FROM messages WHERE uid = 1").Scan(&pj, &norm)
	if pj != 300 || norm != "facture #-#" {
		t.Fatalf("pj_taille %d (attendu 300, inline exclu), objet_norm %q", pj, norm)
	}
	if err := c.Supprimer([]uint32{2}); err != nil {
		t.Fatal(err)
	}
	c.db.QueryRow("SELECT COUNT(*) FROM libelles").Scan(&n)
	if n != 4 {
		t.Fatalf("suppression en cascade des libelles : %d", n)
	}
	uids, _ := c.UIDs()
	if len(uids) != 2 || !uids[1] || !uids[3] {
		t.Fatalf("UIDs : %v", uids)
	}
}

func TestCategoriesEtMeta(t *testing.T) {
	c := ouvrirTest(t)
	c.Enregistrer([]imap.Message{msg(1, "a@x.example", 1), msg(2, "b@x.example", 1)})
	if err := c.DefinirCategories(map[uint32]string{2: "promotions"}); err != nil {
		t.Fatal(err)
	}
	var cat string
	c.db.QueryRow("SELECT categorie FROM messages WHERE uid = 1").Scan(&cat)
	if cat != "principale" {
		t.Fatalf("categorie par defaut : %q", cat)
	}
	if v, _ := c.Meta("uidvalidity"); v != "" {
		t.Fatalf("meta absente : %q", v)
	}
	c.EcrireMeta("uidvalidity", "11")
	c.EcrireMeta("uidvalidity", "12")
	if v, _ := c.Meta("uidvalidity"); v != "12" {
		t.Fatalf("meta : %q", v)
	}
}

func TestReouvertureSansMigrationRejouee(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cache.db")
	c, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	c.Enregistrer([]imap.Message{msg(1, "a@x.example", 1)})
	c.Fermer()
	c, err = OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Fermer()
	if r, _ := c.Resumer(); r.Messages != 1 {
		t.Fatalf("donnees perdues a la reouverture : %+v", r)
	}
}

func TestDomaines(t *testing.T) {
	for adr, att := range map[string][2]string{
		"x@news.marque.fr": {"news.marque.fr", "marque.fr"},
		"x@a.b.gouv.fr":    {"a.b.gouv.fr", "b.gouv.fr"},
		"x@bbc.co.uk":      {"bbc.co.uk", "bbc.co.uk"},
		"x@mail.bbc.co.uk": {"mail.bbc.co.uk", "bbc.co.uk"},
		"sans-arobase":     {"", ""},
	} {
		h, d := Domaines(adr)
		if h != att[0] || d != att[1] {
			t.Errorf("%s : %q %q, attendu %q", adr, h, d, att)
		}
	}
}
