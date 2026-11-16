package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/louis49/purgatorius/internal/imap"
)

// remplir : 3 gros domaines + 30 petits, sur deux annees.
func remplir(t *testing.T) *Cache {
	t.Helper()
	c := ouvrirTest(t)
	var msgs []imap.Message
	uid := uint32(0)
	ajout := func(de string, taille int64, annee int, pieces ...imap.Piece) {
		uid++
		libelles := []string{`\Inbox`}
		dest := []string{"moi@example.invalid"}
		if de == "b@gros.example" { // messages envoyes
			libelles = []string{`\Sent`}
			dest = []string{"client@ailleurs.example", "copie@ailleurs.example"}
		}
		msgs = append(msgs, imap.Message{UID: uid, GmMsgID: uint64(1000 + uid), De: de, Taille: taille, Objet: "Objet " + de,
			Date: time.Date(annee, 6, 1, 0, 0, 0, 0, time.UTC), Pieces: pieces, Libelles: libelles, Destinataires: dest})
	}
	for i := 0; i < 10; i++ {
		ajout("a@gros.example", 1_000_000, 2020, imap.Piece{Type: "pdf", Taille: 500_000})
		ajout("b@gros.example", 500_000, 2021)
		ajout("c@moyen.example", 300_000, 2021, imap.Piece{Type: "image", Taille: 100, Inline: true})
		ajout("d@autre.example", 200_000, 2020)
	}
	for i := 0; i < 30; i++ {
		ajout(fmt.Sprintf("x@petit%02d.example", i), 1_000, 2021)
	}
	if err := c.Enregistrer(msgs); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCarteDomaines(t *testing.T) {
	c := remplir(t)
	n, err := c.Carte(RequeteCarte{Axes: []string{"domaine", "expediteur"}, Metrique: "taille"})
	if err != nil {
		t.Fatal(err)
	}
	if n.Axe != "domaine" || n.Nombre != 70 || n.Taille != 20_030_000 {
		t.Fatalf("totaux : %+v", n)
	}
	if n.Noeuds[0].Cle != "gros.example" || n.Noeuds[0].Taille != 15_000_000 || n.Noeuds[0].Domaine != "gros.example" {
		t.Fatalf("premier bloc : %+v", n.Noeuds[0])
	}
	if len(n.Noeuds) != 33 || n.Noeuds[len(n.Noeuds)-1].Autres != 0 {
		t.Fatalf("33 domaines attendus sans bloc Autres : %d", len(n.Noeuds))
	}
	// Au-dela de maxBlocs, le reste forme un bloc « Autres » qui couvre tout.
	ancien := maxBlocs
	maxBlocs = 5
	defer func() { maxBlocs = ancien }()
	n, _ = c.Carte(RequeteCarte{Axes: []string{"domaine", "expediteur"}, Metrique: "taille"})
	der := n.Noeuds[len(n.Noeuds)-1]
	if len(n.Noeuds) != 6 || der.Autres != 28 {
		t.Fatalf("bloc Autres attendu en 6e position (28 valeurs) : %d blocs, %+v", len(n.Noeuds), der)
	}
	somme := 0
	for _, x := range n.Noeuds {
		somme += x.Nombre
	}
	if somme != 70 {
		t.Fatalf("les blocs ne couvrent pas le niveau : %d", somme)
	}
	// Zoom dans « Autres » : le front exclut les valeurs affichees.
	var exclure []string
	for _, x := range n.Noeuds[:5] {
		exclure = append(exclure, x.Cle)
	}
	n2, err := c.Carte(RequeteCarte{Axes: []string{"domaine", "expediteur"}, Chemin: []Filtre{{Axe: "domaine", Exclure: exclure}}, Metrique: "taille"})
	if err != nil || n2.Nombre != der.Nombre {
		t.Fatalf("zoom dans Autres : %d messages, attendu %d (%v)", n2.Nombre, der.Nombre, err)
	}
}

func TestCarteZoomEtNiveauMessages(t *testing.T) {
	c := remplir(t)
	r := RequeteCarte{Axes: []string{"domaine", "expediteur"}, Chemin: []Filtre{{Axe: "domaine", Valeur: "gros.example"}}, Metrique: "nombre"}
	n, err := c.Carte(r)
	if err != nil || n.Axe != "expediteur" || len(n.Noeuds) != 2 || n.Noeuds[0].Domaine != "gros.example" {
		t.Fatalf("niveau expediteurs : %+v %v", n, err)
	}
	r.Chemin = append(r.Chemin, Filtre{Axe: "expediteur", Valeur: "a@gros.example"})
	n, err = c.Carte(r)
	if err != nil || n.Axe != "message" || len(n.Noeuds) != 0 || n.Nombre != 10 {
		t.Fatalf("niveau des messages : %+v %v", n, err)
	}
}

func TestCarteRequetesInvalides(t *testing.T) {
	c := remplir(t)
	for _, r := range []RequeteCarte{
		{},
		{Axes: []string{"domaine; DROP TABLE messages"}},
		{Axes: []string{"domaine"}, Chemin: []Filtre{{Axe: "expediteur", Valeur: "x"}}},
		{Axes: []string{"domaine"}, Chemin: []Filtre{{Axe: "domaine"}, {Axe: "domaine"}}},
	} {
		if _, err := c.Carte(r); err == nil {
			t.Errorf("requete invalide acceptee : %+v", r)
		}
	}
}

func TestDetailCarte(t *testing.T) {
	c := remplir(t)
	d, err := c.DetailCarte(RequeteDetail{Chemin: []Filtre{{Axe: "domaine", Valeur: "gros.example"}}, Metrique: "taille"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Nombre != 20 || d.Taille != 15_000_000 || len(d.ParAnnee) != 2 || d.ParAnnee[0].Cle != "2020" || d.Expediteurs[0].Cle != "a@gros.example" {
		t.Fatalf("detail : %+v", d)
	}
	if len(d.Pieces) != 1 || d.Pieces[0].Cle != "pdf" || d.Pieces[0].Nombre != 10 {
		t.Fatalf("pieces : %+v", d.Pieces)
	}
	// Les parties inline anonymes ne comptent pas comme pieces jointes.
	d, _ = c.DetailCarte(RequeteDetail{Chemin: []Filtre{{Axe: "domaine", Valeur: "moyen.example"}}, Metrique: "taille"})
	if len(d.Pieces) != 0 {
		t.Fatalf("inline compte comme piece jointe : %+v", d.Pieces)
	}
	d, err = c.DetailCarte(RequeteDetail{UID: 1, Metrique: "taille"})
	if err != nil || d.Message == nil || d.Message.Expediteur != "a@gros.example" || d.Message.GmMsgID != "3e9" {
		t.Fatalf("detail d'un message : %+v %v", d.Message, err)
	}
}

func TestFiltreBoiteEtAxes(t *testing.T) {
	c := remplir(t)
	n, err := c.Carte(RequeteCarte{Axes: []string{"destinataire"}, Metrique: "taille", Boite: "envoye"})
	if err != nil || n.Nombre != 10 || len(n.Noeuds) != 1 || n.Noeuds[0].Cle != "client@ailleurs.example" {
		t.Fatalf("envoyes par destinataire : %+v %v", n, err)
	}
	n, err = c.Carte(RequeteCarte{Axes: []string{"boite"}, Metrique: "nombre"})
	if err != nil || len(n.Noeuds) != 2 || n.Noeuds[0].Cle != "recu" || n.Noeuds[0].Nombre != 60 {
		t.Fatalf("axe boite : %+v %v", n, err)
	}
	if _, err := c.Carte(RequeteCarte{Axes: []string{"domaine"}, Boite: "corbeille"}); err == nil {
		t.Fatal("boite inconnue acceptee")
	}
	d, err := c.DetailCarte(RequeteDetail{UID: 2, Metrique: "taille", Boite: "envoye"})
	if err != nil || d.Message == nil || d.Message.Boite != "envoye" || len(d.Message.Destinataires) != 2 {
		t.Fatalf("detail d'un envoye : %+v %v", d.Message, err)
	}
}

func TestBoite(t *testing.T) {
	for att, l := range map[string][]string{
		"recu":      {`\Inbox`},
		"envoye":    {`\Sent`, `\Inbox`}, // envoye a soi-meme
		"brouillon": {`\Draft`, `\Sent`},
	} {
		if b := Boite(l); b != att {
			t.Errorf("Boite(%v) = %q, attendu %q", l, b, att)
		}
	}
}

// Annee et mois en heure locale, comme les dates affichees : un message du
// 31/12 a 23 h 30 UTC est du 1er janvier a Paris. Le resultat attendu suit le
// fuseau de la machine (sous UTC, le test ne discrimine pas).
func TestAnneeEtMoisEnHeureLocale(t *testing.T) {
	c := ouvrirTest(t)
	d := time.Date(2023, 12, 31, 23, 30, 0, 0, time.UTC)
	m := unMessage(1, 0x10)
	m.Date = d
	c.Enregistrer([]imap.Message{m})
	local := d.Local()
	for axe, att := range map[string]string{"annee": local.Format("2006"), "mois": local.Format("2006-01")} {
		n, err := c.Carte(RequeteCarte{Axes: []string{axe}, Metrique: "nombre"})
		if err != nil || len(n.Noeuds) != 1 || n.Noeuds[0].Cle != att {
			t.Fatalf("%s : %+v %v, attendu %s", axe, n.Noeuds, err, att)
		}
	}
	det, err := c.DetailCarte(RequeteDetail{Metrique: "nombre"})
	if err != nil || len(det.ParAnnee) != 1 || det.ParAnnee[0].Cle != local.Format("2006") {
		t.Fatalf("repartition par annee : %+v %v", det.ParAnnee, err)
	}
}
