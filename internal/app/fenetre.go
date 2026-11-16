package app

// Emplacement et taille de la fenetre, retenus d'un lancement a l'autre
// (travail/fenetre.json), sur macOS, Windows et Linux.
//
// Wails positionne la fenetre relativement a l'ecran courant (zone utile sous
// macOS et Windows) et donne la taille des ecrans en pixels logiques : on borne
// donc l'etat relu a l'ecran courant, pour que la fenetre reste visible si
// l'ecran a change (plus petit, debranche). Consequence : une fenetre fermee
// sur un ecran secondaire revient sur l'ecran principal.
//
// Differences par OS (Wails v2.17) :
//   - macOS : lecture et ecriture dans le meme repere, rien a corriger ;
//   - Windows : la lecture est absolue, l'ecriture relative a la zone utile de
//     l'ecran (barre des taches en haut ou a gauche) ; on mesure ce decalage au
//     lancement (etalonnage) ;
//   - Linux : positionner une fenetre masquee est sans effet, elle est donc
//     affichee avant d'etre replacee ; sous Wayland le compositeur ignore la
//     position, seule la taille est restauree.

import (
	"context"
	"encoding/json"
	"log"
	goruntime "runtime"
	"time"

	"github.com/louis49/purgatorius/internal/store"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	ficFenetre      = "travail/fenetre.json"
	largeurDefaut   = 1280
	hauteurDefaut   = 800
	largeurMin      = 900
	hauteurMin      = 600
	margeVisible    = 120 // pixels de la fenetre qui doivent rester sur l'ecran
	periodeSauvegde = 3 * time.Second
	// Au-dela, l'etat relu est aberrant (fichier abime ou modifie a la main)
	// et ignore : aucun ecran ne fait 10 000 pixels logiques, aucun bureau ne
	// s'etend a 100 000.
	tailleMaxFenetre   = 10000
	positionMaxFenetre = 100000
)

// EtatFenetre : ce qui est retenu de la fenetre.
type EtatFenetre struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Largeur   int  `json:"largeur"`
	Hauteur   int  `json:"hauteur"`
	Maximisee bool `json:"maximisee"`
	Position  bool `json:"position"` // X et Y sont connus
}

func lireFenetre() (EtatFenetre, bool) {
	data, err := store.Load(ficFenetre)
	if err != nil {
		return EtatFenetre{}, false
	}
	var e EtatFenetre
	if json.Unmarshal(data, &e) != nil || !plausible(e) {
		return EtatFenetre{}, false
	}
	return e, true
}

func ecrireFenetre(e EtatFenetre) {
	data, _ := json.Marshal(e)
	if err := store.Save(ficFenetre, data); err != nil {
		log.Printf("etat de la fenetre non enregistre : %v", err)
	}
}

// plausible : tailles positives et bornees, position bornee.
func plausible(e EtatFenetre) bool {
	dans := func(v, max int) bool { return v >= -max && v <= max }
	return e.Largeur > 0 && e.Hauteur > 0 && e.Largeur <= tailleMaxFenetre && e.Hauteur <= tailleMaxFenetre &&
		dans(e.X, positionMaxFenetre) && dans(e.Y, positionMaxFenetre)
}

// taillesDemarrage : taille a donner a Wails au lancement (minimums respectes).
func taillesDemarrage(e EtatFenetre, ok bool) (int, int) {
	if !ok {
		return largeurDefaut, hauteurDefaut
	}
	return max(e.Largeur, largeurMin), max(e.Hauteur, hauteurMin)
}

