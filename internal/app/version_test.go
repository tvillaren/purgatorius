package app

import "testing"

func TestVersionDepuisConfig(t *testing.T) {
	t.Setenv("PURGATORIUS_VERSION", "")
	cas := []struct {
		nom, json, attendu string
	}{
		{"release", `{"info":{"productVersion":"1.2.3"}}`, "1.2.3"},
		{"sans version", `{"info":{"productName":"Purgatorius"}}`, versionDev},
		{"json invalide", `{`, versionDev},
	}
	for _, c := range cas {
		if v := versionDepuisConfig([]byte(c.json)); v != c.attendu {
			t.Errorf("%s : %q, attendu %q", c.nom, v, c.attendu)
		}
	}
}

func TestVersionDepuisEnvironnement(t *testing.T) {
	t.Setenv("PURGATORIUS_VERSION", "0.9.0")
	if v := versionDepuisConfig([]byte(`{"info":{"productVersion":"1.2.3"}}`)); v != "0.9.0" {
		t.Errorf("version %q, attendu 0.9.0 (variable d'environnement)", v)
	}
}
