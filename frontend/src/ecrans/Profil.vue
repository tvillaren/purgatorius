<script setup>
import { computed, ref } from 'vue';
import { TesterConnexion, AjouterCompte } from '../../wailsjs/go/app/App';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { t } from '../i18n.js';
import { fmtNombre } from '../format.js';
import { COULEURS_COMPTE } from '../couleurs.js';

const emit = defineEmits(['cree', 'annuler']);

const nom = ref('');
const adresse = ref('');
const mdp = ref('');
const couleur = ref('#60A5FA');
const etat = ref('saisie'); // saisie | test | ok
const erreur = ref('');
const messages = ref(0);

const pastilles = COULEURS_COMPTE;

const pret = computed(() => adresse.value.trim() && mdp.value.replace(/\s/g, '').length >= 16 && etat.value === 'saisie');

function ouvrir(url) { BrowserOpenURL(url); }

async function valider() {
  erreur.value = '';
  etat.value = 'test';
  const r = await TesterConnexion(adresse.value, mdp.value);
  if (!r.ok) {
    etat.value = 'saisie';
    erreur.value = r.code === 'autre' ? t('erreur.autre', { detail: r.detail }) : t('erreur.' + r.code);
    return;
  }
  messages.value = r.messages;
  etat.value = 'ok';
  try {
    await AjouterCompte({ nom: nom.value, adresse: adresse.value, couleur: couleur.value, motDePasse: mdp.value });
    mdp.value = '';
    emit('cree');
  } catch (e) {
    etat.value = 'saisie';
    erreur.value = t('erreur.coffre', { detail: String(e) });
  }
}
</script>

<template>
  <div class="profil">
    <header>
      <button type="button" class="retour" @click="emit('annuler')">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 6l-6 6 6 6" /></svg>
        {{ t('commun.retour') }}
      </button>
      <h1>{{ t('profil.titre') }}</h1>
    </header>
    <div class="corps">
      <form class="formulaire" @submit.prevent="valider">
        <div class="ligne-deux">
          <label class="groupe"><span class="etiquette">{{ t('profil.nom') }}</span>
            <input v-model="nom" class="champ" type="text" :placeholder="t('profil.nomExemple')" autocomplete="off" />
          </label>
          <label class="groupe"><span class="etiquette">{{ t('profil.adresse') }}</span>
            <input v-model="adresse" class="champ" type="email" placeholder="prenom.nom@gmail.com" autocomplete="email" required />
          </label>
        </div>
        <fieldset>
          <legend class="etiquette">{{ t('profil.couleur') }}</legend>
          <div class="couleurs">
            <button v-for="p in pastilles" :key="p.hex" type="button" class="pastille" :aria-label="t('profil.couleurs.' + p.cle)"
              :aria-pressed="couleur.toLowerCase() === p.hex.toLowerCase()" :style="{ background: p.hex }" @click="couleur = p.hex" />
            <span class="separateur" aria-hidden="true" />
            <label class="libre">
              <input v-model="couleur" type="color" />
              {{ t('profil.autreCouleur') }}
            </label>
            <span class="mono aide">{{ couleur }}</span>
          </div>
        </fieldset>
        <fieldset>
          <legend class="etiquette">{{ t('profil.connexion') }}</legend>
          <div class="modes">
            <label class="mode actif"><input type="radio" name="mode" checked />
              <span><span class="etiquette">{{ t('profil.modeMdp') }}</span><br /><span class="sec">{{ t('profil.modeMdpAide') }}</span></span>
            </label>
            <label class="mode"><input type="radio" name="mode" disabled />
              <span><span class="etiquette">{{ t('profil.modeOauth') }}</span><br /><span class="sec">{{ t('profil.modeOauthAide') }}</span></span>
            </label>
          </div>
        </fieldset>
        <label class="groupe"><span class="etiquette">{{ t('profil.mdp') }}</span>
          <input v-model="mdp" class="champ mono mdp" type="password" autocomplete="off" spellcheck="false" required />
          <span class="aide">{{ t('profil.mdpAide') }}</span>
        </label>
        <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
        <p v-else-if="etat === 'test'" class="statut" role="status">{{ t('profil.test') }}</p>
        <p v-else-if="etat === 'ok'" class="statut ok" role="status">{{ t('profil.ok', { n: fmtNombre(messages) }) }}</p>
        <div class="actions">
          <button type="button" class="btn btn-secondaire" @click="emit('annuler')">{{ t('commun.annuler') }}</button>
          <button type="submit" class="btn btn-principal" :disabled="!pret">{{ t('profil.tester') }}</button>
        </div>
      </form>
      <aside class="guide">
        <h2>{{ t('profil.guideTitre') }}</h2>
        <ol>
          <li>{{ t('profil.guide1') }}<br /><a href="#" @click.prevent="ouvrir('https://myaccount.google.com/signinoptions/twosv')">{{ t('profil.guide1Lien') }}</a></li>
          <li>{{ t('profil.guide2') }}<br /><a href="#" @click.prevent="ouvrir('https://myaccount.google.com/apppasswords')">{{ t('profil.guide2Lien') }}</a></li>
          <li>{{ t('profil.guide3') }}</li>
        </ol>
        <p class="note">{{ t('profil.guideNote') }}</p>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.profil { min-height: 100vh; display: flex; flex-direction: column; }
