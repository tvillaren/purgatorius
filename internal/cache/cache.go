// Package cache est la base SQLite locale d'un compte Gmail : les metadonnees
// des messages (jamais leur corps), lues par IMAP. Une base par compte, sous
// cache/comptes/<id>/cache.db : re-derivable par une synchro, donc dans cache/.
//
// Le Purgatoire, lui, est le travail de l'utilisateur et ne se re-derive pas :
// il vit dans une base a part, travail/comptes/<id>/purgatoire.db, rattachee
// (ATTACH, schema « purg ») a chaque connexion du cache. Il survit ainsi a la
// recreation du cache (changement de schema, fichier illisible).
//
// Driver modernc.org/sqlite (pur Go) : la compilation croisee Windows ARM64 de
// la CI reste possible sans cgo.
package cache

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/louis49/purgatorius/internal/store"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Cache : la base d'un compte. Une seule connexion par Cache ; la synchro et
// la carte (lecture, Purgatoire) passent par des connexions distinctes.
// Toute transaction prend le verrou d'ecriture des son debut (_txlock=immediate)
// : elle attend alors (busy_timeout) qu'une autre connexion ait fini d'ecrire.
// Une transaction ouverte en lecture qui ecrit ensuite echouerait, elle, sans
// attendre (SQLITE_BUSY) si une autre connexion ecrit.
type Cache struct {
	db *sql.DB
	// Reinitialise : la base, d'un schema anterieur ou illisible, a ete
	// recreee vide a l'ouverture ; le compte doit etre resynchronise.
	Reinitialise bool
	// PurgatoireMisDeCote : nom du fichier sous lequel le Purgatoire,
	// illisible, a ete mis de cote a l'ouverture ("" sinon). Le Purgatoire
	// est alors neuf, donc vide : l'utilisateur doit en etre averti.
	PurgatoireMisDeCote string
}

// CheminRelatif : emplacement de la base d'un compte dans le dossier de donnees.
func CheminRelatif(id string) string { return "cache/comptes/" + id + "/cache.db" }

// CheminPurgatoire : emplacement du Purgatoire d'un compte (dossier travail/,
// a conserver).
func CheminPurgatoire(id string) string { return "travail/comptes/" + id + "/purgatoire.db" }

// Ouvrir ouvre (ou cree) la base du compte id et son Purgatoire.
func Ouvrir(id string) (*Cache, error) {
	if id == "" {
		return nil, errors.New("identifiant de compte vide")
	}
	p, err := store.CheminAbsolu(CheminRelatif(id))
	if err != nil {
		return nil, err
	}
	pp, err := store.CheminAbsolu(CheminPurgatoire(id))
	if err != nil {
		return nil, err
	}
	return OuvrirChemins(p, pp)
}

// OuvrirChemin ouvre une base a un chemin absolu, avec son Purgatoire a cote
// (purgatoire.db, meme dossier) : pour les tests et les outils.
func OuvrirChemin(p string) (*Cache, error) {
	return OuvrirChemins(p, filepath.Join(filepath.Dir(p), "purgatoire.db"))
}

// OuvrirChemins ouvre (ou cree) la base p et le Purgatoire purg. Les fichiers
// sont crees en 0600 avant que SQLite ne les ouvre : SQLite reprend ces droits
// pour les fichiers -wal et -shm.
//
// La base est recreee vide (Reinitialise) si son schema est anterieur (avant
// publication, le schema change sans migration), plus recent que
// l'application, ou si le fichier est illisible (tronque, corrompu), y compris
// quand l'abime n'apparait qu'a la premiere lecture apres l'ouverture (qui ne
// lit que l'en-tete et le schema). Toute autre erreur remonte sans rien
// effacer : une erreur passagere ne doit pas couter une resynchro. Le
// Purgatoire n'est jamais efface : illisible (ou en echec au controle rapide
// de sa structure), il est mis de cote sous un autre nom et remplace par un
// neuf (PurgatoireMisDeCote).
func OuvrirChemins(p, purg string) (*Cache, error) {
	cote := ""
	c, err := ouvrir(p, purg)
	if err != nil && enPurgatoire(err) && illisible(err) {
		de := purg + ".illisible-" + time.Now().Format("20060102-150405")
		if errR := mettreDeCote(purg, de); errR != nil {
			return nil, errR
		}
		cote = filepath.Base(de)
		log.Printf("purgatoire illisible (%v), mis de cote : %s", err, cote)
		c, err = ouvrir(p, purg)
	}
	if err == nil {
		if err = c.verifierEmpreinte(); err != nil {
			c.Fermer()
		}
	}
	if err != nil {
		if enPurgatoire(err) || !(illisible(err) || errors.Is(err, errSchemaRecent) || errors.Is(err, errSchemaAncien)) {
			return nil, err
		}
		if c, err = recreer(p, purg); err != nil {
			return nil, err
		}
	}
	c.PurgatoireMisDeCote = cote
	return c, nil
}

