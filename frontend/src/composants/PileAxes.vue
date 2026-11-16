<script setup>
// Pile d'axes de regroupement : reordonnable par glisser-deposer (ou par les
// fleches, au clavier), retrait, ajout et prereglages.
import { computed, ref } from 'vue';
import { AXES } from '../axes.js';
import { t } from '../i18n.js';

const props = defineProps({ modelValue: { type: Array, required: true } });
const emit = defineEmits(['update:modelValue']);

const traine = ref(-1);
const restants = computed(() => AXES.filter((a) => !props.modelValue.includes(a)));

function maj(liste) { emit('update:modelValue', liste); }
function retirer(i) { if (props.modelValue.length > 1) maj(props.modelValue.filter((_, j) => j !== i)); }
function deplacer(de, vers) {
  if (vers < 0 || vers >= props.modelValue.length || de === vers) return;
  const l = [...props.modelValue];
  const [x] = l.splice(de, 1);
  l.splice(vers, 0, x);
  maj(l);
}
function ajouter(ev) {
  if (ev.target.value) maj([...props.modelValue, ev.target.value]);
  ev.target.value = '';
}
function deposer(i) { deplacer(traine.value, i); traine.value = -1; }
</script>

<template>
  <section class="pile">
    <h2>{{ t('axes.titre') }}</h2>
    <ol>
      <li v-for="(a, i) in modelValue" :key="a" draggable="true" :class="{ traine: traine === i }"
        @dragstart="traine = i" @dragend="traine = -1" @dragover.prevent @drop.prevent="deposer(i)">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true" class="poignee"><path d="M9 6h.01M15 6h.01M9 12h.01M15 12h.01M9 18h.01M15 18h.01" /></svg>
        <span class="mono rang">{{ i + 1 }}</span>
        <span class="nom">{{ t('axe.' + a) }}</span>
        <button type="button" class="icone" :aria-label="t('axes.monter', { axe: t('axe.' + a) })" :disabled="i === 0" @click="deplacer(i, i - 1)">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 15l6-6 6 6" /></svg>
        </button>
        <button type="button" class="icone" :aria-label="t('axes.retirer', { axe: t('axe.' + a) })" :disabled="modelValue.length === 1" @click="retirer(i)">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><path d="M7 7l10 10M17 7L7 17" /></svg>
        </button>
      </li>
      <li class="feuille"><span class="poignee-vide" /><span class="mono rang">{{ modelValue.length + 1 }}</span><span class="nom">{{ t('axe.message') }}</span></li>
    </ol>
    <label v-if="restants.length" class="ajout">
      <span class="sr-only">{{ t('axes.ajouter') }}</span>
      <select @change="ajouter">
        <option value="">{{ t('axes.ajouter') }}</option>
        <option v-for="a in restants" :key="a" :value="a">{{ t('axe.' + a) }}</option>
      </select>
    </label>
  </section>
</template>

<style scoped>
.pile { display: flex; flex-direction: column; gap: 8px; }
h2 { margin: 0; padding: 0 8px; font-size: 13px; font-weight: 500; color: var(--muet); }
ol { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
li { display: flex; align-items: center; gap: 8px; min-height: 36px; padding: 0 4px 0 8px; border-radius: 8px; background: var(--carte); box-shadow: inset 0 0 0 1px var(--trait); font-size: 14px; cursor: grab; }
li.traine { opacity: 0.5; }
li.feuille { background: transparent; color: var(--muet); cursor: default; }
.poignee { color: var(--muet); flex-shrink: 0; }
.poignee-vide { width: 16px; }
.rang { font-size: 12px; color: var(--muet); }
.nom { flex-grow: 1; }
.icone { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; border: 0; border-radius: 6px; background: transparent; color: var(--muet); padding: 0; }
.icone:hover:not(:disabled) { background: var(--releve); color: var(--txt); }
.icone:disabled { opacity: 0.3; }
.ajout select { font: inherit; width: 100%; min-height: 36px; padding: 0 8px; border-radius: 8px; border: 1px solid var(--champ-bord); background: var(--carte); color: var(--sec); font-size: 14px; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
</style>
