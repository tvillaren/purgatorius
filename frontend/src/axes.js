import { t } from './i18n.js';
import { fmtTaille } from './format.js';

export const AXES = ['domaine', 'expediteur', 'destinataire', 'titre', 'boite', 'categorie', 'annee', 'mois', 'tranche'];

// Libelle affiche d'une valeur d'axe.
export function libelleValeur(axe, noeud) {
  if (noeud.autres) return t('carte.autres', { n: noeud.autres });
  if (axe === 'categorie') return t('categorie.' + noeud.cle);
  if (axe === 'tranche') return t('tranche.' + noeud.cle);
  if (axe === 'boite') return t('boite.' + noeud.cle);
  if (axe === 'destinataire' && !noeud.cle) return t('carte.sansDestinataire');
  return noeud.libelle || noeud.cle;
}

export function valeurMetrique(noeud, metrique) {
  return metrique === 'nombre' ? noeud.nombre : noeud.taille;
}

export { fmtTaille };

// Surface minimale d'un bloc a l'ecran (px2) : en dessous, il rejoint « Autres ».
// Environ 14 x 14 px : visible et cliquable, sans libelle.
export const AIRE_MIN = 196;

// regrouper : garde les blocs assez grands pour la surface de la carte et
// rassemble les autres (et l'eventuel reste renvoye par le backend) en un
// bloc « Autres », dont la liste d'exclusion permet d'y entrer.
export function regrouper(niveau, aire, metrique) {
  if (!niveau || niveau.axe === 'message' || !aire) return niveau;
  const total = metrique === 'nombre' ? niveau.nombre : niveau.taille;
  if (!total) return niveau;
  const garder = [];
  const reste = { cle: '\u0000autres', libelle: '', domaine: '', nombre: 0, taille: 0, autres: 0, exclure: [] };
  for (const n of niveau.noeuds) {
    if (n.autres) { reste.nombre += n.nombre; reste.taille += n.taille; reste.autres += n.autres; continue; }
    const v = metrique === 'nombre' ? n.nombre : n.taille;
    if (garder.length < 12 || (v / total) * aire >= AIRE_MIN) garder.push(n);
    else { reste.nombre += n.nombre; reste.taille += n.taille; reste.autres += 1; }
  }
  if (!reste.autres) return niveau;
  reste.exclure = garder.map((n) => n.cle);
  return { ...niveau, noeuds: [...garder, reste] };
}
