<script setup>
// Menu d'un compte : renommer, changer la couleur, changer le mot de passe
// d'application (teste aupres de Gmail avant d'etre enregistre), supprimer
// (confirmation dans le menu : les dialogues systeme ne sont pas fiables dans
// la webview).
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue';
import { ChangerMotDePasse, ModifierCompte, SupprimerCompte } from '../../wailsjs/go/app/App';
import { COULEURS_COMPTE } from '../couleurs.js';
import { t } from '../i18n.js';

const props = defineProps({
  compte: { type: Object, required: true },
  vueInitiale: { type: String, default: 'menu' }, // ouvrir directement un volet (ex. 'motdepasse')
});
const emit = defineEmits(['fermer', 'modifie', 'supprime']);

const vue = ref(props.vueInitiale); // menu | renommer | couleur | motdepasse | supprimer
const mdp = ref('');
const test = ref(false); // test de connexion en cours
const mdpPret = computed(() => mdp.value.replace(/\s/g, '').length >= 16 && !test.value);
const nom = ref(props.compte.nom);
const couleur = ref(props.compte.couleur);
const erreur = ref('');
const racine = ref(null);
const champ = ref(null);

async function ouvrir(v) {
  vue.value = v;
  erreur.value = '';
  if (v === 'renommer' || v === 'motdepasse') { await nextTick(); champ.value?.focus(); champ.value?.select?.(); }
}

async function changerMotDePasse() {
  if (!mdpPret.value) return;
  test.value = true;
  erreur.value = '';
  try {
    const r = await ChangerMotDePasse(props.compte.id, mdp.value);
    if (r.ok) { mdp.value = ''; emit('modifie', 'motdepasse'); return; }
    erreur.value = r.code === 'autre' ? t('erreur.autre', { detail: r.detail }) : r.code === 'coffre' ? t('erreur.coffre', { detail: r.detail }) : t('erreur.' + r.code);
  } catch (e) { erreur.value = String(e); } finally { test.value = false; }
}

async function enregistrer() {
  try {
    await ModifierCompte(props.compte.id, nom.value, couleur.value);
    emit('modifie');
  } catch (e) { erreur.value = String(e); }
}

async function choisirCouleur(hex) {
  couleur.value = hex;
  await enregistrer();
}

async function supprimer() {
  try {
    await SupprimerCompte(props.compte.id);
    emit('supprime');
  } catch (e) { erreur.value = String(e); }
}

function dehors(ev) { if (racine.value && !racine.value.contains(ev.target)) emit('fermer'); }
function touche(ev) { if (ev.key === 'Escape') { ev.stopPropagation(); emit('fermer'); } }
onMounted(() => {
  if (vue.value !== 'menu') ouvrir(vue.value);
  setTimeout(() => document.addEventListener('mousedown', dehors), 0);
  document.addEventListener('keydown', touche, true);
});
onUnmounted(() => {
  document.removeEventListener('mousedown', dehors);
  document.removeEventListener('keydown', touche, true);
});
</script>

