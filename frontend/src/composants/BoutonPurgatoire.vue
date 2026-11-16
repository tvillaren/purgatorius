<script setup>
// Bouton du Purgatoire : deux jauges, taille puis nombre, avec
// le gain de chacune. Cible de depot des lignes glissees depuis la liste ;
// un clic ouvre (ou referme) la revue du Purgatoire.
import { computed, ref } from 'vue';
import IconePurgatoire from './IconePurgatoire.vue';
import { t } from '../i18n.js';
import { fmtNombre, fmtTaille } from '../format.js';
import { TYPE_GLISSE } from '../glisser.js';

const props = defineProps({
  etat: { type: Object, default: null },
  actif: { type: Boolean, default: false }, // revue du Purgatoire ouverte
});
const emit = defineEmits(['deposer', 'ouvrir']);

const survol = ref(false);
const e = computed(() => props.etat || { nombre: 0, taille: 0, totalNombre: 0, totalTaille: 0 });
// Largeur d'une jauge : au moins quelques pixels des qu'il y a quelque chose,
// sinon un seul message sur des dizaines de milliers ne se verrait pas.
const part = (x, total) => ({ width: (total ? Math.min(100, (x / total) * 100) : 0) + '%', minWidth: x > 0 ? '4px' : '0' });

function dessus(ev) {
  if (!Array.from(ev.dataTransfer.types || []).includes(TYPE_GLISSE)) return;
  ev.preventDefault();
  ev.dataTransfer.dropEffect = 'move';
  survol.value = true;
}
function deposer(ev) {
  survol.value = false;
  const brut = ev.dataTransfer.getData(TYPE_GLISSE);
  if (!brut) return;
  ev.preventDefault();
  try { emit('deposer', JSON.parse(brut)); } catch (_) { /* donnees illisibles : rien */ }
}
</script>

<template>
  <button type="button" class="purgatoire" :class="{ survol }" :aria-pressed="actif"
    :aria-label="t('purgatoire.jauges', { taille: fmtTaille(e.taille), total: fmtTaille(e.totalTaille), n: fmtNombre(e.nombre), totalN: fmtNombre(e.totalNombre) })"
    @click="emit('ouvrir')" @dragover="dessus" @dragenter="dessus" @dragleave="survol = false" @drop="deposer">
    <IconePurgatoire />
    <span class="nom">{{ t('purgatoire.titre') }}</span>
    <span class="jauges" aria-hidden="true">
      <span class="piste"><span class="taille" :style="part(e.taille, e.totalTaille)" /></span>
      <span class="mono gain">−{{ fmtTaille(e.taille) }}</span>
      <span class="piste"><span class="nombre" :style="part(e.nombre, e.totalNombre)" /></span>
      <span class="mono gain">{{ t('purgatoire.msg', { n: fmtNombre(e.nombre) }) }}</span>
    </span>
  </button>
</template>

<style scoped>
.purgatoire { display: inline-flex; align-items: center; gap: 12px; min-height: 44px; padding: 0 12px; border: 0; border-radius: 10px; background: var(--carte); color: var(--txt); box-shadow: inset 0 0 0 1px var(--trait); }
.purgatoire svg { color: var(--sec); }
.purgatoire:hover, .purgatoire[aria-pressed="true"] { background: var(--releve); }
.purgatoire.survol { box-shadow: inset 0 0 0 2px var(--acc); outline: 2px dashed var(--acc); outline-offset: 3px; }
.nom { font-weight: 500; }
.jauges { display: inline-grid; grid-template-columns: 96px auto; column-gap: 8px; row-gap: 2px; align-items: center; }
.piste { display: block; height: 6px; border-radius: 3px; background: var(--releve); overflow: hidden; }
/* Bouton survole ou enfonce : son fond prend la teinte des pistes ; celles-ci
   passent a la teinte de la carte pour rester visibles. */
.purgatoire:hover .piste, .purgatoire[aria-pressed="true"] .piste { background: var(--carte); }
.piste span { display: block; height: 100%; border-radius: 3px; }
.taille { background: var(--acc); }
.nombre { background: var(--jauge-nombre); }
.gain { font-size: 11px; line-height: 14px; color: var(--sec); text-align: left; }
</style>
