package cache

// Les libelles personnels peuvent se recouvrir. Chaque bloc compte ses
// messages une seule fois, y compris « Autres », qui est leur union.

import (
	"slices"
	"strings"
)

const libellePersonnel = "substr(l.libelle, 1, 1) <> '\\'"

// filtreLibelles utilise EXISTS pour ne jamais multiplier les messages de
// la carte, du detail, de la liste ou du Purgatoire.
func filtreLibelles(f Filtre) (string, []any) {
	base := "SELECT 1 FROM libelles l WHERE l.uid = m.uid AND " + libellePersonnel
	sans := "NOT EXISTS (" + base + ")"
	if len(f.Exclure) == 0 {
		if f.Valeur == "" {
			return sans, nil
		}
		return "EXISTS (" + base + " AND l.libelle = ?)", []any{f.Valeur}
	}
	params := make([]any, len(f.Exclure))
	for i, v := range f.Exclure {
		params[i] = v
	}
	cond := "EXISTS (" + base + " AND l.libelle NOT IN (" + strings.TrimSuffix(strings.Repeat("?,", len(params)), ",") + "))"
	// Un message peut porter un libelle affiche ET un libelle d'« Autres ».
	// Sans libelle personnel, il appartient a « Autres » si le groupe sans
	// libelle n'est pas affiche separement.
	if !slices.Contains(f.Exclure, "") {
		cond = "(" + cond + " OR " + sans + ")"
	}
	return cond, params
}

// regrouperLibelles applique le seuil du front (au moins 12 blocs visibles),
// puis recompte « Autres » en SQL : additionner les blocs ferait des doublons.
func (c *Cache) regrouperLibelles(r RequeteCarte, tous []Noeud) ([]Noeud, error) {
	valeur := func(n Noeud) float64 {
		if r.Metrique == "nombre" {
			return float64(n.Nombre)
		}
		return float64(n.Taille)
	}
	total := 0.0
	for _, n := range tous {
		total += valeur(n)
	}
	garder := 0
	for garder < len(tous) && garder < maxBlocs {
		if garder >= 12 && total > 0 && valeur(tous[garder])/total < r.PartMin {
			break
		}
		garder++
	}
	if garder == len(tous) {
		return tous, nil
	}
	autres := Noeud{Cle: "\x00autres", Autres: len(tous) - garder, Exclure: make([]string, garder)}
	for i, n := range tous[:garder] {
		autres.Exclure[i] = n.Cle
	}
	chemin := append(slices.Clone(r.Chemin), Filtre{Axe: "libelle", Exclure: autres.Exclure})
	ou, params := clause(r.Boite, r.Lecture, chemin)
	if err := c.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(m.taille), 0) FROM messages m"+ou, params...).Scan(&autres.Nombre, &autres.Taille); err != nil {
		return nil, err
	}
	return append(tous[:garder:garder], autres), nil
}
