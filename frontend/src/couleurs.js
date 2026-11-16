// Teintes des blocs : palette « Bonbon » (E), vive et claire, choisie par
// l'utilisateur parmi huit candidates (contre l'avis d'unslop, qui preferait des
// teintes douces). La meme en clair et en sombre ; le texte passe en blanc ou
// en noir selon le contraste.
//
// A la racine, une teinte par domaine, stable (hachage du nom). Dans un
// sous-niveau, les teintes suivent le rang des blocs (du plus gros au plus
// petit) : les douze plus gros ont des teintes toutes differentes, au lieu
// d'heriter tous de celle du domaine parent.
const palette = ['#60A5FA', '#34D399', '#FBBF24', '#F87171', '#A78BFA', '#F472B6', '#2DD4BF', '#FB923C', '#818CF8', '#A3E635', '#22D3EE', '#FACC15'];

export function estSombre() {
  return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function hachage(s) {
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619); }
  return h >>> 0;
}

// rang : position du bloc dans son niveau, ou -1 a la racine.
export function couleurBloc(noeud, rang = -1) {
  if (noeud.autres) return estSombre() ? '#3A3A3E' : '#C9CED6';
  if (rang >= 0) return palette[rang % palette.length];
  return palette[hachage(noeud.domaine || noeud.cle) % palette.length];
}

// encreSur : texte lisible sur une couleur de fond (contraste WCAG le plus fort).
export function encreSur(hex) {
  const n = parseInt(hex.slice(1), 16);
  const lin = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); };
  const L = 0.2126 * lin(n >> 16) + 0.7152 * lin((n >> 8) & 255) + 0.0722 * lin(n & 255);
  return 1.05 / (L + 0.05) >= (L + 0.05) / 0.05 ? '#FFFFFF' : '#171717';
}

// Pastilles proposees pour la couleur d'un compte (palette « Bonbon »).
export const COULEURS_COMPTE = [
  { cle: 'bleu', hex: '#60A5FA' }, { cle: 'vert', hex: '#34D399' }, { cle: 'ambre', hex: '#FBBF24' },
  { cle: 'rouge', hex: '#F87171' }, { cle: 'violet', hex: '#A78BFA' }, { cle: 'rose', hex: '#F472B6' },
];