// borner ramene la fenetre dans un ecran de taille ecranL x ecranH (pixels
// logiques) : taille au plus celle de l'ecran, et au moins margeVisible pixels
// de la fenetre visibles horizontalement, la barre de titre visible en haut.
// Ecran inconnu (Wails ne le donne pas) : la position, inverifiable, est
// abandonnee (la fenetre reste ou le systeme l'a placee) ; la taille est
// seulement bornee (minimums, tailleMaxFenetre).
func borner(e EtatFenetre, ecranL, ecranH int) EtatFenetre {
	if ecranL <= 0 || ecranH <= 0 {
		e.Position = false
		e.Largeur = min(max(e.Largeur, largeurMin), tailleMaxFenetre)
		e.Hauteur = min(max(e.Hauteur, hauteurMin), tailleMaxFenetre)
		return e
	}
	e.Largeur = min(max(e.Largeur, largeurMin), ecranL)
	e.Hauteur = min(max(e.Hauteur, hauteurMin), ecranH)
	if e.Position {
		e.X = min(max(e.X, margeVisible-e.Largeur), ecranL-margeVisible)
		e.Y = min(max(e.Y, 0), ecranH-margeVisible)
	}
	return e
}

// ecranCourant renvoie la taille logique de l'ecran courant (0, 0 si inconnue).
func ecranCourant(ctx context.Context) (int, int) {
	ecrans, err := wailsruntime.ScreenGetAll(ctx)
	if err != nil {
		return 0, 0
	}
	for _, s := range ecrans {
		if s.IsCurrent {
			return s.Size.Width, s.Size.Height
		}
	}
	for _, s := range ecrans {
		if s.IsPrimary {
			return s.Size.Width, s.Size.Height
		}
	}
	return 0, 0
}

// restaurerFenetre replace la fenetre (demarree masquee si une position est
// connue), puis l'affiche et lance la sauvegarde periodique.
func (a *App) restaurerFenetre(ctx context.Context) {
	e := a.fenetre
	if !masqueeAuDemarrage() {
		wailsruntime.WindowShow(ctx)
	}
	if a.fenetreConnue && !e.Maximisee {
		if goruntime.GOOS == "windows" && e.Position {
			// Etalonnage : position absolue de l'origine de la zone utile.
			wailsruntime.WindowSetPosition(ctx, 0, 0)
			ox, oy := wailsruntime.WindowGetPosition(ctx)
			e.X, e.Y = e.X-ox, e.Y-oy
		}
		l, h := ecranCourant(ctx)
		e = borner(e, l, h)
		wailsruntime.WindowSetSize(ctx, e.Largeur, e.Hauteur)
		if e.Position {
			wailsruntime.WindowSetPosition(ctx, e.X, e.Y)
		}
	}
	wailsruntime.WindowShow(ctx)
	go func() {
		t := time.NewTicker(periodeSauvegde)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				a.sauverFenetre(ctx)
			}
		}
	}()
}

// masqueeAuDemarrage : la fenetre demarre masquee (pour etre replacee sans
// saut visible) sauf sous Linux, ou une fenetre masquee ne se positionne pas.
func masqueeAuDemarrage() bool {
	return goruntime.GOOS != "linux"
}

// sauverFenetre enregistre l'etat de la fenetre s'il a change. Plein ecran et
// reduction ne sont pas retenus ; une fenetre agrandie garde sa derniere
// taille normale.
func (a *App) sauverFenetre(ctx context.Context) {
	if wailsruntime.WindowIsFullscreen(ctx) || wailsruntime.WindowIsMinimised(ctx) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	e := a.fenetre
	if wailsruntime.WindowIsMaximised(ctx) {
		e.Maximisee = true
	} else {
		x, y := wailsruntime.WindowGetPosition(ctx)
		l, h := wailsruntime.WindowGetSize(ctx)
		if l <= 0 || h <= 0 {
			return
		}
		e = EtatFenetre{X: x, Y: y, Largeur: l, Hauteur: h, Position: true}
	}
	if e == a.fenetre && a.fenetreConnue {
		return
	}
	a.fenetre, a.fenetreConnue = e, true
	ecrireFenetre(e)
}

// avantFermeture : derniere sauvegarde de la fenetre ; n'empeche jamais la
// fermeture.
func (a *App) avantFermeture(ctx context.Context) bool {
	a.sauverFenetre(ctx)
	return false
}
