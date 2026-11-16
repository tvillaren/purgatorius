<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { LancerSynchro, PauseSynchro } from '../../wailsjs/go/app/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import Logo from '../composants/Logo.vue';
import MenuCompte from '../composants/MenuCompte.vue';
import { t } from '../i18n.js';
import { fmtDuree, fmtNombre, fmtTaille } from '../format.js';
import { synchronises } from '../synchroAuto.js';

// L'ecran n'est jamais une impasse : retour a la carte si le compte a deja une
// synchro complete, menu du compte (dont le changement de mot de passe et la
// suppression), passage a un autre compte.
const props = defineProps({ compte: { type: Object, required: true }, etat: { type: Object, default: null } });
const emit = defineEmits(['terminee', 'retour', 'choisir', 'recharger', 'supprime']);
const menu = ref(''); // '' | 'menu' | 'motdepasse' : volet initial du menu du compte
const code = ref(''); // code de la derniere erreur
const autres = computed(() => (props.etat?.comptes || []).filter((c) => c.id !== props.compte.id));

function apresMenu(quoi) {
  menu.value = '';
  emit('recharger');
  if (quoi === 'motdepasse') lancer(); // nouveau mot de passe : on reprend
}

const p = ref({ etape: 'connexion', faits: 0, total: 0, dejaEnCache: 0, volume: 0, restantSecondes: 0, annee: 0, domaines: 0 });
const enPause = ref(false);
const erreur = ref('');
const desabonnements = [];

const etapes = ['connexion', 'liste', 'entetes', 'categories'];
const rang = computed(() => etapes.indexOf(p.value.etape));
const tous = computed(() => p.value.dejaEnCache + p.value.total);
const lus = computed(() => p.value.dejaEnCache + p.value.faits);
const pourcent = computed(() => {
  if (p.value.etape === 'categories' || p.value.etape === 'terminee') return 100;
  return tous.value ? Math.floor((lus.value / tous.value) * 100) : 0;
});
const periode = computed(() => p.value.annee ? p.value.annee + ' → ' + new Date().getFullYear() : '');

async function lancer() {
  erreur.value = '';
  code.value = '';
  enPause.value = false;
  try {
    await LancerSynchro(props.compte.id);
  } catch (e) {
    erreur.value = t('erreur.autre', { detail: String(e) });
  }
}

async function pause() {
  await PauseSynchro(props.compte.id);
}

onMounted(() => {
  desabonnements.push(EventsOn('synchro:progression', (ev) => { if (ev.compte === props.compte.id) p.value = ev; }));
  desabonnements.push(EventsOn('synchro:terminee', (ev) => { if (ev.compte === props.compte.id) { synchronises.add(ev.compte); emit('terminee'); } }));
  // Cache vide (UIDVALIDITY changee) : le compte n'est plus complet, le retour
  // a la carte (partielle) disparait.
  desabonnements.push(EventsOn('synchro:vidage', (ev) => { if (ev.compte === props.compte.id) emit('recharger'); }));
  desabonnements.push(EventsOn('synchro:erreur', (ev) => {
    if (ev.compte !== props.compte.id) return;
    if (ev.code === 'annulee') { enPause.value = true; return; }
    code.value = ev.code;
    erreur.value = ev.code === 'autre' ? t('erreur.autre', { detail: ev.detail }) : t('erreur.' + ev.code);
  }));
  lancer();
});

onUnmounted(() => desabonnements.forEach((f) => f()));
</script>

