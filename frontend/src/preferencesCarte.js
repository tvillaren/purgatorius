// Reglages de la carte, communs aux comptes. Le chemin de zoom et la
// selection restent temporaires. Un stockage indisponible ne bloque rien.
import { AXES } from './axes.js';

const CLE = 'purgatorius.carte';

export function lireReglagesCarte() {
  let r = {};
  try { r = JSON.parse(localStorage.getItem(CLE)) || {}; } catch (_) { /* facultatif */ }
  const axes = Array.isArray(r.axes) ? [...new Set(r.axes.filter((a) => AXES.includes(a)))] : [];
  return {
    axes: axes.length ? axes : ['domaine', 'expediteur'],
    metrique: ['taille', 'nombre'].includes(r.metrique) ? r.metrique : 'taille',
    boite: ['', 'recu', 'envoye'].includes(r.boite) ? r.boite : '',
    lecture: ['', 'nonlu', 'lu'].includes(r.lecture) ? r.lecture : '',
  };
}

export function ecrireReglagesCarte({ axes, metrique, boite, lecture }) {
  try { localStorage.setItem(CLE, JSON.stringify({ axes, metrique, boite, lecture })); } catch (_) { /* facultatif */ }
}
