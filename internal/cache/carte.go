package cache

// Requetes du treemap : un niveau a la fois (le zoom recharge le niveau
// suivant), agrege par SQL sur la table messages.
//
// Les axes sont une liste blanche : aucune saisie du front n'entre telle quelle
// dans le SQL, seules les valeurs passent en parametres.

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Axes de regroupement disponibles et leur expression SQL.
var axes = map[string]string{
	"domaine":    "m.domaine",
	"expediteur": "m.expediteur",
	"titre":      "m.objet_norm",
	"libelle":    "COALESCE(l.libelle, '')",
	// Annee et mois en heure locale, comme les dates affichees (un message du
	// 1er janvier a 0 h 30 a Paris est de la nouvelle annee).
	"annee":        "strftime('%Y', m.date, 'unixepoch', 'localtime')",
	"mois":         "strftime('%Y-%m', m.date, 'unixepoch', 'localtime')",
	"categorie":    "m.categorie",
	"boite":        "m.boite",
	"destinataire": "m.destinataire",
	"tranche":      "CASE WHEN m.taille < 100000 THEN '1' WHEN m.taille < 1000000 THEN '2' WHEN m.taille < 10000000 THEN '3' ELSE '4' END",
}

// AxeValide indique si nom est un axe de regroupement connu.
func AxeValide(nom string) bool { _, ok := axes[nom]; return ok }

// Filtre : une etape du chemin de zoom. Exclure non vide designe le bloc
// « Autres » d'un niveau : toutes les valeurs sauf celles affichees.
type Filtre struct {
	Axe     string   `json:"axe"`
	Valeur  string   `json:"valeur"`
	Exclure []string `json:"exclure"`
}

// RequeteCarte : pile d'axes, chemin de zoom et metrique ("taille" ou
// "nombre"). Quand le chemin est aussi long que la pile, le niveau demande est
// celui des messages.
type RequeteCarte struct {
	Axes     []string `json:"axes"`
	Chemin   []Filtre `json:"chemin"`
	Metrique string   `json:"metrique"`
	Boite    string   `json:"boite"`   // "" : tous ; "recu", "envoye" ou "brouillon"
	Lecture  string   `json:"lecture"` // "" : tous ; "nonlu" ou "lu"
	PartMin  float64  `json:"partMin"` // part minimale d'un bloc visible ; 0 : pas de seuil
}

// Boites : valeurs du filtre global par boite.
var Boites = map[string]bool{"": true, "recu": true, "envoye": true, "brouillon": true}

// Lectures : valeurs du filtre global par etat de lecture.
var Lectures = map[string]bool{"": true, "nonlu": true, "lu": true}

// portee verifie les filtres globaux (boite, lecture) et les axes du chemin.
func portee(boite, lecture string, chemin []Filtre) error {
	if !Boites[boite] {
		return fmt.Errorf("boite inconnue : %q", boite)
	}
	if !Lectures[lecture] {
		return fmt.Errorf("lecture inconnue : %q", lecture)
	}
	for _, f := range chemin {
		if !AxeValide(f.Axe) {
			return fmt.Errorf("axe inconnu : %q", f.Axe)
		}
	}
	return nil
}

// Noeud : un bloc du treemap.
type Noeud struct {
	Cle     string   `json:"cle"`
	Libelle string   `json:"libelle"`
	Domaine string   `json:"domaine"` // pour la couleur ; vide si inconnu
	Nombre  int      `json:"nombre"`
	Taille  int64    `json:"taille"`
	Autres  int      `json:"autres"`  // > 0 : bloc « Autres » regroupant autant de valeurs
	Exclure []string `json:"exclure"` // valeurs affichees a cote du bloc « Autres »
}

// Niveau : les blocs d'un niveau et ses totaux.
type Niveau struct {
	Axe    string  `json:"axe"` // axe de ce niveau ; "message" : totaux seuls, les messages viennent de la liste
	Noeuds []Noeud `json:"noeuds"`
	Nombre int     `json:"nombre"`
	Taille int64   `json:"taille"`
}

// maxBlocs : valeurs renvoyees au plus par niveau, triees par la metrique ;
// au-dela, le reste forme un bloc « Autres ». Le front regroupe ensuite les
// blocs trop petits pour etre vus, d'apres la surface reelle de la carte.
// Variable pour les tests.
var maxBlocs = 3000