// verifierEmpreinte compare l'empreinte du schema de la base a celle de
// l'application. C'est la premiere lecture d'une table : une page abimee y
// apparait (erreur illisible).
func (c *Cache) verifierEmpreinte() error {
	e, err := c.Meta("empreinte_schema")
	if err != nil {
		return err
	}
	if e != empreinteSchema() {
		return errSchemaAncien
	}
	return nil
}

// recreer efface la base p (et ses -wal, -shm) et en ouvre une neuve. Le
// Purgatoire, dans un autre fichier, est garde.
func recreer(p, purg string) (*Cache, error) {
	for _, suffixe := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(p + suffixe); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	c, err := ouvrir(p, purg)
	if err != nil {
		return nil, err
	}
	c.Reinitialise = true
	return c, nil
}

// mettreDeCote renomme un fichier SQLite et ses -wal, -shm.
func mettreDeCote(p, de string) error {
	for _, suffixe := range []string{"", "-wal", "-shm"} {
		if err := os.Rename(p+suffixe, de+suffixe); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

var (
	// errSchemaRecent : base d'une version de l'application plus recente.
	errSchemaRecent = errors.New("base plus recente que l'application")
	// errSchemaAncien : base creee avec un autre texte de schema.
	errSchemaAncien = errors.New("schema anterieur")
	// errCorrompu : le controle rapide de structure (quick_check) a echoue.
	errCorrompu = errors.New("structure abimee")
)

// illisible : SQLite ne reconnait pas le fichier comme une base, ou la trouve
// corrompue.
func illisible(err error) bool {
	if errors.Is(err, errCorrompu) {
		return true
	}
	var e *sqlite.Error
	if !errors.As(err, &e) {
		return false
	}
	code := e.Code() & 0xff
	return code == sqlite3.SQLITE_NOTADB || code == sqlite3.SQLITE_CORRUPT
}

// errPurgatoire : erreur survenue sur la base du Purgatoire.
type errPurgatoire struct{ err error }

func (e errPurgatoire) Error() string { return "purgatoire : " + e.err.Error() }
func (e errPurgatoire) Unwrap() error { return e.err }

func enPurgatoire(err error) bool {
	var e errPurgatoire
	return errors.As(err, &e)
}

// schemaPurgatoire : messages mis de cote avant la corbeille, designes par
// X-GM-MSGID (stable, contrairement a l'UID).
const schemaPurgatoire = `CREATE TABLE IF NOT EXISTS purg.purgatoire (
	gm_msgid INTEGER PRIMARY KEY,
	ajoute   INTEGER NOT NULL  -- secondes Unix
)`

// connecteur ouvre les connexions du cache : chacune rattache le Purgatoire,
// y compris celles que database/sql rouvrirait apres une erreur.
type connecteur struct {
	driver.Connector
	purg string
}

func (k connecteur) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := k.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	ex, ok := conn.(driver.ExecerContext)
	if !ok {
		conn.Close()
		return nil, errors.New("connexion SQLite sans ExecContext")
	}
	etapes := []struct {
		sql    string
		params []driver.NamedValue
	}{
		{"ATTACH DATABASE ? AS purg", []driver.NamedValue{{Ordinal: 1, Value: k.purg}}},
		{"PRAGMA purg.journal_mode = WAL", nil},
		{schemaPurgatoire, nil},
	}
	for _, e := range etapes {
		if _, err := ex.ExecContext(ctx, e.sql, e.params); err != nil {
			conn.Close()
			return nil, errPurgatoire{err}
		}
	}
	return conn, nil
}

func ouvrir(p, purg string) (*Cache, error) {
	for _, f := range []string{p, purg} {
		if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
			return nil, err
		}
		fic, err := os.OpenFile(f, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return nil, err
		}
		fic.Close()
	}
	base, err := sqlite.NewConnector(adresse(p))
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connecteur{Connector: base, purg: purg})
	db.SetMaxOpenConns(1)
	c := &Cache{db: db}
	if err := c.migrer(); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema du cache : %w", err)
	}
	if err := c.verifierPurgatoire(); err != nil {
		db.Close()
		return nil, err
	}
	return c, nil
}

// adresse : URI SQLite de la base p, avec ses reglages. Un chemin Windows
// (C:\...) devient file:///C:/... : sans la barre initiale, file://C:/...
// ferait de « C: » le nom d'un hote, que SQLite refuse.
func adresse(p string) string {
	chemin := filepath.ToSlash(p)
	if !strings.HasPrefix(chemin, "/") {
		chemin = "/" + chemin
	}
	u := url.URL{Scheme: "file", Path: chemin}
	return u.String() + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_txlock=immediate"
}

