package cache

import (
	"fmt"
	"slices"
	"testing"

	"github.com/louis49/purgatorius/internal/imap"
)

func remplirLibelles(t *testing.T) *Cache {
	t.Helper()
	c := ouvrirTest(t)
	msgs := []imap.Message{
		{UID: 1, GmMsgID: 1, De: "a@x.example", Taille: 500, Libelles: []string{`\Inbox`, "Factures", "Maison", "Factures"}},
		{UID: 2, GmMsgID: 2, De: "b@y.example", Taille: 500, Lu: true, Libelles: []string{`\Sent`, "Factures"}},
		{UID: 3, GmMsgID: 3, De: "a@x.example", Taille: 100, Lu: true, Libelles: []string{`\Inbox`, "Maison", "Étés d'été"}},
		{UID: 4, GmMsgID: 4, De: "a@x.example", Taille: 50, Libelles: []string{`\Inbox`, `\Important`}},
		{UID: 5, GmMsgID: 5, De: "a@x.example", Taille: 25},
	}
	if err := c.Enregistrer(msgs); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCarteLibelles(t *testing.T) {
	c := remplirLibelles(t)
	for _, metrique := range []string{"nombre", "taille"} {
		n, err := c.Carte(RequeteCarte{Axes: []string{"libelle"}, Metrique: metrique})
		if err != nil || n.Nombre != 5 || n.Taille != 1175 || len(n.Noeuds) != 4 {
			t.Fatalf("totaux uniques et libelles personnels : %+v %v", n, err)
		}
		attendus := map[string]Noeud{
			"Factures":   {Nombre: 2, Taille: 1000},
			"Maison":     {Nombre: 2, Taille: 600},
			"Étés d'été": {Nombre: 1, Taille: 100},
			"":           {Nombre: 2, Taille: 75},
		}
		for _, b := range n.Noeuds {
			att, ok := attendus[b.Cle]
			if !ok || b.Nombre != att.Nombre || b.Taille != att.Taille {
				t.Fatalf("bloc %q : %+v, attendu %+v", b.Cle, b, att)
			}
		}
	}
	// Les libelles se combinent avec les axes et filtres existants.
	n, err := c.Carte(RequeteCarte{Axes: []string{"domaine", "libelle"}, Chemin: []Filtre{{Axe: "domaine", Valeur: "x.example"}}, Boite: "recu", Lecture: "lu"})
	if err != nil || n.Nombre != 1 || n.Taille != 100 || len(n.Noeuds) != 2 {
		t.Fatalf("libelles apres un domaine, recus et lus : %+v %v", n, err)
	}
	n, err = c.Carte(RequeteCarte{Axes: []string{"libelle", "domaine"}, Chemin: []Filtre{{Axe: "libelle", Valeur: "Factures"}}, Boite: "envoye"})
	if err != nil || n.Nombre != 1 || len(n.Noeuds) != 1 || n.Noeuds[0].Cle != "y.example" {
		t.Fatalf("domaines apres un libelle, envoyes : %+v %v", n, err)
	}
}

func TestFiltresLibelles(t *testing.T) {
	c := remplirLibelles(t)
	cas := []struct {
		nom    string
		filtre Filtre
		uids   []uint32
		taille int64
	}{
		{"un libelle", Filtre{Valeur: "Factures"}, []uint32{1, 2}, 1000},
		{"sans libelle personnel", Filtre{}, []uint32{4, 5}, 75},
		{"accents et apostrophe", Filtre{Valeur: "Étés d'été"}, []uint32{3}, 100},
		{"libelle systeme exclu", Filtre{Valeur: `\Inbox`}, []uint32{}, 0},
		{"autres avec chevauchement", Filtre{Exclure: []string{"Factures"}}, []uint32{1, 3, 4, 5}, 675},
		{"sans libelle deja affiche", Filtre{Exclure: []string{"Factures", ""}}, []uint32{1, 3}, 600},
		{"tous affiches", Filtre{Exclure: []string{"Factures", "Maison", "Étés d'été", ""}}, []uint32{}, 0},
	}
	for _, tt := range cas {
		t.Run(tt.nom, func(t *testing.T) {
			f := tt.filtre
			f.Axe = "libelle"
			chemin := []Filtre{f}
			n, err := c.Carte(RequeteCarte{Axes: []string{"libelle"}, Chemin: chemin})
			if err != nil || n.Axe != "message" || n.Nombre != len(tt.uids) || n.Taille != tt.taille {
				t.Fatalf("zoom : %+v %v", n, err)
			}
			d, err := c.DetailCarte(RequeteDetail{Chemin: chemin})
			if err != nil || d.Nombre != len(tt.uids) || d.Taille != tt.taille {
				t.Fatalf("detail : %+v %v", d, err)
			}
			idx, err := c.IndexListe(RequeteListe{Chemin: chemin, Tri: "taille"})
			slices.Sort(idx.UIDs)
			if err != nil || !slices.Equal(idx.UIDs, tt.uids) {
				t.Fatalf("liste : %+v %v, attendu %v", idx, err, tt.uids)
			}
		})
	}
}

func TestLibellesAutresEtPurgatoire(t *testing.T) {
	c := remplirLibelles(t)
	avant := maxBlocs
	maxBlocs = 1
	defer func() { maxBlocs = avant }()
	n, err := c.Carte(RequeteCarte{Axes: []string{"libelle"}, Metrique: "taille"})
	if err != nil || len(n.Noeuds) != 2 || n.Noeuds[0].Cle != "Factures" {
		t.Fatalf("carte regroupee : %+v %v", n, err)
	}
	b := n.Noeuds[1]
	if b.Autres != 3 || b.Nombre != 4 || b.Taille != 675 || !slices.Equal(b.Exclure, []string{"Factures"}) {
		t.Fatalf("Autres doit compter chaque message une fois : %+v", b)
	}
	lot, err := c.EnvoyerBlocPurgatoire(RequeteBloc{Chemin: []Filtre{{Axe: "libelle", Exclure: b.Exclure}}, Nombre: b.Nombre, Taille: b.Taille})
	if err != nil || lot.Ecart || lot.Nombre != 4 || lot.Taille != 675 {
		t.Fatalf("envoi d'Autres avec les chiffres affiches : %+v %v", lot, err)
	}
	n, err = c.Carte(RequeteCarte{Axes: []string{"libelle"}})
	if err != nil || n.Nombre != 1 || n.Taille != 500 || len(n.Noeuds) != 1 || n.Noeuds[0].Cle != "Factures" {
		t.Fatalf("les messages envoyes disparaissent de tous les libelles : %+v %v", n, err)
	}
	lot, err = c.EnvoyerBlocPurgatoire(RequeteBloc{Chemin: []Filtre{{Axe: "libelle", Valeur: "Factures"}}, Nombre: 1, Taille: 500})
	if err != nil || lot.Ecart || lot.Nombre != 1 {
		t.Fatalf("envoi du libelle restant : %+v %v", lot, err)
	}
	e, err := c.EtatDuPurgatoire()
	if err != nil || e.Nombre != 5 || e.Taille != 1175 {
		t.Fatalf("Purgatoire sans doublons : %+v %v", e, err)
	}
}

func TestLibellesPetitsBlocs(t *testing.T) {
	c := ouvrirTest(t)
	var msgs []imap.Message
	for i := 0; i < 12; i++ {
		msgs = append(msgs, imap.Message{UID: uint32(i + 1), GmMsgID: uint64(i + 1), Taille: 1000, Libelles: []string{fmt.Sprintf("Grand %02d", i)}})
	}
	msgs = append(msgs, imap.Message{UID: 13, GmMsgID: 13, Taille: 1, Libelles: []string{"Grand 00", "Petit A", "Petit B"}})
	if err := c.Enregistrer(msgs); err != nil {
		t.Fatal(err)
	}
	for _, metrique := range []string{"nombre", "taille"} {
		// Une petite surface garde 12 blocs et reunit les deux derniers.
		n, err := c.Carte(RequeteCarte{Axes: []string{"libelle"}, Metrique: metrique, PartMin: 0.1})
		if err != nil || len(n.Noeuds) != 13 {
			t.Fatalf("petite surface (%s) : %+v %v", metrique, n, err)
		}
		b := n.Noeuds[12]
		if b.Autres != 2 || b.Nombre != 1 || b.Taille != 1 || len(b.Exclure) != 12 {
			t.Fatalf("petits libelles dedoublonnes (%s) : %+v", metrique, b)
		}
		n, err = c.Carte(RequeteCarte{Axes: []string{"libelle"}, Metrique: metrique})
		if err != nil || len(n.Noeuds) != 14 {
			t.Fatalf("sans seuil (%s) : %+v %v", metrique, n, err)
		}
	}
}