header { display: flex; align-items: center; gap: 16px; min-height: 56px; padding: 0 24px; border-bottom: 1px solid var(--trait); background: var(--carte); }
.retour { display: inline-flex; align-items: center; gap: 8px; min-height: 32px; border: 0; background: transparent; color: var(--sec); font-size: 14px; padding: 0; }
h1 { margin: 0; font-size: 16px; font-weight: 500; }
.corps { flex-grow: 1; display: flex; flex-wrap: wrap; gap: 24px; padding: 32px 24px; max-width: 1120px; width: 100%; margin: 0 auto; box-sizing: border-box; align-items: flex-start; }
.formulaire { flex: 999 1 520px; min-width: 0; background: var(--carte); border-radius: 16px; padding: 32px; display: flex; flex-direction: column; gap: 24px; box-sizing: border-box; box-shadow: inset 0 0 0 1px var(--trait); }
.ligne-deux { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.groupe { display: flex; flex-direction: column; gap: 8px; }
fieldset { border: 0; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
legend { padding: 0; margin-bottom: 8px; }
.couleurs { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.pastille { width: 32px; height: 32px; border-radius: 50%; border: 0; padding: 0; }
.pastille[aria-pressed="true"] { box-shadow: 0 0 0 2px var(--carte), 0 0 0 4px var(--txt); }
.separateur { width: 1px; height: 24px; background: var(--trait); margin: 0 8px; }
.libre { display: inline-flex; align-items: center; gap: 8px; min-height: 36px; padding: 0 12px 0 4px; border: 1px solid var(--champ-bord); border-radius: 8px; color: var(--sec); cursor: pointer; }
.libre input { width: 28px; height: 28px; padding: 0; border: 0; background: transparent; }
.modes { display: flex; flex-wrap: wrap; gap: 8px; }
.mode { flex: 1 1 200px; display: flex; gap: 12px; align-items: flex-start; padding: 12px; border-radius: 10px; box-shadow: inset 0 0 0 1px var(--trait); line-height: 20px; color: var(--sec); }
.mode.actif { box-shadow: inset 0 0 0 1px var(--acc); background: var(--acc-doux); color: var(--txt); }
.mode input { margin-top: 3px; }
.sec { color: var(--sec); }
.mdp { letter-spacing: 0.12em; }
.erreur { margin: 0; padding: 12px; border-radius: 10px; background: var(--erreur-doux); color: var(--erreur); line-height: 20px; }
.statut { margin: 0; color: var(--sec); }
.statut.ok { color: var(--ok); }
.actions { display: flex; justify-content: flex-end; gap: 8px; padding-top: 24px; border-top: 1px solid var(--trait); }
.guide { flex: 1 1 300px; min-width: 0; background: var(--carte); border-radius: 16px; padding: 24px; display: flex; flex-direction: column; gap: 16px; box-sizing: border-box; box-shadow: inset 0 0 0 1px var(--trait); }
.guide h2 { margin: 0; font-size: 15px; font-weight: 500; }
.guide ol { margin: 0; padding-left: 20px; display: flex; flex-direction: column; gap: 12px; line-height: 20px; color: var(--sec); }
.note { margin: 0; padding: 12px; border-radius: 10px; background: var(--releve); font-size: 13px; line-height: 20px; color: var(--sec); }
</style>
