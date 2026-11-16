package cache

import (
	"testing"
	"time"

	"github.com/louis49/purgatorius/internal/imap"
)

func remplirListe(t *testing.T) *Cache {
	t.Helper()
	c := ouvrirTest(t)
	j := func(jour int) time.Time { return time.Date(2024, 3, jour, 0, 0, 0, 0, time.UTC) }
	msgs := []imap.Message{
		{UID: 1, GmMsgID: 0x10, De: "a@x.example", NomDe: "Zoé", Objet: "un", Taille: 300, Date: j(1), Lu: true, Libelles: []string{`\Inbox`, "Factures"}},
		{UID: 2, GmMsgID: 0x20, De: "b@x.example", Objet: "deux", Taille: 900, Date: j(2), Libelles: []string{`\Inbox`, `\Important`}},
		{UID: 3, GmMsgID: 0x30, De: "c@x.example", NomDe: "alice", Objet: "trois", Taille: 300, Date: j(3), Lu: true, Libelles: []string{`\Inbox`},
			Pieces: []imap.Piece{{Type: "pdf", Taille: 200}}},
		{UID: 4, GmMsgID: 0x40, De: "d@y.example", Objet: "quatre", Taille: 50, Date: j(4), Libelles: []string{`\Sent`}, Destinataires: []string{"z@w.example"}},
	}
	if err := c.Enregistrer(msgs); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestIndexListe(t *testing.T) {
	c := remplirListe(t)
	dom := []Filtre{{Axe: "domaine", Valeur: "x.example"}}
	cas := []struct {
		nom  string
		r    RequeteListe
		uids []uint32
	}{
		// Taille egale (1 et 3) : le plus recent d'abord.
		{"taille decroissante", RequeteListe{Chemin: dom, Tri: "taille", Desc: true}, []uint32{2, 3, 1}},
		{"date croissante", RequeteListe{Chemin: dom, Tri: "date"}, []uint32{1, 2, 3}},
		// Nom absent : l'adresse sert de cle ; casse ignoree.
		{"expediteur", RequeteListe{Chemin: dom, Tri: "expediteur"}, []uint32{3, 2, 1}},
		{"non lus", RequeteListe{Chemin: dom, Lecture: "nonlu", Tri: "taille", Desc: true}, []uint32{2}},
		{"lus", RequeteListe{Lecture: "lu", Tri: "date", Desc: true}, []uint32{3, 1}},
		{"envoyes", RequeteListe{Boite: "envoye", Tri: "destinataire"}, []uint32{4}},
	}
	for _, k := range cas {
		idx, err := c.IndexListe(k.r)
		if err != nil {
			t.Fatalf("%s : %v", k.nom, err)
		}
		if len(idx.UIDs) != len(k.uids) || len(idx.Tailles) != len(k.uids) {
			t.Fatalf("%s : %+v, attendu %v", k.nom, idx, k.uids)
		}
		for i := range k.uids {
			if idx.UIDs[i] != k.uids[i] {
				t.Fatalf("%s : %v, attendu %v", k.nom, idx.UIDs, k.uids)
			}
		}
	}
	for _, r := range []RequeteListe{
		{Tri: "taille; DROP TABLE messages"},
		{Tri: "taille", Lecture: "peut-etre"},
		{Tri: "taille", Boite: "spam"},
		{Tri: "taille", Chemin: []Filtre{{Axe: "inconnu"}}},
	} {
		if _, err := c.IndexListe(r); err == nil {
			t.Errorf("requete invalide acceptee : %+v", r)
		}
	}
}

func TestLignes(t *testing.T) {
	c := remplirListe(t)
	l, err := c.Lignes([]uint32{3, 99, 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(l) != 2 || l[0].UID != 3 || l[1].UID != 1 {
		t.Fatalf("ordre ou UID absent mal geres : %+v", l)
	}
	if l[0].PJ != 200 || l[0].Nom != "alice" || len(l[0].Libelles) != 0 || l[0].GmMsgID != "30" {
		t.Fatalf("ligne 3 : %+v", l[0])
	}
	if !l[1].Lu || len(l[1].Libelles) != 1 || l[1].Libelles[0] != "Factures" {
		t.Fatalf("libelles systeme non exclus ou lecture : %+v", l[1])
	}
	if _, err := c.Lignes(make([]uint32, MaxLignes+1)); err == nil {
		t.Fatal("lot trop gros accepte")
	}
}

func TestCarteFiltreLecture(t *testing.T) {
	c := remplirListe(t)
	n, err := c.Carte(RequeteCarte{Axes: []string{"domaine"}, Metrique: "taille", Lecture: "nonlu"})
	if err != nil || n.Nombre != 2 || n.Taille != 950 {
		t.Fatalf("carte des non lus : %+v %v", n, err)
	}
	d, err := c.DetailCarte(RequeteDetail{Metrique: "taille", Lecture: "lu"})
	if err != nil || d.Nombre != 2 {
		t.Fatalf("detail des lus : %+v %v", d, err)
	}
}