<template>
  <div class="page">
    <div class="carte">
      <div class="tete">
        <div class="compte">
          <span class="point" :style="{ background: compte.couleur }" aria-hidden="true" /><span class="nom">{{ compte.nom }}</span><span class="sec">{{ compte.adresse }}</span>
          <button type="button" class="plus" :aria-label="t('menuCompte.ouvrir', { nom: compte.nom })" :aria-expanded="!!menu" @click="menu = menu ? '' : 'menu'">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><circle cx="5" cy="12" r="1.6" /><circle cx="12" cy="12" r="1.6" /><circle cx="19" cy="12" r="1.6" /></svg>
          </button>
          <MenuCompte v-if="menu" :compte="compte" :vue-initiale="menu" @fermer="menu = ''" @modifie="apresMenu" @supprime="menu = ''; emit('supprime')" />
        </div>
        <div class="droite">
          <select v-if="autres.length" class="champ autre" :aria-label="t('synchro.autreCompte')" @change="emit('choisir', $event.target.value)">
            <option value="" selected disabled>{{ t('synchro.autreCompte') }}</option>
            <option v-for="c in autres" :key="c.id" :value="c.id">{{ c.nom }}</option>
          </select>
          <Logo :taille="24" />
        </div>
      </div>
      <div class="titres">
        <h1>{{ compte.synchroComplete ? t('synchro.titreMaj') : t('synchro.titre') }}</h1>
        <p>{{ t('synchro.texte') }}</p>
      </div>
      <div class="avancement">
        <div class="ligne">
          <span class="mono pourcent">{{ pourcent }} %</span>
          <span class="sec">{{ enPause ? t('synchro.enPause') : (p.restantSecondes ? t('synchro.restant', { d: fmtDuree(p.restantSecondes) }) : '') }}</span>
        </div>
        <div class="barre" role="progressbar" :aria-valuenow="pourcent" aria-valuemin="0" aria-valuemax="100"><div :style="{ width: pourcent + '%' }" /></div>
      </div>
      <dl class="chiffres">
        <div><dt>{{ t('synchro.messages') }}</dt><dd class="mono">{{ fmtNombre(lus) }} / {{ fmtNombre(tous) }}</dd></div>
        <div><dt>{{ t('synchro.volume') }}</dt><dd class="mono">{{ fmtTaille(p.volume) }}</dd></div>
        <div><dt>{{ t('synchro.domaines') }}</dt><dd class="mono">{{ fmtNombre(p.domaines) }}</dd></div>
        <div><dt>{{ t('synchro.periode') }}</dt><dd class="mono">{{ periode }}</dd></div>
      </dl>
      <ol class="etapes">
        <li v-for="(e, i) in etapes" :key="e" :class="{ faite: i < rang, enCours: i === rang && !enPause && !erreur }">
          <svg v-if="i < rang" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--ok)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12l5 5 9-10" /></svg>
          <svg v-else-if="i === rang && !enPause && !erreur" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--acc)" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="12" r="8" stroke-dasharray="32 18" /></svg>
          <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><circle cx="12" cy="12" r="8" /></svg>
          {{ t('synchro.etape.' + e) }}
        </li>
      </ol>
      <div v-if="erreur" class="erreur" role="alert">
        <span>{{ erreur }}</span>
        <button v-if="code === 'authentification'" type="button" class="btn btn-secondaire" @click="menu = 'motdepasse'">{{ t('menuCompte.motDePasse') }}</button>
      </div>
      <div class="pied">
        <span class="aide">{{ t('synchro.reprise') }}</span>
        <span class="boutons">
          <button v-if="compte.synchroComplete" type="button" class="btn btn-secondaire" @click="emit('retour')">{{ t('synchro.retourCarte') }}</button>
          <button v-if="enPause || erreur" type="button" class="btn btn-principal" @click="lancer">{{ t('synchro.reprendre') }}</button>
          <button v-else type="button" class="btn btn-secondaire" @click="pause">{{ t('synchro.pause') }}</button>
        </span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page { min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 32px 16px; box-sizing: border-box; }
.carte { width: 100%; max-width: 720px; background: var(--carte); border-radius: 16px; padding: 40px; box-sizing: border-box; display: flex; flex-direction: column; gap: 32px; box-shadow: inset 0 0 0 1px var(--trait), var(--ombre-petite); }
.tete { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.compte { position: relative; display: flex; align-items: center; gap: 8px; min-width: 0; }
.compte .menu { left: 0; right: auto; width: 320px; }
.plus { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; border: 0; border-radius: 6px; background: transparent; color: var(--muet); padding: 0; }
.plus:hover, .plus[aria-expanded="true"] { background: var(--releve); color: var(--txt); }
.droite { display: flex; align-items: center; gap: 12px; }
.autre { min-height: 32px; width: auto; font-size: 13px; }
.boutons { display: flex; gap: 8px; flex-wrap: wrap; }
.point { width: 10px; height: 10px; border-radius: 50%; }
.nom { font-weight: 500; }
.sec { color: var(--sec); }
.titres { display: flex; flex-direction: column; gap: 8px; }
h1 { margin: 0; font-size: 28px; line-height: 1.15; font-weight: 500; }
.titres p { margin: 0; font-size: 15px; line-height: 22px; color: var(--sec); }
.avancement { display: flex; flex-direction: column; gap: 12px; }
.ligne { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; flex-wrap: wrap; }
.pourcent { font-size: 40px; font-weight: 500; line-height: 1; }
.barre { height: 8px; border-radius: 4px; background: var(--releve); overflow: hidden; }
.barre div { height: 100%; border-radius: 4px; background: var(--acc); transition: width 300ms var(--courbe); }
.chiffres { margin: 0; display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border-top: 1px solid var(--trait); border-bottom: 1px solid var(--trait); }
.chiffres div { padding: 16px; display: flex; flex-direction: column; gap: 4px; }
.chiffres div:first-child { padding-left: 0; }
.chiffres div + div { border-left: 1px solid var(--trait); }
dt { font-size: 13px; color: var(--sec); }
dd { margin: 0; font-size: 16px; }
.etapes { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
.etapes li { display: flex; gap: 8px; align-items: center; min-height: 24px; color: var(--muet); }
.etapes li.faite { color: var(--sec); }
.etapes li.enCours { color: var(--txt); font-weight: 500; }
/* Seule boucle admise (unslop) : elle signale un traitement reellement en cours. */
.etapes li.enCours svg { animation: tourner 1s linear infinite; }
@keyframes tourner { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .etapes li.enCours svg { animation: none; } }
.erreur { margin: 0; padding: 12px; border-radius: 10px; background: var(--erreur-doux); color: var(--erreur); line-height: 20px; display: flex; flex-direction: column; align-items: flex-start; gap: 8px; }
.pied { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; }
</style>
