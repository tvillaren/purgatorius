package imap

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
)

func lire(t *testing.T, s string) (string, []noeud) {
	t.Helper()
	l := &lecteur{bufio.NewReader(strings.NewReader(s))}
	tag, e, err := l.reponse()
	if err != nil {
		t.Fatalf("reponse(%q) : %v", s, err)
	}
	return tag, e
}

func TestLecteur(t *testing.T) {
	tag, e := lire(t, "* 12 FETCH (UID 7 BODY[HEADER.FIELDS (FROM SUBJECT)] {5}\r\nabc\r\n X-GM-LABELS (\"\\\\Inbox\" Perso) FLAGS NIL)\r\n")
	if tag != "*" || len(e) != 3 || e[1].s != "FETCH" {
		t.Fatalf("decoupage : %q %+v", tag, e)
	}
	l := e[2].l
	if l[2].s != "BODY[HEADER.FIELDS (FROM SUBJECT)]" || l[3].s != "abc\r\n" {
		t.Fatalf("atome a section ou litteral : %+v", l[2:4])
	}
	if l[5].l[0].s != `\Inbox` || l[5].l[1].s != "Perso" || l[7].genre != 'n' {
		t.Fatalf("liste, echappement ou NIL : %+v", l[4:])
	}
	_, e = lire(t, "p3 OK [READ-ONLY] EXAMINE completed (Success :)\r\n")
	if e[0].s != "OK" || !strings.Contains(e[1].s, "Success :)") {
		t.Fatalf("statut en texte libre : %+v", e)
	}
}

func TestLitteralAberrant(t *testing.T) {
	l := &lecteur{bufio.NewReader(strings.NewReader("* 1 FETCH (BODY[] {999999999999}\r\n"))}
	if _, _, err := l.reponse(); err == nil {
		t.Fatal("litteral geant accepte")
	}
}

func TestAnalyserFetch(t *testing.T) {
	entetes := "From: =?iso-8859-1?Q?Ren=E9?= <Rene@Exemple.Example>\r\nSubject: =?windows-1252?Q?R=E9union?=\r\n" +
		"To: Moi <Moi@Example.Invalid>, autre@x.example\r\nCc: autre@x.example, \"Equipe\" <equipe@y.example>\r\n\r\n"
	brut := "* 3 FETCH (UID 42 RFC822.SIZE 123456 INTERNALDATE \" 5-Mar-2015 09:30:00 +0100\" FLAGS (\\Seen) " +
		"X-GM-MSGID 1500000000000000001 X-GM-THRID 1500000000000000000 X-GM-LABELS (\\Inbox \\Important \"Mes &AOk-t&AOk-s\") " +
		"BODYSTRUCTURE ((\"text\" \"plain\" (\"charset\" \"utf-8\") NIL NIL \"7bit\" 10 1 NIL NIL NIL)" +
		"(\"application\" \"pdf\" (\"name\" \"facture.pdf\") NIL NIL \"base64\" 2000 NIL (\"inline\" (\"filename\" \"facture.pdf\")) NIL)" +
		"(\"image\" \"png\" NIL \"<logo>\" NIL \"base64\" 300 NIL (\"inline\" NIL) NIL) \"mixed\") " +
		"BODY[HEADER.FIELDS (FROM SUBJECT)] {" + strconv.Itoa(len(entetes)) + "}\r\n" + entetes + ")\r\n"
	_, e := lire(t, brut)
	m := analyserFetch(e[2].l, false)
	if m.UID != 42 || m.Taille != 123456 || !m.Lu || m.GmThrID != 1500000000000000000 || m.Date.Year() != 2015 {
		t.Fatalf("champs de base : %+v", m)
	}
	if m.De != "rene@exemple.example" || m.NomDe != "René" || m.Objet != "Réunion" || m.ObjetIllisible {
		t.Fatalf("en-tetes : de=%q nom=%q objet=%q", m.De, m.NomDe, m.Objet)
	}
	if strings.Join(m.Destinataires, ",") != "moi@example.invalid,autre@x.example,equipe@y.example" {
		t.Fatalf("destinataires (To puis Cc, sans doublon) : %q", m.Destinataires)
	}
	if len(m.Libelles) != 3 || m.Libelles[2] != "Mes étés" {
		t.Fatalf("libelles : %q", m.Libelles)
	}
	if len(m.Pieces) != 2 || m.Pieces[0] != (Piece{Type: "pdf", Taille: 2000}) || !m.Pieces[1].Inline || !m.AvecPieceJointe() {
		t.Fatalf("pieces (PDF inline nomme = piece jointe, image anonyme = inline) : %+v", m.Pieces)
	}
}

