package imap

import (
	"mime"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Message : metadonnees d'un message, telles que lues par FETCH.
type Message struct {
	UID           uint32
	Taille        int64 // RFC822.SIZE (octets)
	Date          time.Time
	Lu            bool
	GmMsgID       uint64
	GmThrID       uint64
	Libelles      []string // libelles Gmail ("\\Inbox", "\\Sent", libelles perso)
	De            string   // adresse de l'expediteur, en minuscules
	NomDe         string   // nom affiche de l'expediteur
	Objet         string   // objet decode
	Destinataires []string // adresses To puis Cc, en minuscules, sans doublon
	Pieces        []Piece

	ObjetIllisible bool // jeu de caracteres inconnu : objet laisse brut
}

// Piece : partie non textuelle d'un message (piece jointe ou partie inline).
type Piece struct {
	Type   string // pdf, image, video, audio, archive, bureautique, e-mail, invitation, autre
	Taille int64  // taille encodee (base64 : environ 4/3 de la taille reelle)
	Inline bool   // partie inline sans nom (logo, signature) : pas une piece jointe
}

// AvecPieceJointe : au moins une vraie piece jointe (hors inline anonymes).
func (m Message) AvecPieceJointe() bool {
	for _, p := range m.Pieces {
		if !p.Inline {
			return true
		}
	}
	return false
}

func analyserFetch(paires []noeud, utf8 bool) Message {
	var m Message
	var entetes string
	var structure noeud
	for i := 0; i+1 < len(paires); i += 2 {
		v := paires[i+1]
		switch k := strings.ToUpper(paires[i].s); {
		case k == "UID":
			u, _ := strconv.ParseUint(v.s, 10, 32)
			m.UID = uint32(u)
		case k == "RFC822.SIZE":
			m.Taille, _ = strconv.ParseInt(v.s, 10, 64)
		case k == "INTERNALDATE":
			m.Date, _ = time.Parse("_2-Jan-2006 15:04:05 -0700", v.s)
		case k == "X-GM-MSGID":
			m.GmMsgID, _ = strconv.ParseUint(v.s, 10, 64)
		case k == "X-GM-THRID":
			m.GmThrID, _ = strconv.ParseUint(v.s, 10, 64)
		case k == "FLAGS":
			for _, f := range v.l {
				if strings.EqualFold(f.s, `\Seen`) {
					m.Lu = true
				}
			}
		case k == "X-GM-LABELS":
			for _, l := range v.l {
				nom := l.s
				if !utf8 && !strings.HasPrefix(nom, `\`) {
					nom = decoderUTF7Modifie(nom)
				}
				m.Libelles = append(m.Libelles, nom)
			}
		case k == "BODYSTRUCTURE":
			structure = v
		case strings.HasPrefix(k, "BODY["):
			entetes = v.s
		}
	}
	m.De, m.NomDe, m.Objet, m.ObjetIllisible = analyserEntetes(entetes)
	m.Destinataires = analyserDestinataires(entetes)
	if structure.genre == 'l' {
		parties(structure, &m.Pieces)
	}
	return m
}

var reAdresse = regexp.MustCompile(`[A-Za-z0-9._%+\-=&'/]+@[A-Za-z0-9.\-]+`)

// analyserEntetes extrait l'expediteur et l'objet (decodage RFC 2047, jeux de
// caracteres de golang.org/x/text).
func analyserEntetes(brut string) (adresse, nom, objet string, illisible bool) {
	var de string
	if m, err := mail.ReadMessage(strings.NewReader(brut + "\r\n")); err == nil {
		de, objet = m.Header.Get("From"), m.Header.Get("Subject")
	}
	dec := &mime.WordDecoder{CharsetReader: lecteurCharset}
	if s, err := dec.DecodeHeader(objet); err == nil {
		objet = s
	} else {
		illisible = true
	}
	objet = strings.TrimSpace(objet)
	p := mail.AddressParser{WordDecoder: dec}
	if a, err := p.Parse(de); err == nil {
		adresse, nom = a.Address, a.Name
	} else if m := reAdresse.FindString(de); m != "" {
		adresse = m
	}
	return strings.ToLower(adresse), strings.TrimSpace(nom), objet, illisible
}

// analyserDestinataires extrait les adresses des en-tetes To puis Cc.
func analyserDestinataires(brut string) []string {
	m, err := mail.ReadMessage(strings.NewReader(brut + "\r\n"))
	if err != nil {
		return nil
	}
	p := mail.AddressParser{WordDecoder: &mime.WordDecoder{CharsetReader: lecteurCharset}}
	vus := map[string]bool{}
	var out []string
	for _, champ := range []string{"To", "Cc"} {
		v := m.Header.Get(champ)
		if v == "" {
			continue
		}
		var adresses []string
		if liste, err := p.ParseList(v); err == nil {
			for _, a := range liste {
				adresses = append(adresses, a.Address)
			}
		} else {
			adresses = reAdresse.FindAllString(v, -1)
		}
		for _, a := range adresses {
			a = strings.ToLower(a)
			if a != "" && !vus[a] {
				vus[a] = true
				out = append(out, a)
			}
		}
	}
	return out
}

func chaine(n noeud) string {
	if n.genre == 'n' {
		return ""
	}
	return strings.ToLower(n.s)
}

func param(liste noeud, cle string) string {
	for i := 0; i+1 < len(liste.l); i += 2 {
		if strings.EqualFold(liste.l[i].s, cle) {
			return liste.l[i+1].s
		}
	}
	return ""
}

// parties parcourt un BODYSTRUCTURE (RFC 3501, 7.4.2) et retient les parties
// non textuelles. Regle tiree des mesures sur une vraie boite : une partie non
// textuelle qui porte un nom de fichier est une piece jointe, quelle que soit
// sa disposition (des clients marquent "inline" des PDF et des photos) ; une
// partie inline sans nom (logo, signature) est notee a part.
func parties(n noeud, out *[]Piece) {
	if len(n.l) == 0 {
		return
	}
	if n.l[0].genre == 'l' { // multipart : sous-parties puis sous-type
		for _, x := range n.l {
			if x.genre != 'l' {
				break
			}
			parties(x, out)
		}
		return
	}
	if len(n.l) < 7 {
		return
	}
	typ, sous := chaine(n.l[0]), chaine(n.l[1])
	taille, _ := strconv.ParseInt(n.l[6].s, 10, 64)
	idxDispo := 8 // md5 en 7, disposition en 8
	switch {
	case typ == "text":
		idxDispo = 9 // + nombre de lignes
	case typ == "message" && sous == "rfc822":
		idxDispo = 11 // + enveloppe, corps, lignes
	}
	dispo, nom := "", ""
	if idxDispo < len(n.l) && n.l[idxDispo].genre == 'l' && len(n.l[idxDispo].l) >= 1 {
		dispo = chaine(n.l[idxDispo].l[0])
		if len(n.l[idxDispo].l) >= 2 {
			nom = param(n.l[idxDispo].l[1], "filename")
		}
	}
	if nom == "" {
		nom = param(n.l[2], "name")
	}
	switch {
	case typ == "message" && sous == "rfc822", dispo == "attachment":
	case typ == "text" || typ == "multipart":
		return
	case nom != "":
	case dispo == "inline" || typ == "image":
		*out = append(*out, Piece{Type: Categorie(typ, sous, nom), Taille: taille, Inline: true})
		return
	}
	*out = append(*out, Piece{Type: Categorie(typ, sous, nom), Taille: taille})
}

// Categorie classe une partie d'apres son type MIME et l'extension du nom.
func Categorie(typ, sous, nom string) string {
	ext := ""
	if i := strings.LastIndex(nom, "."); i >= 0 {
		ext = strings.ToLower(nom[i+1:])
	}
	contient := func(mots ...string) bool {
		for _, m := range mots {
			if strings.Contains(sous, m) {
				return true
			}
		}
		return false
	}
	dans := func(exts ...string) bool {
		for _, e := range exts {
			if ext == e {
				return true
			}
		}
		return false
	}
	switch {
	case sous == "pdf" || ext == "pdf":
		return "pdf"
	case typ == "image":
		return "image"
	case typ == "video":
		return "video"
	case typ == "audio":
		return "audio"
	case typ == "message":
		return "e-mail"
	case sous == "calendar" || ext == "ics":
		return "invitation"
	case contient("zip", "rar", "7z", "x-tar", "gzip") || dans("zip", "rar", "7z", "gz", "tgz", "tar"):
		return "archive"
	case contient("msword", "officedocument", "ms-excel", "ms-powerpoint", "opendocument") ||
		dans("doc", "docx", "xls", "xlsx", "ppt", "pptx", "odt", "ods", "odp", "rtf"):
		return "bureautique"
	case dans("jpg", "jpeg", "png", "gif", "heic", "webp"):
		return "image"
	case dans("mp4", "mov", "avi", "mkv"):
		return "video"
	}
	return "autre"
}
