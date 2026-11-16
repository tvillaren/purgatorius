// Traductions FR / EN. t('cle', { n: 3 }) remplace {n} dans le texte.
import { ref } from 'vue';
import { textes } from './locales.js';

export const langue = ref('fr');

export function choisirLangue(l) {
  langue.value = l === 'en' ? 'en' : 'fr';
  document.documentElement.lang = langue.value;
}

export function langueSysteme() {
  return (navigator.language || 'fr').toLowerCase().startsWith('fr') ? 'fr' : 'en';
}

export function t(cle, params = {}) {
  const s = (textes[langue.value] && textes[langue.value][cle]) ?? textes.fr[cle] ?? cle;
  return s.replace(/\{(\w+)\}/g, (_, k) => (params[k] ?? ''));
}