// verifierPurgatoire controle la structure du Purgatoire (quick_check, rapide
// sur ce petit fichier). Une page abimee, que l'ouverture ne lit pas, ferait
// sinon echouer chaque requete de la carte, qui toutes le consultent.
func (c *Cache) verifierPurgatoire() error {
	var res string
	if err := c.db.QueryRow("PRAGMA purg.quick_check(1)").Scan(&res); err != nil {
		return errPurgatoire{err}
	}
	if res != "ok" {
		return errPurgatoire{fmt.Errorf("%w : %s", errCorrompu, res)}
	}
	return nil
}

// empreinteSchema identifie le texte du schema : une base creee avec un autre
// texte est recreee (voir OuvrirChemins).
func empreinteSchema() string {
	h := sha256.Sum256([]byte(strings.Join(migrations, "\x00")))
	return hex.EncodeToString(h[:8])
}

// Fermer ferme la base.
func (c *Cache) Fermer() error { return c.db.Close() }

// migrations : une entree par version de schema (PRAGMA user_version). Tant
// que l'application n'est pas publiee, on modifie la version 1 directement :
// l'empreinte du schema change, la base est recreee a l'ouverture et le compte
// resynchronise de zero (pas de migration sans utilisateurs a proteger).
var migrations = []string{
	`CREATE TABLE meta (
		cle    TEXT PRIMARY KEY,
		valeur TEXT NOT NULL
	);
	CREATE TABLE messages (
		uid            INTEGER PRIMARY KEY,
		gm_msgid       INTEGER NOT NULL,
		gm_thrid       INTEGER NOT NULL,
		taille         INTEGER NOT NULL,
		date           INTEGER NOT NULL,   -- INTERNALDATE, secondes Unix
		lu             INTEGER NOT NULL,
		expediteur     TEXT NOT NULL,      -- adresse en minuscules
		nom_expediteur TEXT NOT NULL,
		hote           TEXT NOT NULL,      -- partie domaine de l'adresse
		domaine        TEXT NOT NULL,      -- domaine enregistrable (publicsuffix)
		objet          TEXT NOT NULL,
		objet_norm     TEXT NOT NULL,
		categorie      TEXT NOT NULL DEFAULT 'principale',
		pj_taille      INTEGER NOT NULL DEFAULT 0, -- taille encodee des vraies pieces jointes
		objet_illisible INTEGER NOT NULL DEFAULT 0,
		boite          TEXT NOT NULL DEFAULT 'recu', -- recu / envoye / brouillon (libelles \Sent, \Draft)
		destinataire   TEXT NOT NULL DEFAULT ''      -- premier destinataire (To puis Cc)
	);
	CREATE INDEX messages_domaine    ON messages(domaine);
	CREATE INDEX messages_expediteur ON messages(expediteur);
	CREATE INDEX messages_date       ON messages(date);
	CREATE INDEX messages_boite      ON messages(boite);
	CREATE INDEX messages_destinataire ON messages(destinataire);
	CREATE TABLE libelles (
		uid     INTEGER NOT NULL REFERENCES messages(uid) ON DELETE CASCADE,
		libelle TEXT NOT NULL,
		PRIMARY KEY (uid, libelle)
	) WITHOUT ROWID;
	CREATE TABLE pieces (
		uid    INTEGER NOT NULL REFERENCES messages(uid) ON DELETE CASCADE,
		rang   INTEGER NOT NULL,
		type   TEXT NOT NULL,
		taille INTEGER NOT NULL,
		inline INTEGER NOT NULL,
		PRIMARY KEY (uid, rang)
	) WITHOUT ROWID;
	CREATE TABLE destinataires (
		uid     INTEGER NOT NULL REFERENCES messages(uid) ON DELETE CASCADE,
		rang    INTEGER NOT NULL,
		adresse TEXT NOT NULL,
		PRIMARY KEY (uid, rang)
	) WITHOUT ROWID;
	CREATE INDEX messages_gm_msgid ON messages(gm_msgid);`,
}

func (c *Cache) migrer() error {
	var v int
	if err := c.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	if v > len(migrations) {
		return fmt.Errorf("%w : version %d, application %d", errSchemaRecent, v, len(migrations))
	}
	for i := v; i < len(migrations); i++ {
		tx, err := c.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	if v < len(migrations) {
		return c.EcrireMeta("empreinte_schema", empreinteSchema())
	}
	return nil
}

// Meta lit une valeur de la table meta ("" si absente).
func (c *Cache) Meta(cle string) (string, error) {
	var v string
	err := c.db.QueryRow("SELECT valeur FROM meta WHERE cle = ?", cle).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// EcrireMeta ecrit une valeur de la table meta.
func (c *Cache) EcrireMeta(cle, valeur string) error {
	_, err := c.db.Exec("INSERT INTO meta(cle, valeur) VALUES(?, ?) ON CONFLICT(cle) DO UPDATE SET valeur = excluded.valeur", cle, valeur)
	return err
}