func TestCharsets(t *testing.T) {
	for _, cs := range []string{"windows-1252", "cp1252", "iso-8859-15", "iso8859-15", "ISO-8859-1", "utf8", "us-ascii", "gb2312", "windows-1256"} {
		if _, err := trouverEncodage(cs); err != nil {
			t.Errorf("%s : %v", cs, err)
		}
	}
	if _, err := trouverEncodage("dzcrt836fd4ipiuy2y9a4fixqutf-8"); err == nil {
		t.Error("jeu de caracteres fantaisiste accepte")
	}
}

func TestUTF7Modifie(t *testing.T) {
	for clair, code := range map[string]string{"Mes étés": "Mes &AOk-t&AOk-s", "A&B": "A&-B", "[Gmail]/Tous les messages": "[Gmail]/Tous les messages", "日本語": "&ZeVnLIqe-"} {
		if got := encoderUTF7Modifie(clair); got != code {
			t.Errorf("encoder(%q) = %q, attendu %q", clair, got, code)
		}
		if got := decoderUTF7Modifie(code); got != clair {
			t.Errorf("decoder(%q) = %q, attendu %q", code, got, clair)
		}
	}
}

func TestEnsemble(t *testing.T) {
	if got := Ensemble([]uint32{1, 2, 3, 7, 9, 10}); got != "1:3,7,9:10" {
		t.Fatalf("Ensemble = %q", got)
	}
}

// fauxServeur repond a une suite de commandes attendues (prefixe apres
// l'etiquette) par des lignes ; "%t" est remplace par l'etiquette recue.
func fauxServeur(t *testing.T, script [][2]string) *Client {
	c, _ := fauxServeurJournal(t, script)
	return c
}

// fauxServeurJournal : comme fauxServeur, et renvoie les commandes recues
// (lisibles une fois la session terminee).
func fauxServeurJournal(t *testing.T, script [][2]string) (*Client, *[]string) {
	t.Helper()
	var journal []string
	cli, srv := net.Pipe()
	go func() {
		defer srv.Close()
		w := bufio.NewWriter(srv)
		r := bufio.NewReader(srv)
		w.WriteString("* OK Gimap ready\r\n")
		w.Flush()
		for _, etape := range script {
			ligne, err := r.ReadString('\n')
			if err != nil {
				return
			}
			tag, cmd, _ := strings.Cut(strings.TrimRight(ligne, "\r\n"), " ")
			journal = append(journal, cmd)
			if !strings.HasPrefix(cmd, etape[0]) {
				w.WriteString(tag + " BAD inattendu\r\n")
				w.Flush()
				return
			}
			w.WriteString(strings.ReplaceAll(etape[1], "%t", tag))
			w.Flush()
		}
	}()
	c, err := nouveauClient(cli)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Fermer() })
	return c, &journal
}

// Aucune reponse du serveur a LOGIN ne fait apparaitre le mot de passe dans
// l'erreur : refus (NO), commande rejetee (BAD), fermeture (BYE), connexion
// coupee, ni un serveur qui recopierait la commande dans son texte.
func TestAuthentificationRefuseeSansFuite(t *testing.T) {
	cas := map[string]string{
		"NO":     "%t NO [AUTHENTICATIONFAILED] Invalid credentials (Failure)\r\n",
		"BAD":    "%t BAD LOGIN \"moi@example.invalid\" \"abcdefghijklmnop\"\r\n",
		"BYE":    "* BYE abcdefghijklmnop\r\n%t NO abcdefghijklmnop\r\n",
		"coupee": "",
	}
	for nom, rep := range cas {
		c := fauxServeur(t, [][2]string{{"LOGIN", rep}})
		err := c.Authentifier("moi@example.invalid", "abcd efgh ijkl mnop")
		if err == nil {
			t.Fatalf("%s : authentification acceptee", nom)
		}
		if nom == "NO" && !errors.Is(err, ErrAuthentification) {
			t.Fatalf("NO : erreur attendue ErrAuthentification, obtenu %v", err)
		}
		if strings.Contains(err.Error(), "abcd") || strings.Contains(err.Error(), "ijkl") {
			t.Fatalf("%s : le mot de passe apparait dans l'erreur : %v", nom, err)
		}
	}
}

