package cache

// Purgatoire : messages mis de cote pour la corbeille. Tant qu'ils y sont, ils
// disparaissent de la carte et de la liste (voir clause) ; rien ne part chez
// Gmail avant la mise a la corbeille, decidee depuis la revue du Purgatoire
// (synchro.MettreALaCorbeille). Table purg.purgatoire, dans une base a part
// rattachee au cache (voir OuvrirChemins).

import (
	"database/sql"
	"strings"
	"time"
)

// horsPurgatoire : condition (alias m) qui ecarte les messages du Purgatoire.
const horsPurgatoire = "m.gm_msgid NOT IN (SELECT gm_msgid FROM purg.purgatoire)"

// dansPurgatoire : l'inverse, pour la revue du Purgatoire.
const dansPurgatoire = "m.gm_msgid IN (SELECT gm_msgid FROM purg.purgatoire)"

// RequeteBloc : un bloc entier de la carte (chemin jusqu'au bloc), sous les
// filtres globaux. Nombre et Taille : ce que la carte affichait pour ce bloc
// (0 : pas de controle). Le cache a pu changer depuis (synchro en
// arriere-plan) : un bloc qui ne correspond plus n'est pas envoye.
type RequeteBloc struct {
	Chemin  []Filtre `json:"chemin"`
	Boite   string   `json:"boite"`
	Lecture string   `json:"lecture"`
	Nombre  int      `json:"nombre"`
	Taille  int64    `json:"taille"`
}

// Lot : messages envoyes ensemble au Purgatoire (pour l'annulation). Ecart :
// le bloc ne correspondait plus a l'affichage, rien n'a ete envoye.
type Lot struct {
	UIDs   []uint32 `json:"uids"`
	Nombre int      `json:"nombre"`
	Taille int64    `json:"taille"`
	Ecart  bool     `json:"ecart"`
}

// EtatPurgatoire : contenu du Purgatoire et totaux de la boite (jauges et
// bilan de la revue).
type EtatPurgatoire struct {
	Nombre      int   `json:"nombre"`
	Taille      int64 `json:"taille"`
	PJ          int64 `json:"pj"`     // dont pieces jointes
	NonLus      int   `json:"nonLus"` // jamais lus
	TotalNombre int   `json:"totalNombre"`
	TotalTaille int64 `json:"totalTaille"`
}

// lotSQL : parametres par requete au plus (limite de SQLite : 32766).
const lotSQL = 500

// EnvoyerPurgatoire met les messages designes au Purgatoire. Les UID inconnus
// sont ignores, comme un message sans X-GM-MSGID (0) : il ne pourrait pas etre
// retrouve sans ambiguite chez Gmail. Le lot renvoye ne compte que les
// messages retenus.
func (c *Cache) EnvoyerPurgatoire(uids []uint32) (Lot, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return Lot{UIDs: []uint32{}}, err
	}
	defer tx.Rollback()
	lot, err := envoyer(tx, uids)
	if err != nil {
		return lot, err
	}
	return lot, tx.Commit()
}

// envoyer : EnvoyerPurgatoire dans la transaction tx.
func envoyer(tx *sql.Tx, uids []uint32) (Lot, error) {
	lot := Lot{UIDs: []uint32{}}
	maintenant := time.Now().Unix()
	for debut := 0; debut < len(uids); debut += lotSQL {
		part := uids[debut:min(debut+lotSQL, len(uids))]
		marques, params := enParametres(part)
		if err := ajouterLot(&lot, tx, "SELECT m.uid, m.taille FROM messages m WHERE m.uid IN ("+marques+") AND m.gm_msgid <> 0 AND "+horsPurgatoire, params...); err != nil {
			return lot, err
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO purg.purgatoire(gm_msgid, ajoute) SELECT gm_msgid, ? FROM messages WHERE uid IN ("+marques+") AND gm_msgid <> 0",
			append([]any{maintenant}, params...)...); err != nil {
			return lot, err
		}
	}
	lot.Nombre = len(lot.UIDs)
	return lot, nil
}

// EnvoyerBlocPurgatoire met au Purgatoire tous les messages d'un bloc. La
// lecture du bloc, sa comparaison a l'affichage et l'envoi forment une seule
// transaction (verrou d'ecriture pris des le debut) : la synchro ne peut rien
// y ajouter entre les trois.
func (c *Cache) EnvoyerBlocPurgatoire(r RequeteBloc) (Lot, error) {
	if err := portee(r.Boite, r.Lecture, r.Chemin); err != nil {
		return Lot{}, err
	}
	ou, params := clause(r.Boite, r.Lecture, r.Chemin)
	tx, err := c.db.Begin()
	if err != nil {
		return Lot{}, err
	}
	defer tx.Rollback()
	var bloc Lot
	if err := ajouterLot(&bloc, tx, "SELECT m.uid, m.taille FROM messages m"+ou, params...); err != nil {
		return Lot{}, err
	}
	if r.Nombre > 0 && (len(bloc.UIDs) != r.Nombre || bloc.Taille != r.Taille) {
		return Lot{UIDs: []uint32{}, Ecart: true}, nil
	}
	lot, err := envoyer(tx, bloc.UIDs)
	if err != nil {
		return lot, err
	}
	return lot, tx.Commit()
}

