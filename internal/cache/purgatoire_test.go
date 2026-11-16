package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/louis49/purgatorius/internal/imap"
)

func TestPurgatoire(t *testing.T) {
	c := remplirListe(t)
	e, err := c.EtatDuPurgatoire()
	if err != nil || e.Nombre != 0 || e.TotalNombre != 4 || e.TotalTaille != 1550 {
		t.Fatalf("etat initial : %+v %v", e, err)
	}
	lot, err := c.EnvoyerPurgatoire([]uint32{2, 99})
	if err != nil || lot.Nombre != 1 || lot.Taille != 900 || lot.UIDs[0] != 2 {
		t.Fatalf("envoi : %+v %v", lot, err)
	}
	// Masque de la carte, du detail et de la liste.
	n, _ := c.Carte(RequeteCarte{Axes: []string{"domaine"}, Metrique: "taille"})
	if n.Nombre != 3 || n.Taille != 650 {
		t.Fatalf("carte sans le Purgatoire : %+v", n)
	}
	d, _ := c.DetailCarte(RequeteDetail{Metrique: "taille"})
	if d.Nombre != 3 {
		t.Fatalf("detail sans le Purgatoire : %+v", d)
	}
	idx, _ := c.IndexListe(RequeteListe{Tri: "date"})
	if len(idx.UIDs) != 3 {
		t.Fatalf("liste sans le Purgatoire : %+v", idx)
	}
	// Bloc : le domaine x.example (deja en partie au Purgatoire), lus seulement.
	lot, err = c.EnvoyerBlocPurgatoire(RequeteBloc{Chemin: []Filtre{{Axe: "domaine", Valeur: "x.example"}}, Lecture: "lu"})
	if err != nil || lot.Nombre != 2 {
		t.Fatalf("envoi d'un bloc : %+v %v", lot, err)
	}
	e, _ = c.EtatDuPurgatoire()
	if e.Nombre != 3 || e.Taille != 1500 || e.PJ != 200 || e.NonLus != 1 {
		t.Fatalf("etat apres envoi : %+v", e)
	}
	// Revue : tout le Purgatoire, sans les filtres globaux.
	idx, err = c.IndexListe(RequeteListe{Purgatoire: true, Tri: "taille", Desc: true})
	if err != nil || len(idx.UIDs) != 3 || idx.UIDs[0] != 2 {
		t.Fatalf("revue : %+v %v", idx, err)
	}
	if idx, _ = c.IndexListe(RequeteListe{Purgatoire: true, Boite: "envoye", Lecture: "nonlu", Tri: "date"}); len(idx.UIDs) != 3 {
		t.Fatalf("revue filtree : %+v", idx)
	}
	if _, err := c.IndexListe(RequeteListe{Purgatoire: true, Chemin: []Filtre{{Axe: "domaine", Valeur: "x.example"}}, Tri: "date"}); err == nil {
		t.Fatal("revue avec un chemin acceptee")
	}
	// Annulation du dernier lot : le lot retire ne compte que ce qui y etait.
	retire, err := c.RetirerPurgatoire(append(lot.UIDs, 4))
	if err != nil || retire.Nombre != 2 || retire.Taille != 600 {
		t.Fatalf("retrait : %+v %v", retire, err)
	}
	if e, _ = c.EtatDuPurgatoire(); e.Nombre != 1 {
		t.Fatalf("etat apres annulation : %+v", e)
	}
	// Tout retirer.
	vide, err := c.ViderPurgatoire()
	if err != nil || vide.Nombre != 1 || vide.UIDs[0] != 2 {
		t.Fatalf("vidage : %+v %v", vide, err)
	}
	if e, _ = c.EtatDuPurgatoire(); e.Nombre != 0 || e.Taille != 0 {
		t.Fatalf("etat apres vidage : %+v", e)
	}
	if _, err := c.EnvoyerBlocPurgatoire(RequeteBloc{Lecture: "peut-etre"}); err == nil {
		t.Fatal("requete de bloc invalide acceptee")
	}
}

func TestSchemaAnterieurRecree(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cache.db")
	c, err := OuvrirChemin(p)
	if err != nil || c.Reinitialise {
		t.Fatalf("creation : %v %v", c, err)
	}
	c.EcrireMeta("empreinte_schema", "ancienne")
	c.Enregistrer(nil)
	c.Fermer()
	c, err = OuvrirChemin(p)
	if err != nil || !c.Reinitialise {
		t.Fatalf("schema anterieur non recree : %v", err)
	}
	c.Fermer()
	c, err = OuvrirChemin(p)
	if err != nil || c.Reinitialise {
		t.Fatalf("base recreee rouverte comme anterieure : %v", err)
	}
	c.Fermer()
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}