// Session complete : chaque commande envoyee est en lecture seule (liste
// autorisee), et les en-tetes sont lus par BODY.PEEK (BODY[...] marquerait le
// message lu). La synchro entiere est verifiee de meme avec le faux serveur
// imaptest (paquet synchro).
func TestSessionLectureSeule(t *testing.T) {
	c, journal := fauxServeurJournal(t, [][2]string{
		{"LOGIN", "%t OK moi@example.invalid authenticated (Success)\r\n"},
		{"CAPABILITY", "* CAPABILITY IMAP4rev1 X-GM-EXT-1 CONDSTORE ENABLE UTF8=ACCEPT\r\n%t OK Success\r\n"},
		{"ENABLE UTF8=ACCEPT", "* ENABLED UTF8=ACCEPT\r\n%t OK Success\r\n"},
		{"LIST", "* LIST (\\All \\HasNoChildren) \"/\" \"[Gmail]/Tous les messages\"\r\n* LIST (\\HasNoChildren) \"/\" \"Été\"\r\n%t OK Success\r\n"},
		{`EXAMINE "[Gmail]/Tous les messages" (CONDSTORE)`, "* 3 EXISTS\r\n* OK [UIDVALIDITY 11] UIDs valid.\r\n* OK [HIGHESTMODSEQ 4242] Highest\r\n%t OK [READ-ONLY] Success\r\n"},
		{`UID SEARCH X-GM-RAW "category:promotions"`, "* SEARCH 5 9\r\n%t OK SEARCH completed\r\n"},
		{"UID FETCH 5,9 ", "* 1 FETCH (UID 5 RFC822.SIZE 10 X-GM-MSGID 105)\r\n* 2 FETCH (UID 9 RFC822.SIZE 20 X-GM-MSGID 109)\r\n%t OK Success\r\n"},
	})
	if err := c.Authentifier("moi@example.invalid", "x"); err != nil {
		t.Fatal(err)
	}
	ds, err := c.Dossiers()
	if err != nil || len(ds) != 2 || !ds[0].Special(`\All`) || ds[1].Nom != "Été" {
		t.Fatalf("dossiers : %+v %v", ds, err)
	}
	et, err := c.Examiner(ds[0].Nom)
	if err != nil || et != (Etat{Existe: 3, UIDValidity: 11, HighestModSeq: 4242}) {
		t.Fatalf("examiner : %+v %v", et, err)
	}
	uids, err := c.RechercherGmail("category:promotions")
	if err != nil || Ensemble(uids) != "5,9" {
		t.Fatalf("recherche : %v %v", uids, err)
	}
	var tailles int64
	if err := c.Recuperer(Ensemble(uids), func(m Message) { tailles += m.Taille }); err != nil || tailles != 30 {
		t.Fatalf("fetch : %d %v", tailles, err)
	}
	autorises := map[string]bool{"LOGIN": true, "CAPABILITY": true, "ENABLE": true, "LIST": true, "EXAMINE": true, "UID SEARCH": true, "UID FETCH": true}
	for _, cmd := range *journal {
		v, reste, _ := strings.Cut(cmd, " ")
		if v == "UID" {
			v2, _, _ := strings.Cut(reste, " ")
			v += " " + v2
		}
		if !autorises[v] {
			t.Errorf("commande hors lecture seule : %q", cmd)
		}
		if strings.Contains(cmd, "BODY[") {
			t.Errorf("corps lu sans PEEK : %q", cmd)
		}
		if v == "UID FETCH" && !strings.Contains(cmd, "BODY.PEEK[HEADER.FIELDS") {
			t.Errorf("en-tetes attendus par BODY.PEEK : %q", cmd)
		}
	}
	if len(*journal) != 7 {
		t.Fatalf("commandes recues : %q", *journal)
	}
}