<template>
  <div ref="racine" class="menu" role="dialog" :aria-label="t('menuCompte.titre', { nom: compte.nom })">
    <template v-if="vue === 'menu'">
      <button type="button" class="item" @click="ouvrir('renommer')">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 20h4L19 9l-4-4L4 16v4z" /></svg>
        {{ t('menuCompte.renommer') }}
      </button>
      <button type="button" class="item" @click="ouvrir('couleur')">
        <span class="pastille-petite" :style="{ background: compte.couleur }" aria-hidden="true" />
        {{ t('menuCompte.couleur') }}
      </button>
      <button type="button" class="item" @click="ouvrir('motdepasse')">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="8" cy="15" r="4" /><path d="M11 12l9-9M17 6l3 3M15 8l2 2" /></svg>
        {{ t('menuCompte.motDePasse') }}
      </button>
      <span class="separateur" aria-hidden="true" />
      <button type="button" class="item danger" @click="ouvrir('supprimer')">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" /></svg>
        {{ t('menuCompte.supprimer') }}
      </button>
    </template>

    <form v-else-if="vue === 'renommer'" class="volet" @submit.prevent="enregistrer">
      <label class="etiquette" for="nomCompte">{{ t('profil.nom') }}</label>
      <input id="nomCompte" ref="champ" v-model="nom" class="champ" type="text" :placeholder="compte.adresse" />
      <div class="actions">
        <button type="button" class="btn btn-secondaire" @click="emit('fermer')">{{ t('commun.annuler') }}</button>
        <button type="submit" class="btn btn-principal">{{ t('menuCompte.enregistrer') }}</button>
      </div>
    </form>

    <form v-else-if="vue === 'motdepasse'" class="volet" @submit.prevent="changerMotDePasse">
      <label class="etiquette" for="mdpCompte">{{ t('profil.mdp') }}</label>
      <input id="mdpCompte" ref="champ" v-model="mdp" class="champ mono mdp" type="password" autocomplete="off" spellcheck="false" />
      <span class="aide-volet">{{ t('menuCompte.motDePasseAide') }}</span>
      <div class="actions">
        <button type="button" class="btn btn-secondaire" @click="emit('fermer')">{{ t('commun.annuler') }}</button>
        <button type="submit" class="btn btn-principal" :disabled="!mdpPret">{{ test ? t('profil.test') : t('menuCompte.tester') }}</button>
      </div>
    </form>

    <div v-else-if="vue === 'couleur'" class="volet">
      <span class="etiquette">{{ t('profil.couleur') }}</span>
      <div class="pastilles">
        <button v-for="p in COULEURS_COMPTE" :key="p.hex" type="button" class="pastille" :aria-label="t('profil.couleurs.' + p.cle)"
          :aria-pressed="couleur.toLowerCase() === p.hex.toLowerCase()" :style="{ background: p.hex }" @click="choisirCouleur(p.hex)" />
        <label class="libre" :title="t('profil.autreCouleur')">
          <span class="sr-only">{{ t('profil.autreCouleur') }}</span>
          <input type="color" :value="couleur" @change="choisirCouleur($event.target.value)" />
        </label>
      </div>
    </div>

    <div v-else class="volet">
      <p class="avertissement">{{ t('menuCompte.confirmer', { nom: compte.nom }) }}</p>
      <div class="actions">
        <button type="button" class="btn btn-secondaire" @click="emit('fermer')">{{ t('commun.annuler') }}</button>
        <button type="button" class="btn danger-plein" @click="supprimer">{{ t('menuCompte.supprimer') }}</button>
      </div>
    </div>

    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
  </div>
</template>

<style scoped>
.menu { position: absolute; z-index: 20; left: 8px; right: 8px; top: calc(100% + 4px); background: var(--carte); border-radius: 12px; box-shadow: inset 0 0 0 1px var(--trait), 0 8px 24px rgba(0, 0, 0, 0.12); padding: 4px; display: flex; flex-direction: column; }
.item { display: flex; align-items: center; gap: 12px; min-height: 36px; padding: 0 8px; border: 0; border-radius: 8px; background: transparent; color: var(--txt); text-align: left; font-size: 14px; }
.item:hover { background: var(--releve); }
.item svg { color: var(--muet); }
.item.danger, .item.danger svg { color: var(--erreur); }
.pastille-petite { width: 12px; height: 12px; border-radius: 50%; margin: 0 2px; }
.separateur { height: 1px; background: var(--trait); margin: 4px 0; }
.volet { display: flex; flex-direction: column; gap: 8px; padding: 8px; }
.actions { display: flex; justify-content: flex-end; gap: 8px; }
.actions .btn { min-height: 32px; padding: 0 12px; font-size: 13px; }
.pastilles { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
.pastille { width: 28px; height: 28px; border-radius: 50%; border: 0; padding: 0; }
.pastille[aria-pressed="true"] { box-shadow: 0 0 0 2px var(--carte), 0 0 0 4px var(--txt); }
.libre { display: inline-flex; width: 28px; height: 28px; border-radius: 50%; overflow: hidden; box-shadow: inset 0 0 0 1px var(--champ-bord); cursor: pointer; }
.libre input { width: 40px; height: 40px; margin: -6px; padding: 0; border: 0; background: transparent; cursor: pointer; }
.avertissement { margin: 0; font-size: 13px; line-height: 20px; color: var(--sec); }
.aide-volet { font-size: 12px; line-height: 18px; color: var(--muet); }
.mdp { letter-spacing: 0.12em; }
.danger-plein { background: var(--erreur); color: #ffffff; }
.erreur { margin: 4px 8px 8px; font-size: 13px; color: var(--erreur); }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
</style>
