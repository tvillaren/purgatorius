package app

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/louis49/purgatorius/internal/cache"
	"github.com/louis49/purgatorius/internal/comptes"
	"github.com/louis49/purgatorius/internal/secrets"
	"github.com/louis49/purgatorius/internal/synchro"
)

// Evenements envoyes au front.
const (
	EvProgression = "synchro:progression"
	EvTerminee    = "synchro:terminee"
	EvErreur      = "synchro:erreur"
	// EvVidage : le cache vient d'etre vide (UIDVALIDITY changee), le compte
	// repasse en premiere synchro ; le front quitte la carte.
	EvVidage = "synchro:vidage"
)

// Progression : evenement synchro:progression.
type Progression struct {
	Compte          string  `json:"compte"`
	Etape           string  `json:"etape"`
	Faits           int     `json:"faits"`
	Total           int     `json:"total"`
	DejaEnCache     int     `json:"dejaEnCache"`
	Volume          int64   `json:"volume"`
	Debit           float64 `json:"debit"`
	RestantSecondes int     `json:"restantSecondes"`
	Annee           int     `json:"annee"`
	Domaines        int     `json:"domaines"`
	Expediteurs     int     `json:"expediteurs"`
}

// FinSynchro : evenement synchro:terminee ou synchro:erreur.
type FinSynchro struct {
	Compte     string `json:"compte"`
	Lus        int    `json:"lus"`
	Supprimes  int    `json:"supprimes"`
	Actualises int    `json:"actualises"`
	Total      int    `json:"total"`
	Code       string `json:"code"`
	Detail     string `json:"detail"`
}

// SynchroEnCours indique si le compte est en cours de synchro.
func (a *App) SynchroEnCours(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.synchros[id]
	return ok
}

// LancerSynchro demarre (ou reprend) la synchro du compte en arriere-plan. Sans
// effet si elle tourne deja. La progression arrive par evenements.
func (a *App) LancerSynchro(id string) error {
	r, err := comptes.Lire()
	if err != nil {
		return err
	}
	c, ok := r.Compte(id)
	if !ok {
		return errors.New("compte inconnu")
	}
	mdp, err := secrets.Lire(secrets.Cle{ID: id, Adresse: c.Adresse})
	if err != nil {
		return err
	}
	a.mu.Lock()
	if _, ok := a.synchros[id]; ok {
		a.mu.Unlock()
		return nil
	}
	// Revu sous verrou : une suppression a pu commencer (ou finir) pendant la
	// lecture du secret, sans trouver cette synchro a arreter.
	if err := a.verifierCompte(id); err != nil {
		a.mu.Unlock()
		return err
	}
	// Pendant une mise a la corbeille, la synchro attend : elle relirait des
	// messages en cours de deplacement.
	if a.corbeilles[id] {
		a.mu.Unlock()
		return errCorbeilleEnCours
	}
	ctx, annuler := context.WithCancel(a.contexte())
	s := &enCours{annuler: annuler, fini: make(chan struct{})}
	a.synchros[id] = s
	a.mu.Unlock()

	adresse := c.Adresse
	go a.synchroniser(ctx, s, id, adresse, mdp)
	return nil
}

// PauseSynchro interrompt la synchro du compte ; elle reprendra ou elle s'est
// arretee au prochain LancerSynchro.
func (a *App) PauseSynchro(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.synchros[id]; ok {
		s.annuler()
	}
}

// arreterSynchro interrompt la synchro du compte et attend que sa goroutine
// ait fini (base fermee), au plus delai.
func (a *App) arreterSynchro(id string, delai time.Duration) error {
	a.mu.Lock()
	s, ok := a.synchros[id]
	a.mu.Unlock()
	if !ok {
		return nil
	}
	s.annuler()
	select {
	case <-s.fini:
		return nil
	case <-time.After(delai):
		return errors.New("la synchro du compte ne s'arrete pas, reessayez dans un instant")
	}
}

func (a *App) synchroniser(ctx context.Context, s *enCours, id, adresse, mdp string) {
	defer func() {
		a.mu.Lock()
		delete(a.synchros, id)
		a.mu.Unlock()
		close(s.fini)
	}()
	fin := func(f FinSynchro) {
		f.Compte = id
		if f.Code != "" {
			log.Printf("synchro %s interrompue : %s", id, f.Code)
			a.emettre(EvErreur, f)
		} else {
			a.emettre(EvTerminee, f)
		}
	}
	cc, err := a.ouvrirCache(id)
	if err != nil {
		fin(FinSynchro{Code: synchro.ErreurAutre, Detail: err.Error()})
		return
	}
	defer cc.Fermer()
	debut := time.Now()
	// Cache vide (UIDVALIDITY changee) : le compte n'est plus complet tant que
	// la relecture n'est pas finie. Interrompue, elle reprendra a l'ecran de
	// synchro au lancement suivant, au lieu d'une carte partielle.
	avantVidage := func() error {
		_, err := comptes.Modifier(func(r *comptes.Registre) error {
			if c, ok := r.Compte(id); ok {
				c.SynchroComplete = false
			}
			return nil
		})
		if err == nil {
			a.emettre(EvVidage, FinSynchro{Compte: id})
		}
		return err
	}
	b, err := synchro.Lancer(ctx, cc, synchro.Options{Adresse: adresse, MotDePasse: mdp, AvantVidage: avantVidage, Connecter: a.connecterIMAP}, func(p synchro.Progression) {
		ev := Progression{Compte: id, Etape: p.Etape, Faits: p.Faits, Total: p.Total, DejaEnCache: p.DejaEnCache, Volume: p.Volume,
			Debit: p.Debit, RestantSecondes: int(p.Restant.Seconds()), Annee: p.AnneeLaPlusVieille}
		// Le rappel s'execute dans la goroutine de synchro, entre deux lots :
		// la base n'est pas en cours d'ecriture.
		if res, err := cc.Resumer(); err == nil {
			ev.Domaines, ev.Expediteurs = res.Domaines, res.Expediteurs
			ev.Volume = res.Taille
		}
		a.emettre(EvProgression, ev)
	})
	if err != nil {
		fin(FinSynchro{Code: synchro.CodeErreur(err), Detail: err.Error()})
		return
	}
	maintenant := time.Now().UTC()
	if _, err := comptes.Modifier(func(r *comptes.Registre) error {
		if c, ok := r.Compte(id); ok {
			c.SynchroComplete = true
			c.DerniereSynchro = &maintenant
		}
		return nil
	}); err != nil {
		// Sans ce marquage, l'ecran de synchro reviendrait : on le signale.
		fin(FinSynchro{Code: synchro.ErreurAutre, Detail: err.Error()})
		return
	}
	log.Printf("synchro %s terminee : %d lus sur %d, %d actualises, en %v", id, b.Lus, b.Total, b.Actualises, time.Since(debut).Round(time.Second))
	fin(FinSynchro{Lus: b.Lus, Supprimes: b.Supprimes, Actualises: b.Actualises, Total: b.Total})
}

// ResumeCompte : chiffres globaux du cache d'un compte.
func (a *App) ResumeCompte(id string) (cache.Resume, error) {
	c, err := a.lecteur(id)
	if err != nil {
		return cache.Resume{}, err
	}
	return c.Resumer()
}
