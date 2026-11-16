<script setup>
// Aiguillage des ecrans (pas de routeur) :
//   aucun compte                  -> accueil
//   compte actif non synchronise  -> synchro (reprend la ou elle s'est arretee)
//   sinon                         -> compte
import { computed, onMounted, ref } from 'vue';
import { Etat, ChoisirCompte } from '../wailsjs/go/app/App';
import { choisirLangue, langueSysteme, t } from './i18n.js';
import Accueil from './ecrans/Accueil.vue';
import Profil from './ecrans/Profil.vue';
import Synchro from './ecrans/Synchro.vue';
import Carte from './ecrans/Carte.vue';

const etat = ref(null);
const ecran = ref('chargement');
const compteActif = computed(() => etat.value?.comptes.find((c) => c.id === etat.value.actif) || null);

async function recharger() {
  etat.value = await Etat();
  if (!etat.value.langue) choisirLangue(langueSysteme());
  else choisirLangue(etat.value.langue);
}

function aiguiller() {
  if (!etat.value.comptes.length) ecran.value = 'accueil';
  else if (!compteActif.value?.synchroComplete) ecran.value = 'synchro';
  else ecran.value = 'compte';
}

onMounted(async () => {
  await recharger();
  aiguiller();
});

async function apresCreation() {
  await recharger();
  ecran.value = 'synchro';
}

async function apresSynchro() {
  await recharger();
  ecran.value = 'compte';
}

async function choisir(id) {
  await ChoisirCompte(id);
  await recharger();
  aiguiller();
}

// Apres renommage, changement de couleur ou suppression d'un compte.
async function actualiser() {
  await recharger();
  aiguiller();
}

function annulerProfil() {
  if (etat.value.comptes.length) aiguiller();
  else ecran.value = 'accueil';
}
</script>

<template>
  <p v-if="ecran === 'chargement'" class="chargement">{{ t('commun.chargement') }}</p>
  <p v-else-if="etat?.erreur" class="erreur-globale" role="alert">{{ t('erreur.registre', { detail: etat.erreur }) }}</p>
  <Accueil v-else-if="ecran === 'accueil'" @ajouter="ecran = 'profil'" />
  <Profil v-else-if="ecran === 'profil'" @cree="apresCreation" @annuler="annulerProfil" />
  <Synchro v-else-if="ecran === 'synchro' && compteActif" :key="compteActif.id" :compte="compteActif" :etat="etat" @terminee="apresSynchro"
    @retour="ecran = 'compte'" @choisir="choisir" @recharger="recharger" @supprime="actualiser" />
  <Carte v-else-if="ecran === 'compte' && compteActif" :etat="etat" :compte="compteActif"
    @choisir="choisir" @ajouter="ecran = 'profil'" @synchroniser="ecran = 'synchro'" @recharger="actualiser" />
</template>

<style scoped>
.chargement, .erreur-globale { padding: 32px; color: var(--sec); }
.erreur-globale { color: var(--erreur); }
</style>
