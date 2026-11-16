package imaptest

import (
	"fmt"
	"testing"
)

// espion : un testing.TB qui note les erreurs au lieu d'echouer.
type espion struct {
	testing.TB
	erreurs []string
}

func (e *espion) Helper() {}
func (e *espion) Errorf(format string, args ...any) {
	e.erreurs = append(e.erreurs, fmt.Sprintf(format, args...))
}

// Tout FETCH qui marque le message lu est repere ; les
// elements sans effet (PEEK, RFC822.SIZE, RFC822.HEADER) ne le sont pas.
func TestVerifierLectureSeule(t *testing.T) {
	interdites := []string{
		"UID FETCH 1 (BODY[])",
		"UID FETCH 1 (BODY[TEXT])",
		"UID FETCH 1 (BINARY[1])",
		"UID FETCH 1 (RFC822)",
		"UID FETCH 1 RFC822",
		"UID FETCH 1 (UID RFC822.TEXT)",
		"UID FETCH 1 (rfc822)",
		"SELECT INBOX",
		"UID STORE 1 +FLAGS (\\Seen)",
	}
	admises := []string{
		"UID FETCH 1:* (UID RFC822.SIZE INTERNALDATE FLAGS BODY.PEEK[HEADER.FIELDS (FROM SUBJECT)])",
		"UID FETCH 1 (RFC822.HEADER BINARY.PEEK[1] BINARY.SIZE[1])",
		"EXAMINE \"[Gmail]/Tous les messages\"",
	}
	for _, cmd := range interdites {
		e := &espion{}
		(&Boite{commandes: []string{cmd}}).VerifierLectureSeule(e)
		if len(e.erreurs) == 0 {
			t.Errorf("non repere : %q", cmd)
		}
	}
	for _, cmd := range admises {
		e := &espion{}
		(&Boite{commandes: []string{cmd}}).VerifierLectureSeule(e)
		if len(e.erreurs) != 0 {
			t.Errorf("repere a tort : %q : %v", cmd, e.erreurs)
		}
	}
}
