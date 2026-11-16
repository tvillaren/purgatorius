package app

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

func TestThemeForce(t *testing.T) {
	for v, att := range map[string]string{"": "", "clair": "clair", "sombre": "sombre", "rose": ""} {
		t.Setenv("PURGATORIUS_THEME", v)
		if got := themeForce(); got != att {
			t.Errorf("PURGATORIUS_THEME=%q : %q, attendu %q", v, got, att)
		}
	}
	t.Setenv("GTK_THEME", "")
	o := &options.App{Mac: &mac.Options{}, Windows: &windows.Options{}}
	appliquerTheme("clair", o)
	if o.Mac.Appearance != mac.NSAppearanceNameAqua || o.Windows.Theme != windows.Light {
		t.Fatalf("theme clair non applique : %+v %+v", o.Mac, o.Windows)
	}
	o = &options.App{Mac: &mac.Options{}, Windows: &windows.Options{}}
	appliquerTheme("", o)
	if o.Mac.Appearance != "" || o.Windows.Theme != windows.SystemDefault {
		t.Fatal("theme modifie sans option")
	}
}
