import { langue } from './i18n.js';

// Espace fine insecable interdite (consigne typographique) : espace simple.
export function fmtNombre(n) {
  const s = String(Math.round(n || 0));
  return langue.value === 'fr' ? s.replace(/\B(?=(\d{3})+(?!\d))/g, ' ') : s.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
}

// Octets en Ko / Mo / Go (base 1000, comme Gmail).
export function fmtTaille(octets) {
  const fr = langue.value === 'fr';
  const u = fr ? ['o', 'Ko', 'Mo', 'Go', 'To'] : ['B', 'KB', 'MB', 'GB', 'TB'];
  let v = octets || 0;
  let i = 0;
  while (v >= 1000 && i < u.length - 1) { v /= 1000; i++; }
  const txt = i >= 3 ? v.toFixed(1) : String(Math.round(v));
  return (fr ? txt.replace('.', ',') : txt) + ' ' + u[i];
}

export function fmtDuree(secondes) {
  const s = Math.max(0, Math.round(secondes || 0));
  if (s < 60) return langue.value === 'fr' ? 'moins d\'une minute' : 'less than a minute';
  const m = Math.round(s / 60);
  return langue.value === 'fr' ? m + ' min' : m + ' min';
}

export function fmtDate(iso) {
  if (!iso) return '';
  return new Date(iso).toLocaleString(langue.value === 'fr' ? 'fr-FR' : 'en-GB', { dateStyle: 'medium', timeStyle: 'short' });
}

// Date d'un message (secondes Unix) : « 12 mars 2024 ».
export function fmtJour(secondes) {
  if (!secondes) return '';
  return new Date(secondes * 1000).toLocaleDateString(langue.value === 'fr' ? 'fr-FR' : 'en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
}
