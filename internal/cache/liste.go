package cache

// Liste des messages du dernier niveau. Le front recoit d'abord l'index de la
// liste (UID et taille de chaque message, dans l'ordre affiche) : de quoi
// dimensionner le defilement, selectionner une plage ou tout, et totaliser une
// selection sans tout charger. Il demande ensuite le contenu des lignes par
// lots, au fil du defilement.

import (
	"fmt"
	"strconv"
	"strings"
)

// RequeteListe : messages du chemin sous les filtres globaux, et leur ordre.
type RequeteListe struct {
	Chemin  []Filtre `json:"chemin"`
	Boite   string   `json:"boite"`
	Lecture string   `json:"lecture"`
	Tri     string   `json:"tri"` // une cle de tris
	Desc    bool     `json:"desc"`
	// Purgatoire : tous les messages du Purgatoire (revue) au lieu de ceux de
	// la carte. Boite et lecture n'y filtrent pas ; le chemin doit etre vide.
	Purgatoire bool `json:"purgatoire"`
}

// tris : colonnes de tri de la liste (liste blanche, comme les axes).
var tris = map[string]string{
	"taille":       "m.taille",
	"date":         "m.date",
	"pj":           "m.pj_taille",
	"expediteur":   "CASE WHEN m.nom_expediteur = '' THEN m.expediteur ELSE m.nom_expediteur END COLLATE NOCASE",
	"destinataire": "m.destinataire COLLATE NOCASE",
}

// IndexListe : UID et taille de chaque message, dans l'ordre de la liste.
type IndexListe struct {
	UIDs    []uint32 `json:"uids"`
	Tailles []int64  `json:"tailles"`
}

// LigneMessage : ce qu'affiche une ligne de la liste.
type LigneMessage struct {
	UID          uint32   `json:"uid"`
	GmMsgID      string   `json:"gmMsgId"` // hexadecimal, pour ouvrir le message dans Gmail
	Objet        string   `json:"objet"`
	Expediteur   string   `json:"expediteur"`
	Nom          string   `json:"nom"`
	Destinataire string   `json:"destinataire"`
	Date         int64    `json:"date"`
	Taille       int64    `json:"taille"`
	PJ           int64    `json:"pj"` // taille des vraies pieces jointes
	Lu           bool     `json:"lu"`
	Boite        string   `json:"boite"`
	Libelles     []string `json:"libelles"` // libelles de l'utilisateur (sans les libelles systeme)
}

// MaxLignes : lignes demandees au plus par lot.
const MaxLignes = 500

// IndexListe renvoie l'ordre de la liste. A cle de tri egale, les plus recents
// d'abord.
func (c *Cache) IndexListe(r RequeteListe) (IndexListe, error) {
	idx := IndexListe{UIDs: []uint32{}, Tailles: []int64{}}
	if err := portee(r.Boite, r.Lecture, r.Chemin); err != nil {
		return idx, err
	}
	expr, ok := tris[r.Tri]
	if !ok {
		return idx, fmt.Errorf("tri inconnu : %q", r.Tri)
	}
	sens := " ASC"
	if r.Desc {
		sens = " DESC"
	}
	ou, params := clause(r.Boite, r.Lecture, r.Chemin)
	if r.Purgatoire {
		if len(r.Chemin) > 0 {
			return idx, fmt.Errorf("revue du Purgatoire : pas de chemin")
		}
		ou, params = clauseDe(dansPurgatoire, "", "", nil)
	}
	rows, err := c.db.Query("SELECT m.uid, m.taille FROM messages m"+ou+" ORDER BY "+expr+sens+", m.date DESC, m.uid DESC", params...)
	if err != nil {
		return idx, err
	}
	defer rows.Close()
	for rows.Next() {
		var uid uint32
		var taille int64
		if err := rows.Scan(&uid, &taille); err != nil {
			return idx, err
		}
		idx.UIDs = append(idx.UIDs, uid)
		idx.Tailles = append(idx.Tailles, taille)
	}
	return idx, rows.Err()
}

// Lignes renvoie le contenu des messages demandes, dans l'ordre demande. Un
// UID disparu du cache (synchro entre-temps) est simplement absent.
func (c *Cache) Lignes(uids []uint32) ([]LigneMessage, error) {
	out := []LigneMessage{}
	if len(uids) == 0 {
		return out, nil
	}
	if len(uids) > MaxLignes {
		return out, fmt.Errorf("trop de lignes demandees : %d (maximum %d)", len(uids), MaxLignes)
	}
	marques := strings.TrimSuffix(strings.Repeat("?,", len(uids)), ",")
	params := make([]any, len(uids))
	for i, u := range uids {
		params[i] = u
	}
	rows, err := c.db.Query("SELECT uid, gm_msgid, objet, expediteur, nom_expediteur, destinataire, date, taille, pj_taille, lu, boite FROM messages WHERE uid IN ("+marques+")", params...)
	if err != nil {
		return out, err
	}
	parUID := make(map[uint32]*LigneMessage, len(uids))
	for rows.Next() {
		l := &LigneMessage{Libelles: []string{}}
		var msgid int64
		if err := rows.Scan(&l.UID, &msgid, &l.Objet, &l.Expediteur, &l.Nom, &l.Destinataire, &l.Date, &l.Taille, &l.PJ, &l.Lu, &l.Boite); err != nil {
			rows.Close()
			return out, err
		}
		l.GmMsgID = strconv.FormatUint(uint64(msgid), 16)
		parUID[l.UID] = l
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	// Libelles systeme (\Inbox, \Sent, \Important...) exclus : ils sont deja
	// rendus par la boite et n'aident pas a trier.
	rows, err = c.db.Query("SELECT uid, libelle FROM libelles WHERE uid IN ("+marques+") AND substr(libelle, 1, 1) <> '\\' ORDER BY libelle", params...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var uid uint32
		var lib string
		if err := rows.Scan(&uid, &lib); err != nil {
			rows.Close()
			return out, err
		}
		if l := parUID[uid]; l != nil {
			l.Libelles = append(l.Libelles, lib)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	for _, u := range uids {
		if l := parUID[u]; l != nil {
			out = append(out, *l)
		}
	}
	return out, nil
}
