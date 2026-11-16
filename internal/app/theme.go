package app

// Option de debogage : PURGATORIUS_THEME=clair (ou sombre) force l'apparence
// de l'app au lieu de suivre le systeme, pour verifier un theme sans changer
// celui de la machine.
//
//	macOS :   open --env PURGATORIUS_THEME=clair build/bin/Purgatorius.app
//	Windows : set PURGATORIUS_THEME=clair && build\bin\Purgatorius.exe
//	Linux :   PURGATORIUS_THEME=clair build/bin/purgatorius

import (
	"log"
	"os"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// themeForce renvoie "clair", "sombre" ou "" (suivre le systeme).
func themeForce() string {
	switch v := os.Getenv("PURGATORIUS_THEME"); v {
	case "clair", "sombre":
		return v
	case "":
	default:
		log.Printf("PURGATORIUS_THEME=%q ignore (clair ou sombre)", v)
	}
	return ""
}

// appliquerTheme regle les options de fenetre de chaque OS pour le theme
// force ; le front suit (prefers-color-scheme de la webview).
func appliquerTheme(theme string, o *options.App) {
	if theme == "" {
		return
	}
	log.Printf("theme force : %s", theme)
	sombre := theme == "sombre"
	if sombre {
		o.BackgroundColour = &options.RGBA{R: 20, G: 20, B: 21, A: 1} // #141415
		o.Mac.Appearance = mac.NSAppearanceNameDarkAqua
		o.Windows.Theme = windows.Dark
		// WebKitGTK suit la variante du theme GTK (a verifier selon la
		// version de WebKitGTK).
		os.Setenv("GTK_THEME", "Adwaita:dark")
		return
	}
	o.Mac.Appearance = mac.NSAppearanceNameAqua
	o.Windows.Theme = windows.Light
	os.Setenv("GTK_THEME", "Adwaita")
}
