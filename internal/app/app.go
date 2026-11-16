// Package app porte la struct App : les bindings exposes au front et
// l'orchestration (comptes, synchro Gmail, cache).
package app

import (
	"context"
	"errors"
	"sync"

	"github.com/louis49/purgatorius/internal/cache"
	"github.com/louis49/purgatorius/internal/comptes"
	"github.com/louis49/purgatorius/internal/imap"
	"github.com/louis49/purgatorius/internal/secrets"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App est l'objet lie au front par Wails : ses methodes exportees deviennent
// des fonctions JS dans frontend/wailsjs/go/app/App.js.
type App struct {
	ctx     context.Context
	version string

	mu           sync.Mutex
	synchros     map[string]*enCours     // synchros en cours, par compte
	lecteurs     map[string]*cache.Cache // connexions de lecture du treemap, par compte
	suppressions map[string]bool         // comptes en cours de suppression
	corbeilles   map[string]bool         // comptes en cours de mise a la corbeille

	muAvis    sync.Mutex
	misDeCote map[string]string // Purgatoire mis de cote a l'ouverture, par compte : nom du fichier

	fenetre       EtatFenetre // dernier etat de fenetre enregistre
	fenetreConnue bool

	// emettre envoie un evenement au front (remplace dans les tests).
	emettre func(nom string, donnees any)
	// connecterIMAP remplace la connexion a Gmail (tests : faux serveur).
	connecterIMAP func(ctx context.Context) (*imap.Client, error)
	// supprimerSecret efface le secret d'un compte (remplace dans les tests).
	supprimerSecret func(secrets.Cle) error
}

// enCours : une synchro qui tourne ; fini est ferme quand sa goroutine a
// termine (base fermee, plus aucune ecriture).
type enCours struct {
	annuler context.CancelFunc
	fini    chan struct{}
}

// NewApp cree l'App. Le contexte Wails n'est connu qu'au demarrage (startup).
func NewApp() *App {
	return &App{version: versionDev, synchros: map[string]*enCours{}, lecteurs: map[string]*cache.Cache{}, suppressions: map[string]bool{}, corbeilles: map[string]bool{},
		misDeCote: map[string]string{}, emettre: func(string, any) {}, supprimerSecret: secrets.Supprimer}
}

// errSuppression : le compte est en cours de suppression.
var errSuppression = errors.New("compte en cours de suppression")

// verifierCompte (a.mu tenu) refuse un compte inconnu du registre ou en cours
// de suppression : rien ne doit creer ses fichiers (cache, Purgatoire) alors
// que la suppression les efface, ni pour un identifiant quelconque.
func (a *App) verifierCompte(id string) error {
	if a.suppressions[id] {
		return errSuppression
	}
	r, err := comptes.Lire()
	if err != nil {
		return err
	}
	if _, ok := r.Compte(id); !ok {
		return errors.New("compte inconnu")
	}
	return nil
}

// startup est appele par Wails une fois la fenetre creee.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.emettre = func(nom string, donnees any) { wailsruntime.EventsEmit(ctx, nom, donnees) }
	a.verifierCaches()
}

// shutdown interrompt proprement les synchros en cours : elles reprendront au
// prochain lancement (seuls les messages manquants sont relus).
func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, s := range a.synchros {
		s.annuler()
	}
	for _, c := range a.lecteurs {
		c.Fermer()
	}
}
