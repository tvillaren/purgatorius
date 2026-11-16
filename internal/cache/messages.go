package cache

import (
	"database/sql"
	"regexp"
	"strings"

	"github.com/louis49/purgatorius/internal/imap"

	"golang.org/x/net/publicsuffix"
)

// Enregistrer insere ou remplace des messages, dans une seule transaction.
func (c *Cache) Enregistrer(msgs []imap.Message) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	insMsg, err := tx.Prepare(`INSERT INTO messages
		(uid, gm_msgid, gm_thrid, taille, date, lu, expediteur, nom_expediteur, hote, domaine, objet, objet_norm, pj_taille, objet_illisible,
		 boite, destinataire)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(uid) DO UPDATE SET
			gm_msgid = excluded.gm_msgid, gm_thrid = excluded.gm_thrid, taille = excluded.taille,
			date = excluded.date, lu = excluded.lu, expediteur = excluded.expediteur,
			nom_expediteur = excluded.nom_expediteur, hote = excluded.hote, domaine = excluded.domaine,
			objet = excluded.objet, objet_norm = excluded.objet_norm, pj_taille = excluded.pj_taille,
			objet_illisible = excluded.objet_illisible, boite = excluded.boite,
			destinataire = excluded.destinataire`)
	if err != nil {
		return err
	}
	defer insMsg.Close()
	// Une requete qui ne se prepare pas renvoie une erreur (pas de pointeur
	// nil utilise plus loin : une panique arreterait toute l'application).
	var supLib, insLib, supPie, insPie, supDest, insDest *sql.Stmt
	for _, r := range []struct {
		st  **sql.Stmt
		sql string
	}{
		{&supLib, "DELETE FROM libelles WHERE uid = ?"},
		{&insLib, "INSERT OR IGNORE INTO libelles(uid, libelle) VALUES (?, ?)"},
		{&supPie, "DELETE FROM pieces WHERE uid = ?"},
		{&insPie, "INSERT INTO pieces(uid, rang, type, taille, inline) VALUES (?, ?, ?, ?, ?)"},
		{&supDest, "DELETE FROM destinataires WHERE uid = ?"},
		{&insDest, "INSERT INTO destinataires(uid, rang, adresse) VALUES (?, ?, ?)"},
	} {
		st, err := tx.Prepare(r.sql)
		if err != nil {
			return err
		}
		defer st.Close()
		*r.st = st
	}

	for _, m := range msgs {
		hote, domaine := Domaines(m.De)
		var pj int64
		for _, p := range m.Pieces {
			if !p.Inline {
				pj += p.Taille
			}
		}
		premier := ""
		if len(m.Destinataires) > 0 {
			premier = m.Destinataires[0]
		}
		if _, err := insMsg.Exec(m.UID, int64(m.GmMsgID), int64(m.GmThrID), m.Taille, m.Date.Unix(), m.Lu,
			m.De, m.NomDe, hote, domaine, m.Objet, NormaliserObjet(m.Objet), pj, m.ObjetIllisible,
			Boite(m.Libelles), premier); err != nil {
			return err
		}
		if err := ecrireDestinataires(supDest, insDest, m.UID, m.Destinataires); err != nil {
			return err
		}
		if _, err := supLib.Exec(m.UID); err != nil {
			return err
		}
		for _, l := range m.Libelles {
			if _, err := insLib.Exec(m.UID, l); err != nil {
				return err
			}
		}
		if _, err := supPie.Exec(m.UID); err != nil {
			return err
		}
		for i, p := range m.Pieces {
			if _, err := insPie.Exec(m.UID, i, p.Type, p.Taille, p.Inline); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// ActualiserDrapeaux met a jour lu, boite et libelles des messages deja en
// cache (rattrapage CONDSTORE : seuls UID, Lu et Libelles sont lus). Les UID
// absents du cache sont ignores. Renvoie le nombre de messages mis a jour.
func (c *Cache) ActualiserDrapeaux(msgs []imap.Message) (int, error) {
	if len(msgs) == 0 {
		return 0, nil
	}
	tx, err := c.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var maj, supLib, insLib *sql.Stmt
	for _, r := range []struct {
		st  **sql.Stmt
		sql string
	}{
		{&maj, "UPDATE messages SET lu = ?, boite = ? WHERE uid = ?"},
		{&supLib, "DELETE FROM libelles WHERE uid = ?"},
		{&insLib, "INSERT OR IGNORE INTO libelles(uid, libelle) VALUES (?, ?)"},
	} {
		st, err := tx.Prepare(r.sql)
		if err != nil {
			return 0, err
		}
		defer st.Close()
		*r.st = st
	}
	n := 0
	for _, m := range msgs {
		r, err := maj.Exec(m.Lu, Boite(m.Libelles), m.UID)
		if err != nil {
			return 0, err
		}
		if k, err := r.RowsAffected(); err != nil {
			return 0, err
		} else if k == 0 {
			continue // pas encore en cache : sera lu en entier
		}
		n++
		if _, err := supLib.Exec(m.UID); err != nil {
			return 0, err
		}
		for _, l := range m.Libelles {
			if _, err := insLib.Exec(m.UID, l); err != nil {
				return 0, err
			}
		}
	}
	return n, tx.Commit()
}

// Boite classe un message d'apres ses libelles : brouillon
// (\Draft), envoye (\Sent, y compris envoye a soi-meme), sinon recu.
func Boite(libelles []string) string {
	envoye := false
	for _, l := range libelles {
		switch l {
		case `\Draft`:
			return "brouillon"
		case `\Sent`:
			envoye = true
		}
	}
	if envoye {
		return "envoye"
	}
	return "recu"
}

func ecrireDestinataires(sup, ins *sql.Stmt, uid uint32, adresses []string) error {
	if _, err := sup.Exec(uid); err != nil {
		return err
	}
	for i, a := range adresses {
		if _, err := ins.Exec(uid, i, a); err != nil {
			return err
		}
	}
	return nil
}

// UIDs renvoie l'ensemble des UID presents dans le cache.
func (c *Cache) UIDs() (map[uint32]bool, error) {
	rows, err := c.db.Query("SELECT uid FROM messages")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uint32]bool{}
	for rows.Next() {
		var u uint32
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out[u] = true
	}
	return out, rows.Err()
}

// Supprimer retire des messages du cache (supprimes cote serveur).
func (c *Cache) Supprimer(uids []uint32) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := tx.Prepare("DELETE FROM messages WHERE uid = ?")
	if err != nil {
		return err
	}
	defer st.Close()
	for _, u := range uids {
		if _, err := st.Exec(u); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Vider efface tous les messages (UIDVALIDITY changee : les UID ne designent
// plus les memes messages).
func (c *Cache) Vider() error {
	_, err := c.db.Exec("DELETE FROM messages")
	return err
}

// Categories connues (onglets de Gmail). "principale" est le complement : Gmail
// ne repond pas a category:primary en IMAP (constate sur une vraie boite).
var Categories = map[string]string{
	"reseaux":      "category:social",
	"promotions":   "category:promotions",
	"mises_a_jour": "category:updates",
	"forums":       "category:forums",
}

// DefinirCategories remet tous les messages en "principale" puis applique les
// categories trouvees (uid -> categorie).
func (c *Cache) DefinirCategories(cats map[uint32]string) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE messages SET categorie = 'principale'"); err != nil {
		return err
	}
	st, err := tx.Prepare("UPDATE messages SET categorie = ? WHERE uid = ?")
	if err != nil {
		return err
	}
	defer st.Close()
	for u, cat := range cats {
		if _, err := st.Exec(cat, u); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Resume : chiffres globaux du cache.
type Resume struct {
	Messages     int   `json:"messages"`
	Expediteurs  int   `json:"expediteurs"`
	Domaines     int   `json:"domaines"`
	Fils         int   `json:"fils"`
	Taille       int64 `json:"taille"`
	PremiereDate int64 `json:"premiereDate"` // secondes Unix
	DerniereDate int64 `json:"derniereDate"`
}

// Resumer calcule les chiffres globaux.
func (c *Cache) Resumer() (Resume, error) {
	var r Resume
	var premiere, derniere sql.NullInt64
	err := c.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(taille), 0), COUNT(DISTINCT expediteur), COUNT(DISTINCT domaine),
		COUNT(DISTINCT gm_thrid), MIN(date), MAX(date) FROM messages`).
		Scan(&r.Messages, &r.Taille, &r.Expediteurs, &r.Domaines, &r.Fils, &premiere, &derniere)
	r.PremiereDate, r.DerniereDate = premiere.Int64, derniere.Int64
	return r, err
}

// Domaines renvoie l'hote d'une adresse et son domaine enregistrable
// (news.marque.fr -> marque.fr), d'apres la liste des suffixes publics.
func Domaines(adresse string) (hote, domaine string) {
	_, h, ok := strings.Cut(adresse, "@")
	if !ok || h == "" {
		return "", ""
	}
	h = strings.ToLower(strings.TrimSuffix(h, "."))
	if d, err := publicsuffix.EffectiveTLDPlusOne(h); err == nil {
		return h, d
	}
	return h, h
}

var (
	rePrefixe = regexp.MustCompile(`(?i)^\s*((re|fw|fwd|tr|aw|wg|rv|réf|ref)\s*(\[\d+\])?\s*:\s*)+`)
	reChiffre = regexp.MustCompile(`\d+`)
	reEspaces = regexp.MustCompile(`\s+`)
)

// NormaliserObjet : niveau 1 du regroupement par titre :
// prefixes de reponse et transfert retires, chiffres remplaces par #,
// minuscules, espaces reduits.
func NormaliserObjet(s string) string {
	s = strings.ToLower(rePrefixe.ReplaceAllString(s, ""))
	s = reChiffre.ReplaceAllString(s, "#")
	return strings.TrimSpace(reEspaces.ReplaceAllString(s, " "))
}
