package imap

import (
	"encoding/base64"
	"strings"
	"unicode/utf16"
)

// UTF-7 modifie (RFC 3501, 5.1.3) : encodage des noms de dossiers quand
// UTF8=ACCEPT n'est pas active. "&" ouvre une sequence base64 (alphabet avec
// "," a la place de "/") d'UTF-16 BE, "-" la ferme ; "&-" vaut "&".

var b64UTF7 = base64.NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+,").WithPadding(base64.NoPadding)

func decoderUTF7Modifie(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '&' {
			b.WriteByte(s[i])
			continue
		}
		fin := strings.IndexByte(s[i+1:], '-')
		if fin < 0 {
			return s // mal forme : rendu tel quel
		}
		code := s[i+1 : i+1+fin]
		i += fin + 1
		if code == "" {
			b.WriteByte('&')
			continue
		}
		octets, err := b64UTF7.DecodeString(code)
		if err != nil || len(octets)%2 != 0 {
			return s
		}
		u := make([]uint16, len(octets)/2)
		for j := range u {
			u[j] = uint16(octets[2*j])<<8 | uint16(octets[2*j+1])
		}
		b.WriteString(string(utf16.Decode(u)))
	}
	return b.String()
}

func encoderUTF7Modifie(s string) string {
	var b strings.Builder
	var tampon []rune
	vider := func() {
		if len(tampon) == 0 {
			return
		}
		u := utf16.Encode(tampon)
		octets := make([]byte, 2*len(u))
		for j, v := range u {
			octets[2*j], octets[2*j+1] = byte(v>>8), byte(v)
		}
		b.WriteByte('&')
		b.WriteString(b64UTF7.EncodeToString(octets))
		b.WriteByte('-')
		tampon = tampon[:0]
	}
	for _, r := range s {
		switch {
		case r == '&':
			vider()
			b.WriteString("&-")
		case r >= 0x20 && r <= 0x7e:
			vider()
			b.WriteRune(r)
		default:
			tampon = append(tampon, r)
		}
	}
	vider()
	return b.String()
}