func (r RequeteCarte) valider() error {
	if len(r.Axes) == 0 {
		return errors.New("pile d'axes vide")
	}
	for _, a := range r.Axes {
		if !AxeValide(a) {
			return fmt.Errorf("axe inconnu : %q", a)
		}
	}
	if err := portee(r.Boite, r.Lecture, r.Chemin); err != nil {
		return err
	}
	if len(r.Chemin) > len(r.Axes) {
		return errors.New("chemin plus long que la pile d'axes")
	}
	for i, f := range r.Chemin {
		if f.Axe != r.Axes[i] {
			return fmt.Errorf("chemin incoherent a l'etape %d", i)
		}
	}
	return nil
}

// clause construit le WHERE des filtres globaux (boite, lecture) et du chemin
// (alias m) et ses parametres. Les messages du Purgatoire en sont toujours
// exclus : la carte et la liste montrent ce qui reste.
func clause(boite, lecture string, chemin []Filtre) (string, []any) {
	return clauseDe(horsPurgatoire, boite, lecture, chemin)
}

// clauseDe : comme clause, a partir d'une condition de depart (hors du
// Purgatoire pour la carte, dans le Purgatoire pour sa revue).
func clauseDe(depart, boite, lecture string, chemin []Filtre) (string, []any) {
	conds := []string{depart}
	var params []any
	if boite != "" {
		conds = append(conds, "m.boite = ?")
		params = append(params, boite)
	}
	switch lecture {
	case "nonlu":
		conds = append(conds, "m.lu = 0")
	case "lu":
		conds = append(conds, "m.lu = 1")
	}
	for _, f := range chemin {
		if f.Axe == "libelle" {
			cond, p := filtreLibelles(f)
			conds = append(conds, cond)
			params = append(params, p...)
			continue
		}
		expr := axes[f.Axe]
		if len(f.Exclure) > 0 {
			conds = append(conds, expr+" NOT IN ("+strings.TrimSuffix(strings.Repeat("?,", len(f.Exclure)), ",")+")")
			for _, v := range f.Exclure {
				params = append(params, v)
			}
			continue
		}
		conds = append(conds, expr+" = ?")
		params = append(params, f.Valeur)
	}
	return " WHERE " + strings.Join(conds, " AND "), params
}

func ordre(metrique string) string {
	if metrique == "nombre" {
		return "nombre DESC, taille DESC"
	}
	return "taille DESC, nombre DESC"
}

// Carte renvoie le niveau designe par la requete.
func (c *Cache) Carte(r RequeteCarte) (Niveau, error) {
	if err := r.valider(); err != nil {
		return Niveau{}, err
	}
	ou, params := clause(r.Boite, r.Lecture, r.Chemin)
	var n Niveau
	if err := c.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(taille), 0) FROM messages m"+ou, params...).Scan(&n.Nombre, &n.Taille); err != nil {
		return n, err
	}
	domaineChemin := ""
	for _, f := range r.Chemin {
		if f.Axe == "domaine" && len(f.Exclure) == 0 {
			domaineChemin = f.Valeur
		}
	}
	if len(r.Chemin) == len(r.Axes) {
		n.Axe = "message"
		return n, nil
	}
	axe := r.Axes[len(r.Chemin)]
	n.Axe = axe
	expr := axes[axe]
	jointure := ""
	if axe == "libelle" {
		jointure = " LEFT JOIN libelles l ON l.uid = m.uid AND " + libellePersonnel
	}
	rows, err := c.db.Query("SELECT "+expr+" AS cle, COUNT(*) AS nombre, COALESCE(SUM(m.taille), 0) AS taille, MAX(m.nom_expediteur) "+
		"FROM messages m"+jointure+ou+" GROUP BY cle ORDER BY "+ordre(r.Metrique)+", cle", params...)
	if err != nil {
		return n, err
	}
	defer rows.Close()
	var tous []Noeud
	for rows.Next() {
		var x Noeud
		var nom string
		if err := rows.Scan(&x.Cle, &x.Nombre, &x.Taille, &nom); err != nil {
			return n, err
		}
		x.Libelle = libelle(axe, x.Cle, nom)
		switch axe {
		case "domaine":
			x.Domaine = x.Cle
		case "expediteur":
			_, x.Domaine = Domaines(x.Cle)
		default:
			x.Domaine = domaineChemin
		}
		tous = append(tous, x)
	}
	if err := rows.Err(); err != nil {
		return n, err
	}
	if axe == "libelle" {
		// Liberer la connexion avant de compter les messages du bloc « Autres ».
		rows.Close()
		n.Noeuds, err = c.regrouperLibelles(r, tous)
		return n, err
	}
	n.Noeuds = regrouper(tous)
	return n, nil
}

