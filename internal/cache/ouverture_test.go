package cache

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/louis49/purgatorius/internal/imap"
)

// unMessage : un message de gm_msgid donne, sous l'UID donne.
func unMessage(uid uint32, gm uint64) imap.Message {
	return imap.Message{UID: uid, GmMsgID: gm, De: "a@x.example", Taille: 100, Date: time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)}
}

func ecrireDechet(t *testing.T, p string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(strings.Repeat("pas une base SQLite ", 300)), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Changement de schema : la base est recreee, le Purgatoire (fichier a part)
// est garde.
func TestPurgatoireSurvitALaRecreation(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cache.db")
	c, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	c.Enregistrer([]imap.Message{unMessage(1, 0x10), unMessage(2, 0x20)})
	if lot, err := c.EnvoyerPurgatoire([]uint32{2}); err != nil || lot.Nombre != 1 {
		t.Fatalf("envoi : %+v %v", lot, err)
	}
	c.EcrireMeta("empreinte_schema", "ancienne")
	c.Fermer()

	c, err = OuvrirChemin(p)
	if err != nil || !c.Reinitialise {
		t.Fatalf("recreation attendue : %v", err)
	}
	defer c.Fermer()
	// La resynchro relit le message, sous un autre UID : il est toujours au
	// Purgatoire (designe par son X-GM-MSGID).
	c.Enregistrer([]imap.Message{unMessage(1, 0x10), unMessage(7, 0x20)})
	idx, err := c.IndexListe(RequeteListe{Purgatoire: true, Tri: "date"})
	if err != nil || len(idx.UIDs) != 1 || idx.UIDs[0] != 7 {
		t.Fatalf("Purgatoire apres recreation : %+v %v", idx, err)
	}
}

// Vidage du cache (UIDVALIDITY changee) : le Purgatoire reste. Nettoyage : un
// message disparu de la boite en sort.
func TestPurgatoireSurvitAuVidageEtNettoyage(t *testing.T) {
	c := ouvrirTest(t)
	c.Enregistrer([]imap.Message{unMessage(1, 0x10), unMessage(2, 0x20), unMessage(3, 0x30)})
	c.EnvoyerPurgatoire([]uint32{2, 3})
	if err := c.Vider(); err != nil {
		t.Fatal(err)
	}
	c.Enregistrer([]imap.Message{unMessage(11, 0x10), unMessage(12, 0x20), unMessage(13, 0x30)})
	if e, _ := c.EtatDuPurgatoire(); e.Nombre != 2 {
		t.Fatalf("Purgatoire apres vidage : %+v", e)
	}
	// 13 supprime dans Gmail : la synchro l'enleve du cache, le nettoyage du
	// Purgatoire.
	c.Supprimer([]uint32{13})
	if n, err := c.NettoyerPurgatoire(); err != nil || n != 1 {
		t.Fatalf("nettoyage : %d %v", n, err)
	}
	if n, _ := c.NettoyerPurgatoire(); n != 0 {
		t.Fatalf("second nettoyage : %d", n)
	}
	if e, _ := c.EtatDuPurgatoire(); e.Nombre != 1 {
		t.Fatalf("Purgatoire apres nettoyage : %+v", e)
	}
}

// Un message sans X-GM-MSGID ne part pas au Purgatoire : il ne pourrait pas
// etre retrouve chez Gmail (et tous les messages a 0 partiraient ensemble).
func TestPurgatoireRefuseGmMsgIDNul(t *testing.T) {
	c := ouvrirTest(t)
	c.Enregistrer([]imap.Message{unMessage(1, 0), unMessage(2, 0), unMessage(3, 0x30)})
	lot, err := c.EnvoyerPurgatoire([]uint32{1, 3})
	if err != nil || lot.Nombre != 1 || lot.UIDs[0] != 3 {
		t.Fatalf("envoi : %+v %v", lot, err)
	}
	if e, _ := c.EtatDuPurgatoire(); e.Nombre != 1 {
		t.Fatalf("etat : %+v", e)
	}
}

// Fichier du cache illisible (tronque, ecrase) : recree, compte resynchronise.
func TestCacheIllisibleRecree(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cache.db")
	ecrireDechet(t, p)
	c, err := OuvrirChemin(p)
	if err != nil || !c.Reinitialise {
		t.Fatalf("recreation attendue : %v", err)
	}
	defer c.Fermer()
	if err := c.Enregistrer([]imap.Message{unMessage(1, 0x10)}); err != nil {
		t.Fatal(err)
	}
}

// Purgatoire illisible : mis de cote (jamais efface), remplace par un neuf ;
// le cache, lui, n'est pas recree.
func TestPurgatoireIllisibleMisDeCote(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "cache.db")
	c, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	c.Enregistrer([]imap.Message{unMessage(1, 0x10)})
	c.Fermer()
	for _, s := range []string{"-wal", "-shm"} {
		os.Remove(filepath.Join(d, "purgatoire.db"+s))
	}
	ecrireDechet(t, filepath.Join(d, "purgatoire.db"))

	c, err = OuvrirChemin(p)
	if err != nil || c.Reinitialise {
		t.Fatalf("ouverture : reinitialise=%v %v", c != nil && c.Reinitialise, err)
	}
	defer c.Fermer()
	if r, _ := c.Resumer(); r.Messages != 1 {
		t.Fatalf("cache perdu : %+v", r)
	}
	if lot, err := c.EnvoyerPurgatoire([]uint32{1}); err != nil || lot.Nombre != 1 {
		t.Fatalf("Purgatoire neuf inutilisable : %+v %v", lot, err)
	}
	cote, _ := filepath.Glob(filepath.Join(d, "purgatoire.db.illisible-*"))
	if len(cote) != 1 || c.PurgatoireMisDeCote != filepath.Base(cote[0]) {
		t.Fatalf("fichier mis de cote : %v, annonce %q", cote, c.PurgatoireMisDeCote)
	}
}

