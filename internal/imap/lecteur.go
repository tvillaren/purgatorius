package imap

// Lecture des reponses IMAP (RFC 3501, section 9) : listes, chaines entre
// guillemets, litteraux {n}, NIL et atomes. Un atome de section de message
// peut contenir des crochets avec espaces et parentheses, ex.
// BODY[HEADER.FIELDS (FROM)].

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// noeud : valeur d'une reponse IMAP.
type noeud struct {
	genre byte // 'a' atome, 's' chaine, 'l' liste, 'n' NIL
	s     string
	l     []noeud
}

// compteur : octets recus du serveur (mesure de debit, progression).
type compteur struct {
	r io.Reader
	n int64
}

func (c *compteur) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// tailleMaxLitteral borne un litteral annonce par le serveur : une valeur
// aberrante ne doit pas provoquer une allocation geante. Les en-tetes demandes
// font quelques Ko ; 64 Mo laisse une marge large.
const tailleMaxLitteral = 64 << 20

type lecteur struct{ r *bufio.Reader }

func (l *lecteur) peek() (byte, error) {
	b, err := l.r.Peek(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (l *lecteur) espaces() error {
	for {
		c, err := l.peek()
		if err != nil {
			return err
		}
		if c != ' ' {
			return nil
		}
		l.r.ReadByte()
	}
}

// Bornes de l'analyseur : une reponse pathologique (serveur defaillant) ne doit
// ni epuiser la pile (imbrication), ni la memoire (jeton sans fin). Les vrais
// messages en sont loin : quelques niveaux de BODYSTRUCTURE (un message
// transfere en piece jointe en ajoute deux ou trois ; mille niveaux laissent
// passer des centaines de transferts emboites, sans risque pour la pile), des
// jetons de quelques Ko ; les gros contenus passent par les litteraux {n},
// dont la taille s'ecrit en peu de chiffres.
const (
	profondeurMax       = 1000
	tailleMaxJeton      = 1 << 20
	chiffresMaxLitteral = 20
)

// sections : atomes dont le crochet ouvre une section de message, qui peut
// contenir espaces et parentheses (BODY[HEADER.FIELDS (FROM SUBJECT)]).
// Ailleurs, « [ » est un caractere ordinaire (libelle « [projet »).
var sections = map[string]bool{"BODY": true, "BODY.PEEK": true, "BINARY": true, "BINARY.PEEK": true, "BINARY.SIZE": true}

func (l *lecteur) valeur() (noeud, error) { return l.valeurA(0) }

func (l *lecteur) valeurA(prof int) (noeud, error) {
	if prof > profondeurMax {
		return noeud{}, fmt.Errorf("imbrication de plus de %d niveaux", profondeurMax)
	}
	if err := l.espaces(); err != nil {
		return noeud{}, err
	}
	c, err := l.peek()
	if err != nil {
		return noeud{}, err
	}
	switch c {
	case '(':
		l.r.ReadByte()
		var liste []noeud
		for {
			if err := l.espaces(); err != nil {
				return noeud{}, err
			}
			c, err := l.peek()
			if err != nil {
				return noeud{}, err
			}
			if c == ')' {
				l.r.ReadByte()
				return noeud{genre: 'l', l: liste}, nil
			}
			if c == '\r' || c == '\n' {
				return noeud{}, fmt.Errorf("liste non fermee en fin de ligne")
			}
			v, err := l.valeurA(prof + 1)
			if err != nil {
				return noeud{}, err
			}
			liste = append(liste, v)
		}
	case '"':
		l.r.ReadByte()
		var b []byte
		for {
			c, err := l.r.ReadByte()
			if err != nil {
				return noeud{}, err
			}
			if c == '\\' {
				if c, err = l.r.ReadByte(); err != nil {
					return noeud{}, err
				}
			} else if c == '"' {
				return noeud{genre: 's', s: string(b)}, nil
			}
			if len(b) >= tailleMaxJeton {
				return noeud{}, fmt.Errorf("chaine de plus de %d octets", tailleMaxJeton)
			}
			b = append(b, c)
		}
	case '{':
		l.r.ReadByte()
		var t []byte
		for {
			c, err := l.r.ReadByte()
			if err != nil {
				return noeud{}, err
			}
			if c == '}' {
				break
			}
			if len(t) >= chiffresMaxLitteral {
				return noeud{}, fmt.Errorf("litteral invalide")
			}
			t = append(t, c)
		}
		n, err := strconv.Atoi(strings.TrimSuffix(string(t), "+"))
		if err != nil || n < 0 || n > tailleMaxLitteral {
			return noeud{}, fmt.Errorf("litteral invalide")
		}
		if _, err := l.r.Discard(2); err != nil { // CRLF
			return noeud{}, err
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(l.r, buf); err != nil {
			return noeud{}, err
		}
		return noeud{genre: 's', s: string(buf)}, nil
	}
	var b []byte
	for {
		c, err := l.peek()
		if err != nil {
			return noeud{}, err
		}
		if c == ' ' || c == '(' || c == ')' || c == '\r' || c == '\n' {
			break
		}
		if len(b) >= tailleMaxJeton {
			return noeud{}, fmt.Errorf("atome de plus de %d octets", tailleMaxJeton)
		}
		l.r.ReadByte()
		if c == '[' && sections[strings.ToUpper(string(b))] {
			b = append(b, c)
			for {
				c, err := l.r.ReadByte()
				if err != nil {
					return noeud{}, err
				}
				if c == '\r' || c == '\n' {
					return noeud{}, fmt.Errorf("section non fermee en fin de ligne")
				}
				if len(b) >= tailleMaxJeton {
					return noeud{}, fmt.Errorf("section de plus de %d octets", tailleMaxJeton)
				}
				b = append(b, c)
				if c == ']' {
					break
				}
			}
			continue
		}
		b = append(b, c)
	}
	if len(b) == 0 {
		return noeud{}, fmt.Errorf("jeton vide (caractere %q)", c)
	}
	if string(b) == "NIL" {
		return noeud{genre: 'n'}, nil
	}
	return noeud{genre: 'a', s: string(b)}, nil
}

// ligne lit jusqu'a la fin de ligne (incluse), au plus tailleMaxJeton octets.
func (l *lecteur) ligne() (string, error) {
	var b []byte
	for {
		c, err := l.r.ReadByte()
		if err != nil {
			return "", err
		}
		b = append(b, c)
		if c == '\n' {
			return string(b), nil
		}
		if len(b) >= tailleMaxJeton {
			return "", fmt.Errorf("ligne de plus de %d octets", tailleMaxJeton)
		}
	}
}

func estStatut(s string) bool {
	switch strings.ToUpper(s) {
	case "OK", "NO", "BAD", "BYE", "PREAUTH":
		return true
	}
	return false
}

// reponse lit une reponse complete (etiquette, elements). Pour un statut
// (OK/NO/BAD/BYE), le reste de la ligne est rendu brut : c'est du texte libre,
// pas forcement bien parenthese.
func (l *lecteur) reponse() (string, []noeud, error) {
	tag, err := l.valeur()
	if err != nil {
		return "", nil, err
	}
	var elems []noeud
	for {
		if err := l.espaces(); err != nil {
			return "", nil, err
		}
		c, err := l.peek()
		if err != nil {
			return "", nil, err
		}
		if c == '\r' || c == '\n' {
			if _, err := l.ligne(); err != nil {
				return "", nil, err
			}
			return tag.s, elems, nil
		}
		v, err := l.valeur()
		if err != nil {
			return "", nil, err
		}
		elems = append(elems, v)
		if len(elems) == 1 && v.genre == 'a' && estStatut(v.s) {
			reste, err := l.ligne()
			if err != nil {
				return "", nil, err
			}
			elems = append(elems, noeud{genre: 's', s: strings.TrimSpace(reste)})
			return tag.s, elems, nil
		}
	}
}