// Le bloc a change depuis son affichage (synchro en arriere-plan) :
// rien n'est envoye, l'ecart est signale. Avec les chiffres a jour, il part.
func TestBlocEcartRefuse(t *testing.T) {
	c := ouvrirTest(t)
	c.Enregistrer([]imap.Message{unMessage(1, 0x10), unMessage(2, 0x20)})
	n, err := c.Carte(RequeteCarte{Axes: []string{"domaine"}, Metrique: "nombre"})
	if err != nil || len(n.Noeuds) != 1 {
		t.Fatalf("carte : %+v %v", n, err)
	}
	affiche := n.Noeuds[0]
	demande := RequeteBloc{Chemin: []Filtre{{Axe: "domaine", Valeur: affiche.Cle}}, Nombre: affiche.Nombre, Taille: affiche.Taille}
	c.Enregistrer([]imap.Message{unMessage(3, 0x30)}) // arrive pendant que la carte est affichee
	lot, err := c.EnvoyerBlocPurgatoire(demande)
	if err != nil || !lot.Ecart || lot.Nombre != 0 {
		t.Fatalf("bloc change envoye : %+v %v", lot, err)
	}
	if e, _ := c.EtatDuPurgatoire(); e.Nombre != 0 {
		t.Fatalf("Purgatoire apres refus : %+v", e)
	}
	n, _ = c.Carte(RequeteCarte{Axes: []string{"domaine"}, Metrique: "nombre"})
	demande.Nombre, demande.Taille = n.Noeuds[0].Nombre, n.Noeuds[0].Taille
	if lot, err = c.EnvoyerBlocPurgatoire(demande); err != nil || lot.Ecart || lot.Nombre != 3 {
		t.Fatalf("bloc a jour : %+v %v", lot, err)
	}
}

// Sur une carte qui n'a pas change, chaque bloc de chaque axe (bloc
// « Autres » compris) part avec les chiffres affiches : aucun refus a tort.
func TestBlocsAffichesSansEcart(t *testing.T) {
	c := remplir(t)
	avant := maxBlocs
	maxBlocs = 3 // force un bloc « Autres » renvoye par le serveur
	defer func() { maxBlocs = avant }()
	for axe := range axes {
		for _, lecture := range []string{"", "nonlu"} {
			n, err := c.Carte(RequeteCarte{Axes: []string{axe}, Metrique: "taille", Lecture: lecture})
			if err != nil {
				t.Fatal(err)
			}
			// Comme le front (axes.js, regrouper) : le bloc « Autres » exclut
			// les valeurs affichees a cote de lui.
			var affichees []string
			for _, b := range n.Noeuds {
				if b.Autres == 0 {
					affichees = append(affichees, b.Cle)
				}
			}
			for _, b := range n.Noeuds {
				f := Filtre{Axe: axe, Valeur: b.Cle}
				if b.Autres > 0 {
					f = Filtre{Axe: axe, Exclure: affichees}
				}
				lot, err := c.EnvoyerBlocPurgatoire(RequeteBloc{Chemin: []Filtre{f}, Lecture: lecture, Nombre: b.Nombre, Taille: b.Taille})
				if err != nil || lot.Ecart || lot.Nombre != b.Nombre {
					t.Fatalf("%s %q (%s) : %+v %v, affiche %d", axe, b.Cle, lecture, lot, err, b.Nombre)
				}
				if _, err := c.RetirerPurgatoire(lot.UIDs); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

// Envoi du niveau affiche (on est entre dans un bloc, rien n'est choisi) :
// avec les totaux du niveau, aucun refus a tort.
func TestNiveauAfficheSansEcart(t *testing.T) {
	c := remplir(t)
	racine, err := c.Carte(RequeteCarte{Axes: []string{"domaine", "expediteur"}, Metrique: "taille"})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range racine.Noeuds {
		chemin := []Filtre{{Axe: "domaine", Valeur: b.Cle}}
		n, err := c.Carte(RequeteCarte{Axes: []string{"domaine", "expediteur"}, Chemin: chemin, Metrique: "taille"})
		if err != nil {
			t.Fatal(err)
		}
		lot, err := c.EnvoyerBlocPurgatoire(RequeteBloc{Chemin: chemin, Nombre: n.Nombre, Taille: n.Taille})
		if err != nil || lot.Ecart || lot.Nombre != n.Nombre {
			t.Fatalf("niveau %q : %+v %v, affiche %d", b.Cle, lot, err, n.Nombre)
		}
		if _, err := c.RetirerPurgatoire(lot.UIDs); err != nil {
			t.Fatal(err)
		}
	}
}
