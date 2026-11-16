package synchro

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"testing"

	"github.com/louis49/purgatorius/internal/imap"
)

func TestPlanifier(t *testing.T) {
	aLire, aSupprimer := Planifier([]uint32{1, 2, 5, 9, 10}, map[uint32]bool{2: true, 3: true, 9: true})
	if !slices.Equal(aLire, []uint32{10, 5, 1}) {
		t.Fatalf("a lire (du plus recent au plus ancien) : %v", aLire)
	}
	if !slices.Equal(aSupprimer, []uint32{3}) {
		t.Fatalf("a supprimer : %v", aSupprimer)
	}
	if l, s := Planifier(nil, nil); l != nil || s != nil {
		t.Fatalf("boite vide : %v %v", l, s)
	}
}

func TestCodeErreur(t *testing.T) {
	cas := map[error]string{
		nil:              "",
		context.Canceled: ErreurAnnulee,
		fmt.Errorf("x : %w", imap.ErrAuthentification):      ErreurAuthentification,
		ErrTousLesMessagesAbsent:                            ErreurTousAbsent,
		&net.OpError{Op: "dial", Err: errors.New("refuse")}: ErreurReseau,
		errors.New("autre chose"):                           ErreurAutre,
	}
	for err, code := range cas {
		if got := CodeErreur(err); got != code {
			t.Errorf("CodeErreur(%v) = %q, attendu %q", err, got, code)
		}
	}
}

func TestDuLot(t *testing.T) {
	msgs := []imap.Message{
		{UID: 5, GmMsgID: 105},
		{UID: 8, GmMsgID: 108}, // hors lot
		{UID: 6, GmMsgID: 0},   // identifiant Gmail nul
		{UID: 5, GmMsgID: 105}, // doublon
		{UID: 7, GmMsgID: 107},
	}
	got := DuLot([]uint32{5, 6, 7}, msgs)
	if len(got) != 2 || got[0].UID != 5 || got[1].UID != 7 {
		t.Fatalf("DuLot : %+v", got)
	}
}
