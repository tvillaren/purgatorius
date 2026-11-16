package app

import (
	"encoding/json"
	"os"
)

// Version de l'application.
//
// Source : wails.json (info.productVersion), embarque par main.go. Le workflow
// de release y ecrit le numero du tag avant la compilation (le meme champ
// alimente Info.plist et les proprietes Windows). Hors release le champ est
// absent : version "dev". PURGATORIUS_VERSION la remplace (developpement).

const versionDev = "dev"

// versionDepuisConfig lit info.productVersion du wails.json embarque.
func versionDepuisConfig(wailsJSON []byte) string {
	if v := os.Getenv("PURGATORIUS_VERSION"); v != "" {
		return v
	}
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if json.Unmarshal(wailsJSON, &cfg) != nil || cfg.Info.ProductVersion == "" {
		return versionDev
	}
	return cfg.Info.ProductVersion
}

// Version renvoie la version de l'application ("dev" hors release).
func (a *App) Version() string { return a.version }
