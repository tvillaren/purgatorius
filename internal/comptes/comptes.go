// Package comptes tient le registre des comptes Gmail (travail/comptes.json) :
// nom affiche, adresse, couleur, mode de connexion, etat de la synchro, compte
// actif et langue. Aucun secret ici : les mots de passe d'application sont dans
// le coffre du systeme (package secrets).
package comptes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/louis49/purgatorius/internal/store"
)

const fichier = "travail/comptes.json"

// ModeMotDePasseApp : connexion IMAP par mot de passe d'application (seul mode
// disponible ; OAuth viendra plus tard).
const ModeMotDePasseApp = "mot_de_passe_app"

// Compte : un compte Gmail connu de l'application.
type Compte struct {
	ID              string     `json:"id"`
	Nom             string     `json:"nom"`
	Adresse         string     `json:"adresse"`
	Couleur         string     `json:"couleur"`
	Mode            string     `json:"mode"`
	CreeLe          time.Time  `json:"cree_le"`
	SynchroComplete bool       `json:"synchro_complete"`
	DerniereSynchro *time.Time `json:"derniere_synchro,omitempty"`
}

// Registre : contenu de travail/comptes.json.
type Registre struct {
	Comptes []Compte `json:"comptes"`
	Actif   string   `json:"actif,omitempty"`
	Langue  string   `json:"langue,omitempty"`
}

// Compte renvoie le compte d'identifiant id.
func (r *Registre) Compte(id string) (*Compte, bool) {
	for i := range r.Comptes {
		if r.Comptes[i].ID == id {
			return &r.Comptes[i], true
		}
	}
	return nil, false
}

var mu sync.Mutex

// NormaliserAdresse : adresse sans espaces, en minuscules.
func NormaliserAdresse(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ID : identifiant d'un compte, 12 premiers caracteres hexadecimaux du SHA-256
// de l'adresse normalisee. Deterministe (un compte supprime puis rajoute
// retrouve son dossier) et sans l'adresse en clair dans les noms de fichiers.
func ID(adresse string) string {
	s := sha256.Sum256([]byte(NormaliserAdresse(adresse)))
	return hex.EncodeToString(s[:])[:12]
}

var (
	reAdresse = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	reCouleur = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

// AdresseValide : forme minimale d'une adresse e-mail.
func AdresseValide(s string) bool { return reAdresse.MatchString(NormaliserAdresse(s)) }

// CouleurValide : couleur hexadecimale #rrggbb.
func CouleurValide(s string) bool { return reCouleur.MatchString(s) }

// Lire renvoie le registre (vide s'il n'existe pas encore).
func Lire() (Registre, error) {
	mu.Lock()
	defer mu.Unlock()
	return lire()
}

func lire() (Registre, error) {
	var r Registre
	data, err := store.Load(fichier)
	if errors.Is(err, fs.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, fmt.Errorf("registre des comptes illisible : %w", err)
	}
	return r, nil
}

// Modifier applique fn au registre sous verrou puis l'ecrit (atomiquement). Un
// registre present mais indecodable n'est JAMAIS reecrit : on y perdrait les
// autres comptes. Si fn renvoie une erreur, rien n'est ecrit.
func Modifier(fn func(*Registre) error) (Registre, error) {
	mu.Lock()
	defer mu.Unlock()
	r, err := lire()
	if err != nil {
		return r, err
	}
	if err := fn(&r); err != nil {
		return r, err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return r, err
	}
	return r, store.Save(fichier, data)
}
