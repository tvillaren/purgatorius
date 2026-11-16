package synchro

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/louis49/purgatorius/internal/cache"
	"github.com/louis49/purgatorius/internal/imap"
	"github.com/louis49/purgatorius/internal/imap/imaptest"
)

// corbeille : met le Purgatoire a la corbeille contre la fausse boite.
func corbeille(t *testing.T, c *cache.Cache, b *imaptest.Boite, attendu int) (BilanCorbeille, error) {
	t.Helper()
	opt := Options{Adresse: "moi@example.invalid", MotDePasse: "x",
		Connecter: func(context.Context) (*imap.Client, error) { return b.Connecter(t), nil }}
	return MettreALaCorbeille(context.Background(), c, opt, attendu, nil)
}

// synchronise : boite de test synchronisee, messages uids envoyes au Purgatoire.
func synchronise(t *testing.T, uids ...uint32) (*imaptest.Boite, *cache.Cache) {
	t.Helper()
	b, c := boiteTest(), cacheTest(t)
	if _, err := lancer(t, c, b, Options{}); err != nil {
		t.Fatal(err)
	}
	if lot, err := c.EnvoyerPurgatoire(uids); err != nil || lot.Nombre != len(uids) {
		t.Fatalf("envoi : %+v %v", lot, err)
	}
	return b, c
}

func uidsDe(ms []imaptest.Message) []uint32 {
	var out []uint32
	for _, m := range ms {
		out = append(out, m.UID)
	}
	slices.Sort(out)
	return out
}

func deplacements(b *imaptest.Boite) []string {
	var out []string
	for _, cmd := range b.Commandes() {
		if strings.HasPrefix(cmd, "UID MOVE") {
			out = append(out, cmd)
		}
	}
	return out
}

func etatPurgatoire(t *testing.T, c *cache.Cache) cache.EtatPurgatoire {
	t.Helper()
	e, err := c.EtatDuPurgatoire()
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// Le Purgatoire, et lui seul, part a la corbeille ; il sort du cache et du
// Purgatoire ; une synchro suivante ne le fait pas revenir.
func TestCorbeilleDeplaceLePurgatoire(t *testing.T) {
	b, c := synchronise(t, 2, 5)
	bilan, err := corbeille(t, c, b, 2)
	if err != nil || bilan.Deplaces != 2 || bilan.Taille != 7000 || bilan.Absents != 0 {
		t.Fatalf("bilan : %+v %v", bilan, err)
	}
	if got := uidsDe(b.Corbeille); !slices.Equal(got, []uint32{2, 5}) {
		t.Fatalf("dans la corbeille : %v", got)
	}
	if d := deplacements(b); len(d) != 1 || d[0] != `UID MOVE 2,5 "`+imaptest.NomCorbeille+`"` {
		t.Fatalf("deplacements : %q", d)
	}
	if e := etatPurgatoire(t, c); e.Nombre != 0 || e.TotalNombre != 3 {
		t.Fatalf("apres : %+v", e)
	}
	b.OublierCommandes() // la synchro, elle, doit rester en lecture seule
	if bilan, err := lancer(t, c, b, Options{}); err != nil || bilan.Total != 3 {
		t.Fatalf("synchro suivante : %+v %v", bilan, err)
	}
	if n := nombre(t, c); n != 3 {
		t.Fatalf("messages en cache : %d", n)
	}
}

// L'UID du cache n'est qu'un candidat : s'il designe un autre message (ici
// renumerote sans changement d'UIDVALIDITY), ce message n'est pas touche, et
// le bon est retrouve par son X-GM-MSGID.
func TestCorbeilleVerifieChaqueUID(t *testing.T) {
	b, c := synchronise(t, 2)
	b.Messages[1].UID = 20 // le message 0x102
	b.Messages = append(b.Messages, imaptest.Message{UID: 2, GmMsgID: 0x999, Taille: 1, De: "autre@x.example", ModSeq: 20})
	bilan, err := corbeille(t, c, b, 1)
	if err != nil || bilan.Deplaces != 1 {
		t.Fatalf("bilan : %+v %v", bilan, err)
	}
	if len(b.Corbeille) != 1 || b.Corbeille[0].GmMsgID != 0x102 || b.Corbeille[0].UID != 20 {
		t.Fatalf("dans la corbeille : %+v", b.Corbeille)
	}
}

// Un message deja parti de « Tous les messages » sort du Purgatoire sans
// commande ; les autres partent.
func TestCorbeilleMessageDejaParti(t *testing.T) {
	b, c := synchronise(t, 2, 5)
	b.Messages = slices.DeleteFunc(b.Messages, func(m imaptest.Message) bool { return m.UID == 5 })
	bilan, err := corbeille(t, c, b, 2)
	if err != nil || bilan.Deplaces != 1 || bilan.Absents != 1 {
		t.Fatalf("bilan : %+v %v", bilan, err)
	}
	if e := etatPurgatoire(t, c); e.Nombre != 0 {
		t.Fatalf("Purgatoire : %+v", e)
	}
}

// Numerotation changee depuis la synchro : refus, rien n'est deplace.
func TestCorbeilleRefuseUneAutreNumerotation(t *testing.T) {
	b, c := synchronise(t, 2)
	b.UIDValidity = 99
	if _, err := corbeille(t, c, b, 1); !errors.Is(err, ErrNumerotation) {
		t.Fatalf("erreur : %v", err)
	}
	if len(deplacements(b)) != 0 || len(b.Corbeille) != 0 {
		t.Fatal("messages deplaces malgre la numerotation changee")
	}
	if e := etatPurgatoire(t, c); e.Nombre != 1 {
		t.Fatalf("Purgatoire touche : %+v", e)
	}
}

// Le Purgatoire ne correspond plus a l'affichage : refus, sans connexion.
func TestCorbeilleRefuseUnEcart(t *testing.T) {
	b, c := synchronise(t, 2, 5)
	avant := len(b.Commandes())
	if _, err := corbeille(t, c, b, 3); !errors.Is(err, ErrEcartCorbeille) {
		t.Fatalf("erreur : %v", err)
	}
	if len(b.Commandes()) != avant {
		t.Fatal("connexion ouverte malgre l'ecart")
	}
}

// Coupure au deuxieme lot : le premier est sorti du cache et du Purgatoire,
// le second y reste ; une relance le termine.
func TestCorbeilleInterrompueReprend(t *testing.T) {
	avant := lotCorbeille
	lotCorbeille = 1
	defer func() { lotCorbeille = avant }()
	b, c := synchronise(t, 2, 5)
	b.CoupureMoveApres = 2
	if _, err := corbeille(t, c, b, 2); err == nil {
		t.Fatal("coupure non signalee")
	}
	if got := uidsDe(b.Corbeille); !slices.Equal(got, []uint32{2}) {
		t.Fatalf("dans la corbeille : %v", got)
	}
	if e := etatPurgatoire(t, c); e.Nombre != 1 || e.TotalNombre != 4 {
		t.Fatalf("apres coupure : %+v", e)
	}
	b.CoupureMoveApres = 0
	if bilan, err := corbeille(t, c, b, 1); err != nil || bilan.Deplaces != 1 {
		t.Fatalf("relance : %+v %v", bilan, err)
	}
	if got := uidsDe(b.Corbeille); !slices.Equal(got, []uint32{2, 5}) {
		t.Fatalf("dans la corbeille : %v", got)
	}
}