func libelle(axe, cle, nom string) string {
	switch {
	case cle == "" && axe == "libelle":
		return "(sans libellé personnel)"
	case cle == "" && axe == "titre":
		return "(sans objet)"
	case cle == "":
		return "(inconnu)"
	case axe == "expediteur" && nom != "":
		return nom + " <" + cle + ">"
	}
	return cle
}

// regrouper garde les maxBlocs plus gros blocs (les noeuds arrivent tries
// par la metrique, decroissante) et rassemble les suivants en un bloc
// « Autres ». Sa liste d'exclusion est laissee au front, qui regroupe aussi
// les petits blocs et connait donc les valeurs reellement affichees.
func regrouper(tous []Noeud) []Noeud {
	if len(tous) <= maxBlocs {
		return tous
	}
	autres := Noeud{Cle: "\x00autres", Autres: len(tous) - maxBlocs}
	for _, x := range tous[maxBlocs:] {
		autres.Nombre += x.Nombre
		autres.Taille += x.Taille
	}
	return append(tous[:maxBlocs:maxBlocs], autres)
}

// RequeteDetail : le bloc dont on veut le detail (chemin jusqu'au bloc), ou un
// message (UID > 0), sous le filtre de boite.
type RequeteDetail struct {
	Chemin   []Filtre `json:"chemin"`
	UID      uint32   `json:"uid"`
	Metrique string   `json:"metrique"`
	Boite    string   `json:"boite"`
	Lecture  string   `json:"lecture"`
}

// Detail : chiffres d'un bloc pour le panneau lateral.
type Detail struct {
	Nombre        int            `json:"nombre"`
	Taille        int64          `json:"taille"`
	Premiere      int64          `json:"premiere"`
	Derniere      int64          `json:"derniere"`
	NonLus        int            `json:"nonLus"`
	ParAnnee      []Repartition  `json:"parAnnee"`
	Expediteurs   []Repartition  `json:"expediteurs"`
	Destinataires []Repartition  `json:"destinataires"`
	Pieces        []Repartition  `json:"pieces"`
	Categories    []Repartition  `json:"categories"`
	Message       *DetailMessage `json:"message"`
}

// Repartition : une ligne de ventilation (annee, expediteur, type de piece jointe...).
type Repartition struct {
	Cle     string `json:"cle"`
	Libelle string `json:"libelle"`
	Nombre  int    `json:"nombre"`
	Taille  int64  `json:"taille"`
}

// DetailMessage : en-tetes d'un message (jamais son corps).
type DetailMessage struct {
	Objet      string   `json:"objet"`
	Expediteur string   `json:"expediteur"`
	Nom        string   `json:"nom"`
	Date       int64    `json:"date"`
	Taille     int64    `json:"taille"`
	Lu         bool     `json:"lu"`
	Categorie  string   `json:"categorie"`
	Libelles   []string `json:"libelles"`
	GmMsgID    string   `json:"gmMsgId"`
	Boite      string   `json:"boite"`
	// Destinataires : To puis Cc.
	Destinataires []string `json:"destinataires"`
}

