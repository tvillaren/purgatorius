package imap

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/ianaindex"
)

// lecteurCharset convertit un texte d'un jeu de caracteres quelconque vers
// UTF-8 pour mime.WordDecoder. Une vraie boite contient des objets hors de la
// bibliotheque standard (windows-1252, iso-8859-15, gb2312...) et des alias
// fantaisistes (utf8, iso8859-15, cp1252) : on essaie la table WHATWG
// (tolerante, alias nombreux), puis la table IANA.
func lecteurCharset(charset string, r io.Reader) (io.Reader, error) {
	enc, err := trouverEncodage(charset)
	if err != nil {
		return nil, err
	}
	if enc == nil { // deja UTF-8
		return r, nil
	}
	return enc.NewDecoder().Reader(r), nil
}

func trouverEncodage(charset string) (encoding.Encoding, error) {
	nom := strings.ToLower(strings.TrimSpace(charset))
	switch nom {
	case "utf-8", "utf8", "unicode-1-1-utf-8", "us-ascii", "ascii", "ansi_x3.4-1968":
		return nil, nil
	}
	if e, err := htmlindex.Get(nom); err == nil {
		if n, _ := htmlindex.Name(e); n == "utf-8" {
			return nil, nil
		}
		return e, nil
	}
	if e, err := ianaindex.MIME.Encoding(nom); err == nil && e != nil {
		return e, nil
	}
	if alt := strings.Replace(nom, "iso8859", "iso-8859", 1); alt != nom {
		return trouverEncodage(alt)
	}
	return nil, fmt.Errorf("jeu de caracteres inconnu : %q", charset)
}