// Un chemin Windows donne file:///C:/... (file://C:/... ferait de
// « C: » un nom d'hote, refuse par SQLite). Teste sur tout systeme.
func TestAdresseSQLite(t *testing.T) {
	cas := []struct{ chemin, debut string }{
		{"C:/Users/x y/AppData/Local/purgatorius/cache/comptes/ab/cache.db", "file:///C:/Users/x%20y/AppData/Local/purgatorius/cache/comptes/ab/cache.db?"},
		{"/Users/x/Library/Application Support/purgatorius/cache.db", "file:///Users/x/Library/Application%20Support/purgatorius/cache.db?"},
		{"/home/x/a#b?c%d/cache.db", "file:///home/x/a%23b%3Fc%25d/cache.db?"},
	}
	for _, k := range cas {
		if a := adresse(k.chemin); !strings.HasPrefix(a, k.debut) {
			t.Errorf("%s : %s", k.chemin, a)
		}
	}
}

// Les caracteres reserves des URI dans le chemin (espace, #, %, ?) ne
// deroutent pas l'ouverture : la base est bien au chemin demande.
func TestCheminAvecCaracteresReserves(t *testing.T) {
	nom := "a b#c%d"
	if runtime.GOOS != "windows" {
		nom += "?e"
	}
	d := filepath.Join(t.TempDir(), nom)
	p := filepath.Join(d, "cache.db")
	c, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Enregistrer([]imap.Message{unMessage(1, 0x10)}); err != nil {
		t.Fatal(err)
	}
	c.Fermer()
	c, err = OuvrirChemin(p)
	if err != nil || c.Reinitialise {
		t.Fatalf("reouverture : %v", err)
	}
	defer c.Fermer()
	if r, _ := c.Resumer(); r.Messages != 1 {
		t.Fatalf("base relue ailleurs qu'au chemin demande : %+v", r)
	}
	entrees, _ := os.ReadDir(filepath.Dir(d))
	if len(entrees) != 1 {
		t.Fatalf("fichiers crees hors du dossier : %v", entrees)
	}
}

// abimerPages met a zero les pages d'une base SQLite fermee (rangs de
// premiere a derniere inclus, 1 = la premiere ; derniere 0 = jusqu'a la fin).
func abimerPages(t *testing.T, p string, premiere, derniere int) {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	taille := int(b[16])<<8 | int(b[17])
	if taille == 1 {
		taille = 65536
	}
	if derniere == 0 {
		derniere = len(b) / taille
	}
	if derniere*taille > len(b) || premiere < 2 {
		t.Fatalf("pages %d a %d hors de la base (%d octets)", premiere, derniere, len(b))
	}
	clear(b[(premiere-1)*taille : derniere*taille])
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// fermerSansWAL ecrit le WAL dans les fichiers et ferme la base : ce qui est
// abime ensuite est bien ce que lira l'ouverture suivante.
func fermerSansWAL(t *testing.T, c *Cache) {
	t.Helper()
	for _, q := range []string{"PRAGMA main.wal_checkpoint(TRUNCATE)", "PRAGMA purg.wal_checkpoint(TRUNCATE)"} {
		if _, err := c.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	c.Fermer()
}

// Cache abime au-dela de son en-tete et de son schema. L'ouverture
// reussit, la premiere lecture echoue : la base est recreee (le Purgatoire,
// a part, est garde) au lieu de bloquer carte et synchro a chaque lancement.
func TestCacheAbimeApresOuvertureRecree(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "cache.db")
	c, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	var msgs []imap.Message
	for i := uint32(1); i <= 400; i++ {
		msgs = append(msgs, unMessage(i, uint64(0x1000+i)))
	}
	c.Enregistrer(msgs)
	if lot, err := c.EnvoyerPurgatoire([]uint32{1, 2}); err != nil || lot.Nombre != 2 {
		t.Fatalf("envoi : %+v %v", lot, err)
	}
	fermerSansWAL(t, c)
	abimerPages(t, p, 2, 0)

	c, err = OuvrirChemin(p)
	if err != nil || !c.Reinitialise {
		t.Fatalf("recreation attendue : %v", err)
	}
	defer c.Fermer()
	var n int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM purg.purgatoire").Scan(&n); err != nil || n != 2 {
		t.Fatalf("Purgatoire perdu : %d %v", n, err)
	}
	if c.PurgatoireMisDeCote != "" {
		t.Fatalf("Purgatoire sain mis de cote : %s", c.PurgatoireMisDeCote)
	}
	if err := c.Enregistrer([]imap.Message{unMessage(1, 0x1001)}); err != nil {
		t.Fatal(err)
	}
}