// DetailCarte calcule le detail du bloc designe par chemin (le dernier filtre
// est le bloc lui-meme) ; uid > 0 designe un message.
func (c *Cache) DetailCarte(q RequeteDetail) (Detail, error) {
	var d Detail
	chemin, uid, metrique := q.Chemin, q.UID, q.Metrique
	if err := portee(q.Boite, q.Lecture, chemin); err != nil {
		return d, err
	}
	ou, params := clause(q.Boite, q.Lecture, chemin)
	if uid > 0 {
		if ou == "" {
			ou = " WHERE m.uid = ?"
		} else {
			ou += " AND m.uid = ?"
		}
		params = append(params, uid)
	}
	if err := c.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(taille), 0), COALESCE(MIN(date), 0), COALESCE(MAX(date), 0), COALESCE(SUM(lu = 0), 0) FROM messages m"+ou, params...).
		Scan(&d.Nombre, &d.Taille, &d.Premiere, &d.Derniere, &d.NonLus); err != nil {
		return d, err
	}
	var err error
	if d.ParAnnee, err = c.repartition("SELECT strftime('%Y', m.date, 'unixepoch', 'localtime') AS cle, '', COUNT(*), COALESCE(SUM(m.taille), 0) FROM messages m"+ou+" GROUP BY cle ORDER BY cle", params); err != nil {
		return d, err
	}
	if d.Expediteurs, err = c.repartition("SELECT m.expediteur AS cle, MAX(m.nom_expediteur), COUNT(*) AS nombre, COALESCE(SUM(m.taille), 0) AS taille FROM messages m"+ou+" GROUP BY cle ORDER BY "+ordre(metrique)+" LIMIT 5", params); err != nil {
		return d, err
	}
	if d.Destinataires, err = c.repartition("SELECT m.destinataire AS cle, '', COUNT(*) AS nombre, COALESCE(SUM(m.taille), 0) AS taille FROM messages m"+ou+
		et(ou, "m.destinataire <> ''")+" GROUP BY cle ORDER BY "+ordre(metrique)+" LIMIT 5", params); err != nil {
		return d, err
	}
	if d.Pieces, err = c.repartition("SELECT p.type AS cle, '', COUNT(DISTINCT p.uid), COALESCE(SUM(p.taille), 0) AS taille FROM pieces p JOIN messages m ON m.uid = p.uid"+
		strings.Replace(ou, " WHERE ", " WHERE p.inline = 0 AND ", 1)+condInline(ou)+" GROUP BY cle ORDER BY taille DESC", params); err != nil {
		return d, err
	}
	if d.Categories, err = c.repartition("SELECT m.categorie AS cle, '', COUNT(*) AS nombre, COALESCE(SUM(m.taille), 0) AS taille FROM messages m"+ou+" GROUP BY cle ORDER BY "+ordre(metrique), params); err != nil {
		return d, err
	}
	if uid > 0 && d.Nombre == 1 {
		dm := &DetailMessage{}
		var msgid int64
		if err := c.db.QueryRow("SELECT objet, expediteur, nom_expediteur, date, taille, lu, categorie, gm_msgid, boite FROM messages WHERE uid = ?", uid).
			Scan(&dm.Objet, &dm.Expediteur, &dm.Nom, &dm.Date, &dm.Taille, &dm.Lu, &dm.Categorie, &msgid, &dm.Boite); err != nil {
			return d, err
		}
		dm.GmMsgID = strconv.FormatUint(uint64(msgid), 16)
		if dm.Libelles, err = c.textes("SELECT libelle FROM libelles WHERE uid = ? ORDER BY libelle", uid); err != nil {
			return d, err
		}
		if dm.Destinataires, err = c.textes("SELECT adresse FROM destinataires WHERE uid = ? ORDER BY rang", uid); err != nil {
			return d, err
		}
		d.Message = dm
	}
	return d, nil
}

// textes renvoie la premiere colonne (texte) de chaque ligne d'une requete.
func (c *Cache) textes(requete string, params ...any) ([]string, error) {
	rows, err := c.db.Query(requete, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// et ajoute une condition a une clause WHERE eventuellement vide.
func et(ou, cond string) string {
	if ou == "" {
		return " WHERE " + cond
	}
	return " AND " + cond
}

// condInline : sans chemin (pas de WHERE), la condition inline = 0 s'ajoute ici.
func condInline(ou string) string {
	if ou == "" {
		return " WHERE p.inline = 0"
	}
	return ""
}

func (c *Cache) repartition(requete string, params []any) ([]Repartition, error) {
	rows, err := c.db.Query(requete, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Repartition{}
	for rows.Next() {
		var r Repartition
		if err := rows.Scan(&r.Cle, &r.Libelle, &r.Nombre, &r.Taille); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