// RetirerPurgatoire sort les messages designes du Purgatoire (annulation d'un
// envoi, ou retrait depuis la revue). Le lot renvoye ne compte que les
// messages qui y etaient : il sert a annuler le retrait.
func (c *Cache) RetirerPurgatoire(uids []uint32) (Lot, error) {
	lot := Lot{UIDs: []uint32{}}
	tx, err := c.db.Begin()
	if err != nil {
		return lot, err
	}
	defer tx.Rollback()
	for debut := 0; debut < len(uids); debut += lotSQL {
		marques, params := enParametres(uids[debut:min(debut+lotSQL, len(uids))])
		if err := ajouterLot(&lot, tx, "SELECT m.uid, m.taille FROM messages m WHERE m.uid IN ("+marques+") AND "+dansPurgatoire, params...); err != nil {
			return lot, err
		}
		if _, err := tx.Exec("DELETE FROM purg.purgatoire WHERE gm_msgid IN (SELECT gm_msgid FROM messages WHERE uid IN ("+marques+"))", params...); err != nil {
			return lot, err
		}
	}
	lot.Nombre = len(lot.UIDs)
	return lot, tx.Commit()
}

// ViderPurgatoire retire tout le Purgatoire (rien ne part chez Gmail).
func (c *Cache) ViderPurgatoire() (Lot, error) {
	lot := Lot{UIDs: []uint32{}}
	tx, err := c.db.Begin()
	if err != nil {
		return lot, err
	}
	defer tx.Rollback()
	if err := ajouterLot(&lot, tx, "SELECT m.uid, m.taille FROM messages m WHERE "+dansPurgatoire); err != nil {
		return lot, err
	}
	if _, err := tx.Exec("DELETE FROM purg.purgatoire"); err != nil {
		return lot, err
	}
	lot.Nombre = len(lot.UIDs)
	return lot, tx.Commit()
}

// ajouterLot ajoute au lot les messages (uid, taille) d'une requete.
func ajouterLot(lot *Lot, tx *sql.Tx, requete string, params ...any) error {
	rows, err := tx.Query(requete, params...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var u uint32
		var t int64
		if err := rows.Scan(&u, &t); err != nil {
			return err
		}
		lot.UIDs = append(lot.UIDs, u)
		lot.Taille += t
	}
	return rows.Err()
}

// EtatDuPurgatoire renvoie le contenu du Purgatoire (messages encore dans la
// boite) et les totaux de la boite.
func (c *Cache) EtatDuPurgatoire() (EtatPurgatoire, error) {
	var e EtatPurgatoire
	err := c.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(m.taille), 0),
		COALESCE(SUM(p.gm_msgid IS NOT NULL), 0),
		COALESCE(SUM(CASE WHEN p.gm_msgid IS NOT NULL THEN m.taille ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN p.gm_msgid IS NOT NULL THEN m.pj_taille ELSE 0 END), 0),
		COALESCE(SUM(p.gm_msgid IS NOT NULL AND m.lu = 0), 0)
		FROM messages m LEFT JOIN purg.purgatoire p ON p.gm_msgid = m.gm_msgid`).Scan(&e.TotalNombre, &e.TotalTaille, &e.Nombre, &e.Taille, &e.PJ, &e.NonLus)
	return e, err
}

// NettoyerPurgatoire retire du Purgatoire les messages qui ne sont plus dans
// la boite (supprimes depuis dans Gmail). A n'appeler qu'apres une synchro
// complete : apres un vidage du cache, les messages pas encore relus
// passeraient pour disparus.
func (c *Cache) NettoyerPurgatoire() (int64, error) {
	r, err := c.db.Exec("DELETE FROM purg.purgatoire WHERE gm_msgid NOT IN (SELECT gm_msgid FROM messages)")
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

func enParametres(uids []uint32) (string, []any) {
	params := make([]any, len(uids))
	for i, u := range uids {
		params[i] = u
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(uids)), ","), params
}

// Cible : un message du Purgatoire a mettre a la corbeille. L'UID n'est qu'un
// candidat, a verifier dans la session IMAP par son X-GM-MSGID.
type Cible struct {
	UID     uint32
	GmMsgID uint64
	Taille  int64
}

// CiblesCorbeille : les messages du Purgatoire presents dans le cache, soit
// exactement ce que la revue affiche (et EtatDuPurgatoire compte).
func (c *Cache) CiblesCorbeille() ([]Cible, error) {
	rows, err := c.db.Query("SELECT m.uid, m.gm_msgid, m.taille FROM messages m WHERE m.gm_msgid <> 0 AND " + dansPurgatoire + " ORDER BY m.uid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cible
	for rows.Next() {
		var x Cible
		if err := rows.Scan(&x.UID, &x.GmMsgID, &x.Taille); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// SortirDeLaBoite retire du cache et du Purgatoire les messages designes par
// leur X-GM-MSGID : ils ont ete mis a la corbeille de Gmail (ou n'etaient
// deja plus dans « Tous les messages »). Une seule transaction.
func (c *Cache) SortirDeLaBoite(gms []uint64) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for debut := 0; debut < len(gms); debut += lotSQL {
		part := gms[debut:min(debut+lotSQL, len(gms))]
		params := make([]any, len(part))
		for i, g := range part {
			params[i] = int64(g)
		}
		marques := strings.TrimSuffix(strings.Repeat("?,", len(part)), ",")
		if _, err := tx.Exec("DELETE FROM purg.purgatoire WHERE gm_msgid IN ("+marques+")", params...); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM messages WHERE gm_msgid IN ("+marques+")", params...); err != nil {
			return err
		}
	}
	return tx.Commit()
}
