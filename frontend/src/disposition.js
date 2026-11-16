// Disposition du treemap : variante de « squarify » (d3-hierarchy) qui
// privilegie les rangees horizontales, ou les noms se lisent.
//
// Squarify range chaque groupe de blocs le long du plus petit cote de la place
// restante. Quand un bloc domine, le reste finit en colonne etroite sur le
// bord et les noms y sont tronques. Ici, une rangee s'etend sur toute la
// largeur tant que la place n'est pas plus de LARGE fois plus large que haute ;
// au-dela (bande basse et longue), les blocs s'y suivent cote a cote. Le choix
// des blocs d'une rangee reste celui de squarify : on ajoute un bloc tant que
// le pire rapport largeur / hauteur de la rangee s'ameliore.
import { treemapDice, treemapSlice } from 'd3-hierarchy';

const LARGE = 2;

// Pire rapport d'aspect d'une rangee de longueur L (unite : surface par valeur s).
function pire(somme, min, max, L, s) {
  const epaisseur = (somme * s) / L;
  const petit = (min * s) / epaisseur;
  const grand = (max * s) / epaisseur;
  return Math.max(grand / epaisseur, epaisseur / grand, petit / epaisseur, epaisseur / petit);
}

// Tuile au format d3 (node, x0, y0, x1, y1) ; enfants tries par valeur
// decroissante.
export function tuileHorizontale(parent, x0, y0, x1, y1) {
  const noeuds = parent.children;
  const n = noeuds.length;
  let reste = parent.value;
  let i0 = 0;
  while (i0 < n) {
    const dx = x1 - x0;
    const dy = y1 - y0;
    // Rangee horizontale (sur toute la largeur) sauf place tres large.
    const horizontale = dx < LARGE * dy;
    const L = horizontale ? dx : dy;
    const s = reste > 0 ? (dx * dy) / reste : 0;
    let i1 = i0;
    let somme = 0;
    // Premier bloc non vide.
    do somme = noeuds[i1++].value; while (!somme && i1 < n);
    let min = somme;
    let max = somme;
    let meilleur = s ? pire(somme, min, max, L, s) : Infinity;
    for (; i1 < n; i1++) {
      const v = noeuds[i1].value;
      const r = pire(somme + v, Math.min(min, v), Math.max(max, v), L, s);
      if (r > meilleur) break;
      somme += v;
      min = Math.min(min, v);
      max = Math.max(max, v);
      meilleur = r;
    }
    const rangee = { value: somme, children: noeuds.slice(i0, i1) };
    if (horizontale) {
      const y = reste ? y0 + (dy * somme) / reste : y1;
      treemapDice(rangee, x0, y0, x1, y);
      y0 = y;
    } else {
      const x = reste ? x0 + (dx * somme) / reste : x1;
      treemapSlice(rangee, x0, y0, x, y1);
      x0 = x;
    }
    reste -= somme;
    i0 = i1;
  }
}