// Purgatoire abime dans ses donnees, en-tete et schema intacts.
// L'ouverture le controle : il est mis de cote (et annonce), la carte reste
// utilisable, le cache n'est pas touche.
func TestPurgatoireAbimeMisDeCote(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "cache.db")
	purg := filepath.Join(d, "purgatoire.db")
	c, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	var msgs []imap.Message
	var uids []uint32
	for i := uint32(1); i <= 3000; i++ {
		msgs = append(msgs, unMessage(i, uint64(0x1000+i)))
		uids = append(uids, i)
	}
	c.Enregistrer(msgs)
	if lot, err := c.EnvoyerPurgatoire(uids); err != nil || lot.Nombre != 3000 {
		t.Fatalf("envoi : %+v %v", lot, err)
	}
	fermerSansWAL(t, c)
	abimerPages(t, purg, 3, 3)

	c, err = OuvrirChemin(p)
	if err != nil || c.Reinitialise {
		t.Fatalf("ouverture : reinitialise=%v %v", c != nil && c.Reinitialise, err)
	}
	defer c.Fermer()
	cote, _ := filepath.Glob(purg + ".illisible-*")
	if len(cote) != 1 || c.PurgatoireMisDeCote != filepath.Base(cote[0]) {
		t.Fatalf("fichier mis de cote : %v, annonce %q", cote, c.PurgatoireMisDeCote)
	}
	if e, err := c.EtatDuPurgatoire(); err != nil {
		t.Fatalf("carte bloquee : %+v %v", e, err)
	}
	if r, _ := c.Resumer(); r.Messages != 3000 {
		t.Fatalf("cache touche : %+v", r)
	}
}

// Base d'une version plus recente de l'application : recreee (avant
// publication, pas de migration descendante).
func TestBasePlusRecenteRecreee(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cache.db")
	c, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	c.db.Exec("PRAGMA user_version = 99")
	c.Fermer()
	c, err = OuvrirChemin(p)
	if err != nil || !c.Reinitialise {
		t.Fatalf("recreation attendue : %v", err)
	}
	c.Fermer()
}

// Une autre erreur a l'ouverture (ici, fichier inaccessible) remonte sans rien
// effacer : une erreur passagere ne doit pas couter le cache.
func TestErreurOuvertureSansEffacement(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("droits Unix d'un utilisateur ordinaire requis")
	}
	p := filepath.Join(t.TempDir(), "cache.db")
	c, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	c.Enregistrer([]imap.Message{unMessage(1, 0x10)})
	c.Fermer()
	os.Chmod(p, 0)
	if c, err := OuvrirChemin(p); err == nil {
		c.Fermer()
		t.Fatal("ouverture d'un fichier inaccessible acceptee")
	}
	os.Chmod(p, 0o600)
	c, err = OuvrirChemin(p)
	if err != nil || c.Reinitialise {
		t.Fatalf("reouverture : %v", err)
	}
	defer c.Fermer()
	if r, _ := c.Resumer(); r.Messages != 1 {
		t.Fatalf("cache efface : %+v", r)
	}
}

// Deux connexions (synchro et carte) qui ecrivent le meme fichier (ici le
// Purgatoire : nettoyage en fin de synchro) : la transaction du Purgatoire,
// qui lit avant d'ecrire, attend que l'autre ait fini, au lieu d'echouer
// aussitot (SQLITE_BUSY) comme une transaction ouverte en lecture.
func TestPurgatoireAttendLaSynchro(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cache.db")
	synchro, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	defer synchro.Fermer()
	carte, err := OuvrirChemin(p)
	if err != nil {
		t.Fatal(err)
	}
	defer carte.Fermer()
	synchro.Enregistrer([]imap.Message{unMessage(1, 0x10)})

	tx, err := synchro.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("INSERT INTO meta(cle, valeur) VALUES ('lot', 'en cours')"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("DELETE FROM purg.purgatoire WHERE gm_msgid = 999"); err != nil {
		t.Fatal(err)
	}
	fin := make(chan error, 1)
	go func(tx *sql.Tx) {
		time.Sleep(400 * time.Millisecond)
		fin <- tx.Commit()
	}(tx)
	debut := time.Now()
	lot, err := carte.EnvoyerPurgatoire([]uint32{1})
	if err != nil || lot.Nombre != 1 {
		t.Fatalf("envoi pendant une ecriture de la synchro : %+v %v", lot, err)
	}
	if d := time.Since(debut); d < 300*time.Millisecond {
		t.Fatalf("l'envoi n'a pas attendu la fin de l'ecriture (%v)", d)
	}
	if err := <-fin; err != nil {
		t.Fatal(err)
	}
}
