// Commande icone : rasterise l'icone SVG de Purgatorius en PNG transparent.
//
// Le SVG (frontend/src/assets/purgatorius.svg) reste la seule source. Il ne
// contient que des <rect> (x, y, width, height, rx, fill, stroke,
// stroke-width), eventuellement dans un <g transform="translate(a b)
// scale(s)">. Les outils du systeme (qlmanage) ajoutent un fond blanc opaque :
// d'ou ce rasteriseur minimal, anticrenele par sur-echantillonnage 4 x 4.
//
// Usage : go run ./scripts/icone <icone.svg> <sortie.png> [taille]
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type rect struct {
	x, y, w, h, r      float64
	remplissage, trait color.NRGBA
	epaisseur          float64
	tx, ty, s          float64 // transformation du groupe
}

var (
	reRect  = regexp.MustCompile(`<rect\b[^>]*>`)
	reAttr  = regexp.MustCompile(`([a-z-]+)="([^"]*)"`)
	reG     = regexp.MustCompile(`<g\b[^>]*transform="translate\(([-\d.]+)[ ,]+([-\d.]+)\)\s*scale\(([-\d.]+)\)"[^>]*>([\s\S]*?)</g>`)
	reRGBA  = regexp.MustCompile(`rgba\((\d+),\s*(\d+),\s*(\d+),\s*([\d.]+)\)`)
	reViewB = regexp.MustCompile(`viewBox="0 0 ([\d.]+) ([\d.]+)"`)
)

func couleur(s string) (color.NRGBA, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "" || s == "none":
		return color.NRGBA{}, nil
	case strings.HasPrefix(s, "#") && len(s) == 7:
		v, err := strconv.ParseUint(s[1:], 16, 32)
		return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, err
	}
	if m := reRGBA.FindStringSubmatch(s); m != nil {
		r, _ := strconv.Atoi(m[1])
		g, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		a, _ := strconv.ParseFloat(m[4], 64)
		return color.NRGBA{uint8(r), uint8(g), uint8(b), uint8(math.Round(a * 255))}, nil
	}
	return color.NRGBA{}, fmt.Errorf("couleur non geree : %q", s)
}

func lireRects(svg string, tx, ty, s float64) ([]rect, error) {
	var out []rect
	for _, balise := range reRect.FindAllString(svg, -1) {
		a := map[string]string{}
		for _, m := range reAttr.FindAllStringSubmatch(balise, -1) {
			a[m[1]] = m[2]
		}
		f := func(k string) float64 { v, _ := strconv.ParseFloat(a[k], 64); return v }
		r := rect{x: f("x"), y: f("y"), w: f("width"), h: f("height"), r: f("rx"), epaisseur: f("stroke-width"), tx: tx, ty: ty, s: s}
		var err error
		if r.remplissage, err = couleur(a["fill"]); err != nil {
			return nil, err
		}
		if r.trait, err = couleur(a["stroke"]); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// dedans : le point (px, py) est-il dans le rectangle arrondi agrandi de d ?
func dedans(px, py, x, y, w, h, r, d float64) bool {
	x, y, w, h, r = x-d, y-d, w+2*d, h+2*d, math.Max(0, r+d)
	if px < x || px > x+w || py < y || py > y+h {
		return false
	}
	dx := math.Max(0, math.Max(x+r-px, px-(x+w-r)))
	dy := math.Max(0, math.Max(y+r-py, py-(y+h-r)))
	return dx*dx+dy*dy <= r*r
}

func melanger(fond, c color.NRGBA, couverture float64) color.NRGBA {
	a := float64(c.A) / 255 * couverture
	if a <= 0 {
		return fond
	}
	fa := float64(fond.A) / 255
	ra := a + fa*(1-a)
	comp := func(cs, fs uint8) uint8 {
		return uint8(math.Round((float64(cs)*a + float64(fs)*fa*(1-a)) / ra))
	}
	return color.NRGBA{comp(c.R, fond.R), comp(c.G, fond.G), comp(c.B, fond.B), uint8(math.Round(ra * 255))}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage : icone <icone.svg> <sortie.png> [taille]")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	svg := string(data)
	vb := 1024.0
	if m := reViewB.FindStringSubmatch(svg); m != nil {
		vb, _ = strconv.ParseFloat(m[1], 64)
	}
	taille := int(vb)
	if len(os.Args) > 3 {
		taille, _ = strconv.Atoi(os.Args[3])
	}
	// Ordre du document : les rectangles hors groupe (le carreau) d'abord, puis
	// ceux des groupes (le logo). Suffisant pour l'icone ; un SVG plus riche
	// demanderait un vrai moteur.
	groupes := reG.FindAllStringSubmatch(svg, -1)
	reste := svg
	for _, g := range groupes {
		reste = strings.Replace(reste, g[0], "", 1)
	}
	formes, err := lireRects(reste, 0, 0, 1)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, g := range groupes {
		tx, _ := strconv.ParseFloat(g[1], 64)
		ty, _ := strconv.ParseFloat(g[2], 64)
		s, _ := strconv.ParseFloat(g[3], 64)
		dansG, err := lireRects(g[4], tx, ty, s)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		formes = append(formes, dansG...)
	}

	const ss = 4
	echelle := vb / float64(taille)
	img := image.NewNRGBA(image.Rect(0, 0, taille, taille))
	for py := 0; py < taille; py++ {
		for px := 0; px < taille; px++ {
			pixel := color.NRGBA{}
			for _, f := range formes {
				nr, nt := 0, 0
				for sy := 0; sy < ss; sy++ {
					for sx := 0; sx < ss; sx++ {
						x := (float64(px) + (float64(sx)+0.5)/ss) * echelle
						y := (float64(py) + (float64(sy)+0.5)/ss) * echelle
						lx, ly := (x-f.tx)/f.s, (y-f.ty)/f.s
						if f.remplissage.A > 0 && dedans(lx, ly, f.x, f.y, f.w, f.h, f.r, 0) {
							nr++
						}
						if f.trait.A > 0 && f.epaisseur > 0 && dedans(lx, ly, f.x, f.y, f.w, f.h, f.r, f.epaisseur/2) &&
							!dedans(lx, ly, f.x, f.y, f.w, f.h, f.r, -f.epaisseur/2) {
							nt++
						}
					}
				}
				if nr > 0 {
					pixel = melanger(pixel, f.remplissage, float64(nr)/ss/ss)
				}
				if nt > 0 {
					pixel = melanger(pixel, f.trait, float64(nt)/ss/ss)
				}
			}
			img.SetNRGBA(px, py, pixel)
		}
	}
	sortie, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer sortie.Close()
	if err := png.Encode(sortie, img); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
