package app

import (
	"embed"
	"log"
	"path/filepath"

	"github.com/louis49/purgatorius/internal/store"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// Run monte la fenetre Wails. Les ressources embarquees viennent de main.go :
// le front compile, l'icone (Linux) et wails.json (version).
func Run(assets embed.FS, icone, wailsJSON []byte) error {
	if j := ouvrirJournal(); j != nil {
		defer j.Close()
	}
	// Windows : profil WebView2 dans le dossier de donnees plutot que dans le
	// profil itinerant (comme Merlinus). Vide si introuvable : defaut de Wails.
	profilWebview := ""
	if d, err := store.Dir(); err == nil {
		profilWebview = filepath.Join(d, "webview")
	}
	a := NewApp()
	a.version = versionDepuisConfig(wailsJSON)
	a.fenetre, a.fenetreConnue = lireFenetre()
	largeur, hauteur := taillesDemarrage(a.fenetre, a.fenetreConnue)
	etatDepart := options.Normal
	if a.fenetreConnue && a.fenetre.Maximisee {
		etatDepart = options.Maximised
	}
	log.Printf("Purgatorius %s", a.version)
	o := &options.App{
		Title:     "Purgatorius",
		Width:     largeur,
		Height:    hauteur,
		MinWidth:  largeurMin,
		MinHeight: hauteurMin,
		// Fenetre masquee le temps de la replacer (restaurerFenetre), pour
		// eviter un saut visible au lancement.
		StartHidden:      a.fenetreConnue && masqueeAuDemarrage(),
		WindowStartState: etatDepart,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 250, G: 250, B: 250, A: 1}, // fond clair (#fafafa) avant le premier rendu
		OnStartup:        a.startup,
		OnShutdown:       a.shutdown,
		OnDomReady:       a.restaurerFenetre,
		OnBeforeClose:    a.avantFermeture,
		Bind:             []interface{}{a},
		// Options macOS non nulles : sans elles, Wails desactive le bouton vert
		// (zoom / plein ecran) de la fenetre.
		Mac: &mac.Options{},
		Windows: &windows.Options{
			WebviewUserDataPath: profilWebview,
			// Options non nulles : Wails applique alors leurs valeurs zero, qui
			// couperaient le zoom (Ctrl + molette) actif par defaut. On le garde.
			IsZoomControlEnabled: true,
		},
		Linux: &linux.Options{
			Icon:        icone,         // fenetre et barre des taches
			ProgramName: "purgatorius", // WM_CLASS : rattache la fenetre au lanceur purgatorius.desktop
			// Sans options Linux, Wails force le rendu logiciel (issue wails #2977) ;
			// des options non nulles le repasseraient "a la demande". On garde le
			// comportement eprouve sur Merlinus.
			WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		},
	}
	appliquerTheme(themeForce(), o)
	return wails.Run(o)
}
