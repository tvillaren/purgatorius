package app

import (
	"errors"
	"log"
	"time"

	"github.com/louis49/purgatorius/internal/comptes"
	"github.com/louis49/purgatorius/internal/secrets"
	"github.com/louis49/purgatorius/internal/synchro"
)

// EvCorbeille : avancement d'une mise a la corbeille.
const EvCorbeille = "corbeille:progression"

// errCorbeilleEnCours : une mise a la corbeille du compte est en cours.
var errCorbeilleEnCours = errors.New("mise a la corbeille en cours")

// ProgressionCorbeille : donnees de EvCorbeille.
type ProgressionCorbeille struct {
	Compte string `json:"compte"`
	Faits  int    `json:"faits"`
	Total  int    `json:"total"`
}

// ResultatCorbeille : issue d'une mise a la corbeille. Apres une erreur, les
// compteurs disent ce qui est deja parti (le reste est encore au Purgatoire).
type ResultatCorbeille struct {
	Deplaces int    `json:"deplaces"`
	Taille   int64  `json:"taille"`
	Absents  int    `json:"absents"`
	Ambigus  int    `json:"ambigus"`
	Code     string `json:"code"`   // "" si reussi ; ecart, numerotation, corbeille_absente, en_cours ou un code de synchro
	Detail   string `json:"detail"` // texte de l'erreur, jamais le mot de passe
}

// MettreALaCorbeille met a la corbeille de Gmail tout le Purgatoire du
// compte : les messages que la revue affiche, au nombre de attendu (sinon
// rien ne part, code « ecart »). La synchro du compte est arretee avant et
// refusee pendant. Les messages restent 30 jours dans la corbeille de Gmail.
func (a *App) MettreALaCorbeille(id string, attendu int) ResultatCorbeille {
	a.mu.Lock()
	if err := a.verifierCompte(id); err != nil {
		a.mu.Unlock()
		return ResultatCorbeille{Code: synchro.ErreurAutre, Detail: err.Error()}
	}
	if a.corbeilles[id] {
		a.mu.Unlock()
		return ResultatCorbeille{Code: "en_cours"}
	}
	a.corbeilles[id] = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.corbeilles, id)
		a.mu.Unlock()
	}()

	if err := a.arreterSynchro(id, 10*time.Second); err != nil {
		return ResultatCorbeille{Code: synchro.ErreurAutre, Detail: err.Error()}
	}
	r, err := comptes.Lire()
	if err != nil {
		return ResultatCorbeille{Code: synchro.ErreurAutre, Detail: err.Error()}
	}
	c, ok := r.Compte(id)
	if !ok {
		return ResultatCorbeille{Code: synchro.ErreurAutre, Detail: "compte inconnu"}
	}
	mdp, err := secrets.Lire(secrets.Cle{ID: id, Adresse: c.Adresse})
	if err != nil {
		return ResultatCorbeille{Code: synchro.ErreurAutre, Detail: err.Error()}
	}
	cc, err := a.ouvrirCache(id)
	if err != nil {
		return ResultatCorbeille{Code: synchro.ErreurAutre, Detail: err.Error()}
	}
	defer cc.Fermer()

	debut := time.Now()
	opt := synchro.Options{Adresse: c.Adresse, MotDePasse: mdp, Connecter: a.connecterIMAP}
	b, err := synchro.MettreALaCorbeille(a.contexte(), cc, opt, attendu, func(faits, total int) {
		a.emettre(EvCorbeille, ProgressionCorbeille{Compte: id, Faits: faits, Total: total})
	})
	res := ResultatCorbeille{Deplaces: b.Deplaces, Taille: b.Taille, Absents: b.Absents, Ambigus: b.Ambigus}
	log.Printf("corbeille %s : %d deplaces, %d deja partis, %d ambigus, en %v", id, b.Deplaces, b.Absents, b.Ambigus, time.Since(debut).Round(time.Millisecond))
	if err != nil {
		res.Detail = err.Error()
		switch {
		case errors.Is(err, synchro.ErrEcartCorbeille):
			res.Code = "ecart"
		case errors.Is(err, synchro.ErrNumerotation):
			res.Code = "numerotation"
		case errors.Is(err, synchro.ErrCorbeilleAbsente):
			res.Code = "corbeille_absente"
		default:
			res.Code = synchro.CodeErreur(err)
		}
		log.Printf("corbeille %s interrompue : %s", id, res.Code)
	}
	return res
}
