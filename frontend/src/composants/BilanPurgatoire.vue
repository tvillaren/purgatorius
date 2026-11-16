<script setup>
// Bilan du Purgatoire (panneau de droite de la revue) : place liberee, boite
// avant et apres, quelques chiffres, et les actions sur tout le Purgatoire.
// Mise a la corbeille en deux temps (le bouton, puis la confirmation qui
// rappelle nombre et taille) ; pendant, son avancement remplace le bouton.
import { computed, ref, watch } from 'vue';
import { t } from '../i18n.js';
import { fmtNombre, fmtTaille } from '../format.js';

const props = defineProps({
  etat: { type: Object, default: null },
  avancement: { type: Object, default: null }, // { faits, total } pendant la mise a la corbeille
});
const emit = defineEmits(['vider', 'corbeille']);
const confirmer = ref(false);
// Le Purgatoire change (retrait, envoi) : la confirmation repart de zero.
watch(() => [props.etat?.nombre, props.etat?.taille], () => { confirmer.value = false; });

function confirmerCorbeille() {
  confirmer.value = false;
  emit('corbeille');
}

const e = computed(() => props.etat || { nombre: 0, taille: 0, pj: 0, nonLus: 0, totalNombre: 0, totalTaille: 0 });
const apres = computed(() => Math.max(0, e.value.totalTaille - e.value.taille));
const partApres = computed(() => (e.value.totalTaille ? (apres.value / e.value.totalTaille) * 100 : 0) + '%');
const vide = computed(() => e.value.nombre === 0);
</script>

<template>
  <aside class="bilan" :aria-label="t('bilan.titre')">
    <div class="tete">
      <span class="sec">{{ t('bilan.place') }}</span>
      <span class="mono gros">{{ fmtTaille(e.taille) }}</span>
    </div>
    <div class="barres">
      <div>
        <div class="legende"><span class="sec">{{ t('bilan.aujourdhui') }}</span><span class="mono">{{ fmtTaille(e.totalTaille) }}</span></div>
        <div class="piste pleine" />
      </div>
      <div>
        <div class="legende"><span class="sec">{{ t('bilan.apres') }}</span><span class="mono">{{ fmtTaille(apres) }}</span></div>
        <div class="piste"><span :style="{ width: partApres }" /></div>
      </div>
    </div>
    <dl class="chiffres">
      <div><dt>{{ t('bilan.messages') }}</dt><dd class="mono">{{ fmtNombre(e.nombre) }}</dd></div>
      <div><dt>{{ t('bilan.pj') }}</dt><dd class="mono">{{ fmtTaille(e.pj) }}</dd></div>
      <div><dt>{{ t('bilan.nonLus') }}</dt><dd class="mono">{{ fmtNombre(e.nonLus) }}</dd></div>
    </dl>
    <div class="actions">
      <p v-if="avancement" class="encours" role="status">
        {{ t('bilan.enCours', { faits: fmtNombre(avancement.faits), total: fmtNombre(avancement.total) }) }}
        <span class="piste"><span :style="{ width: (avancement.total ? (avancement.faits / avancement.total) * 100 : 0) + '%' }" /></span>
      </p>
      <div v-else-if="confirmer" class="confirmation" role="group" :aria-label="t('bilan.confirmerOui')">
        <p>{{ t(e.nombre > 1 ? 'bilan.confirmer' : 'bilan.confirmer1', { n: fmtNombre(e.nombre), taille: fmtTaille(e.taille) }) }}</p>
        <div class="boutons">
          <button type="button" class="btn btn-secondaire" @click="confirmer = false">{{ t('commun.annuler') }}</button>
          <button type="button" class="btn btn-principal" @click="confirmerCorbeille">{{ t('bilan.confirmerOui') }}</button>
        </div>
      </div>
      <button v-else type="button" class="btn btn-principal" :disabled="vide" @click="confirmer = true">
        {{ t(vide ? 'bilan.corbeille0' : e.nombre > 1 ? 'bilan.corbeille' : 'bilan.corbeille1', { n: fmtNombre(e.nombre) }) }}
      </button>
      <button type="button" class="btn btn-secondaire" :disabled="vide || !!avancement" @click="emit('vider')">{{ t('bilan.toutRetirer') }}</button>
      <p class="note">{{ t('bilan.trente') }}</p>
    </div>
  </aside>
</template>

<style scoped>
.bilan { flex: 0 0 312px; min-width: 0; background: var(--carte); border-left: 1px solid var(--trait); padding: 24px 20px; box-sizing: border-box; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; }
.tete { display: flex; flex-direction: column; gap: 4px; }
.sec { font-size: 13px; color: var(--sec); }
.gros { font-size: 32px; font-weight: 500; line-height: 1.1; }
.barres { display: flex; flex-direction: column; gap: 12px; }
.barres > div { display: flex; flex-direction: column; gap: 4px; }
.legende { display: flex; justify-content: space-between; font-size: 13px; }
.piste { display: flex; height: 8px; border-radius: 4px; overflow: hidden; background: var(--releve); }
.piste.pleine { background: var(--muet); }
.piste span { display: block; height: 100%; background: var(--acc); }
.chiffres { margin: 0; display: flex; flex-direction: column; }
.chiffres div { display: flex; justify-content: space-between; gap: 12px; padding: 8px 0; border-bottom: 1px solid var(--trait); }
dt { color: var(--sec); }
dd { margin: 0; }
.actions { display: flex; flex-direction: column; gap: 8px; margin-top: auto; }
.actions .btn { min-height: 44px; padding: 0 8px; font-size: 13px; }
.note { margin: 0; font-size: 12px; line-height: 18px; color: var(--muet); }
.confirmation { display: flex; flex-direction: column; gap: 8px; padding: 12px; border-radius: 10px; background: var(--releve); }
.confirmation p { margin: 0; font-size: 13px; line-height: 20px; }
.confirmation .boutons { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.encours { margin: 0; display: flex; flex-direction: column; gap: 8px; font-size: 13px; }
</style>