// Un FETCH non sollicite (drapeaux change ailleurs) glisse au milieu d'un lot
// n'est pas transmis comme un message : il ecraserait le cache.
func TestRecupererIgnoreLesFetchNonSollicites(t *testing.T) {
	c := fauxServeur(t, [][2]string{
		{"UID FETCH 5:6 ", "* 1 FETCH (UID 5 RFC822.SIZE 10 X-GM-MSGID 105)\r\n" +
			"* 4 FETCH (UID 8 FLAGS (\\Seen) MODSEQ (12))\r\n" + // autre message, drapeaux seulement
			"* 3 FETCH (FLAGS (\\Seen))\r\n" + // sans UID
			"* 2 FETCH (UID 6 RFC822.SIZE 20 X-GM-MSGID 0)\r\n" + // identifiant Gmail nul
			"%t OK Success\r\n"},
	})
	var uids []uint32
	if err := c.Recuperer("5:6", func(m Message) { uids = append(uids, m.UID) }); err != nil {
		t.Fatal(err)
	}
	if len(uids) != 1 || uids[0] != 5 {
		t.Fatalf("messages transmis : %v, attendu [5]", uids)
	}
}

// Un « [ » hors section de message est un caractere ordinaire ; une
// section non fermee ne deborde pas sur la ligne suivante.
func TestCrochetsHorsSection(t *testing.T) {
	_, e := lire(t, "* 1 FETCH (UID 5 X-GM-LABELS (\\Inbox [projet (x) Autre) FLAGS ())\r\n")
	labs := e[2].l[3].l
	if len(labs) != 4 || labs[1].s != "[projet" || labs[2].genre != 'l' || labs[3].s != "Autre" {
		t.Fatalf("libelles : %+v", labs)
	}
	l := &lecteur{bufio.NewReader(strings.NewReader("* 1 FETCH (BODY[HEADER (FROM)\r\n* 2 FETCH (UID 9 X])\r\n"))}
	if _, _, err := l.reponse(); err == nil || !strings.Contains(err.Error(), "section non fermee") {
		t.Fatalf("section non fermee : %v", err)
	}
}

// Imbrication et longueurs bornees (une reponse pathologique donne une
// erreur, pas un plantage ni une allocation sans fin).
func TestReponsesPathologiques(t *testing.T) {
	cas := map[string]string{
		"imbrication": "* 1 FETCH " + strings.Repeat("(", 200000) + "\r\n",
		"chaine":      "* 1 FETCH (X \"" + strings.Repeat("a", tailleMaxJeton+10) + "\")\r\n",
		"atome":       "* 1 FETCH (X " + strings.Repeat("a", tailleMaxJeton+10) + ")\r\n",
		"statut":      "* OK " + strings.Repeat("a", tailleMaxJeton+10) + "\r\n",
	}
	for nom, brut := range cas {
		l := &lecteur{bufio.NewReader(strings.NewReader(brut))}
		if _, _, err := l.reponse(); err == nil {
			t.Errorf("%s : reponse acceptee", nom)
		}
	}
	// Juste sous les bornes : accepte.
	_, e := lire(t, "* 1 FETCH "+strings.Repeat("(", profondeurMax)+strings.Repeat(")", profondeurMax)+"\r\n")
	if len(e) != 3 {
		t.Fatalf("imbrication admise : %+v", e)
	}
}

// flotSansFin : n octets c, puis EOF ; compte ce qui a ete lu.
type flotSansFin struct {
	c      byte
	n, lus int
}

func (f *flotSansFin) Read(p []byte) (int, error) {
	if f.lus >= f.n {
		return 0, io.EOF
	}
	k := min(len(p), f.n-f.lus)
	for i := range p[:k] {
		p[i] = f.c
	}
	f.lus += k
	return k, nil
}

// La taille d'un litteral ({n}) est lue avec une borne : un
// « { » suivi d'un flot sans « } » ne fait pas croitre la memoire.
func TestTailleLitteralBornee(t *testing.T) {
	f := &flotSansFin{c: '9', n: 8 << 20}
	l := &lecteur{bufio.NewReader(io.MultiReader(strings.NewReader("* 1 FETCH (BODY[] {"), f))}
	if _, _, err := l.reponse(); err == nil {
		t.Fatal("litteral sans fin accepte")
	}
	if f.lus > 64<<10 {
		t.Fatalf("%d octets lus pour une taille de litteral", f.lus)
	}
}

// Un message legitime tres imbrique (transferts emboites en
// piece jointe) passe, au lieu de bloquer son lot a chaque synchro.
func TestImbricationProfondeAdmise(t *testing.T) {
	const prof = 300
	_, e := lire(t, "* 1 FETCH "+strings.Repeat("(", prof)+strings.Repeat(")", prof)+"\r\n")
	if len(e) != 3 {
		t.Fatalf("imbrication de %d niveaux : %+v", prof, e)
	}
}
