<script setup>
// Ecran principal : treemap du compte actif. Barre laterale (comptes,
// metrique, filtres, pile d'axes), carte au centre (liste des messages au
// dernier niveau), detail a droite. Double-clic : entrer dans un bloc ; Echap
// ou fil d'Ariane : remonter. Le bouton Purgatoire ouvre sa revue a la place
// de la carte (liste des messages du Purgatoire, bilan a droite).
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { Carte, DetailCarte, EnvoyerBlocPurgatoire, EnvoyerPurgatoire, EtatPurgatoire, LancerSynchro, MettreALaCorbeille, PurgatoireMisDeCote, RetirerPurgatoire, SynchroEnCours, ViderPurgatoire } from '../../wailsjs/go/app/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { synchronises } from '../synchroAuto.js';
import Logo from '../composants/Logo.vue';
import CarteCanvas from '../composants/CarteCanvas.vue';
import ListeMessages from '../composants/ListeMessages.vue';
import BoutonPurgatoire from '../composants/BoutonPurgatoire.vue';
import PanneauDetail from '../composants/PanneauDetail.vue';
import BilanPurgatoire from '../composants/BilanPurgatoire.vue';
import PileAxes from '../composants/PileAxes.vue';
import MenuCompte from '../composants/MenuCompte.vue';
import { libelleValeur, regrouper, valeurMetrique } from '../axes.js';
import { couleurBloc } from '../couleurs.js';
import { t } from '../i18n.js';
import { fmtDate, fmtNombre, fmtTaille } from '../format.js';

const props = defineProps({ etat: { type: Object, required: true }, compte: { type: Object, required: true } });
const emit = defineEmits(['choisir', 'ajouter', 'synchroniser', 'recharger']);

const axes = ref(['domaine', 'expediteur']);
const metrique = ref('taille');
const boite = ref(''); // '' : tous ; 'recu' ; 'envoye'
const lecture = ref(''); // '' : tous ; 'nonlu' ; 'lu'
const menuCompte = ref(''); // compte dont le menu est ouvert

// Panneaux repliables (memorises dans ce navigateur integre ; facultatif).
function lirePref(cle, defaut) { try { const v = localStorage.getItem(cle); return v === null ? defaut : v === '1'; } catch (_) { return defaut; } }
function ecrirePref(cle, v) { try { localStorage.setItem(cle, v ? '1' : '0'); } catch (_) { /* facultatif */ } }
const flancVisible = ref(lirePref('purgatorius.flanc', true));
const detailVisible = ref(lirePref('purgatorius.detail', true));
watch(flancVisible, (v) => ecrirePref('purgatorius.flanc', v));
watch(detailVisible, (v) => ecrirePref('purgatorius.detail', v));

// Nouveau mot de passe (teste avant d'etre enregistre) : la synchro du compte
// reprend avec lui, ce qui efface l'erreur d'authentification du pied.
function apresMenu(quoi) {
  const id = menuCompte.value;
  menuCompte.value = '';
  emit('recharger');
  if (quoi === 'motdepasse' && id) relancer(id);
}
const chemin = ref([]); // { axe, valeur, exclure, libelle }
const niveau = ref(null);
// Contexte du niveau affiche : le chemin et les filtres avec lesquels il a ete
// demande. Pendant le chargement d'un autre niveau, chemin, boite et lecture
// ont deja change alors que la carte montre encore l'ancien niveau : tout ce
// qui part d'un bloc affiche (entrer, envoyer, detail) s'appuie sur ce
// contexte, pas sur l'etat courant.
let contexte = { chemin: [], boite: '', lecture: '' };
// Le niveau affiche est la racine (« Tous les messages ») : il n'est jamais
// envoyable en entier.
const aLaRacine = ref(true);
// Bloc ou message selectionne au clic : le panneau s'y fixe. Un bloc porte sa
// demande d'envoi au Purgatoire, figee au moment de la selection.
const choix = ref(null);
const aire = ref(0); // surface de la carte (px2), pour regrouper les petits blocs
const affiche = computed(() => regrouper(niveau.value, aire.value, metrique.value));
const detail = ref(null);
const erreur = ref('');
let jeton = 0;
let jetonDetail = 0;
let minuterie = null;
const memoire = new Map();

const filtres = (c) => c.map(({ axe, valeur, exclure }) => ({ axe, valeur, exclure: exclure || [] }));

// Tout changement de niveau ou de filtre passe par ici : la selection est
// videe tout de suite (un bloc de l'ancien niveau ne doit pas rester
// envoyable pendant le chargement), puis de nouveau a la reponse (un clic sur
// l'ancienne carte pendant le chargement).
async function charger() {
  const n = ++jeton;
  erreur.value = '';
  choix.value = null;
  memoire.clear();
  const demande = { chemin: chemin.value, boite: boite.value, lecture: lecture.value };
  try {
    const r = await Carte(props.compte.id, { axes: axes.value, chemin: filtres(demande.chemin), metrique: metrique.value, boite: demande.boite, lecture: demande.lecture });
    if (n !== jeton) return;
    niveau.value = r;
    contexte = demande;
    aLaRacine.value = !demande.chemin.length;
    choix.value = null;
    chargerDetail();
  } catch (e) {
    erreur.value = String(e);
  }
}

// selectionner : un bloc du niveau affiche, un message, ou rien.
function selectionner(n) {
  if (!n) { choix.value = null; return; }
  if (n.message) { choix.value = n; return; }
  choix.value = { ...n, demande: { chemin: [...filtres(contexte.chemin), filtreDe(n)], boite: contexte.boite, lecture: contexte.lecture } };
}

function filtreDe(noeud) {
  return noeud.autres
    ? { axe: niveau.value.axe, valeur: '', exclure: noeud.exclure || [] }
    : { axe: niveau.value.axe, valeur: noeud.cle, exclure: [] };
}

// Detail du panneau : le bloc ou le message choisi, sinon le niveau. Seule la
// reponse de la derniere demande s'affiche (une demande lente sur un grand
// niveau ne doit pas ecraser celle d'un niveau plus petit, arrivee avant).
function chargerDetail() {
  clearTimeout(minuterie);
  const n = ++jetonDetail;
  minuterie = setTimeout(async () => {
    const ch = choix.value;
    let q = { chemin: filtres(chemin.value), boite: boite.value, lecture: lecture.value };
    let uid = 0;
    if (ch && ch.message) uid = ch.uid;
    else if (ch?.demande && !(ch.autres && niveau.value.axe === 'message')) q = ch.demande;
    const cle = JSON.stringify([q, uid, metrique.value]);
    if (!memoire.has(cle)) {
      const p = DetailCarte(props.compte.id, { ...q, uid, metrique: metrique.value });
      memoire.set(cle, p);
      p.catch(() => { if (memoire.get(cle) === p) memoire.delete(cle); }); // un echec n'est pas garde
    }
    try {
      const d = await memoire.get(cle);
      if (n === jetonDetail) detail.value = d;
    } catch (e) {
      if (n === jetonDetail) erreur.value = String(e);
    }
  }, 30);
}

function entrer(noeud) {
  if (!noeud) return;
  // Double-clic sur un message : rien a ouvrir dans la carte, on montre son
  // detail (et le panneau s'il etait masque).
  if (noeud.message) { selectionner(noeud); detailVisible.value = true; return; }
  if (niveau.value.axe === 'message') return;
  if (noeud.autres && !noeud.exclure?.length) return;
  // Depuis le chemin du niveau affiche, pas depuis l'etat courant.
  chemin.value = [...contexte.chemin, { ...filtreDe(noeud), libelle: libelleValeur(niveau.value.axe, noeud), teinte: teinteDe(noeud) }];
  charger();
}

// Clic sur une ligne de la liste : son detail a droite (panneau affiche).
function choisirMessage(l) {
  choix.value = { message: true, uid: l.uid, cle: String(l.uid), libelle: l.objet || t('carte.sansObjet'), taille: l.taille, nombre: 1 };
  detailVisible.value = true;
}

// Purgatoire : jauges, envoi (lignes, message ou bloc entier) et annulation du
// dernier envoi. Ce qui y part disparait de la carte et de la liste.
const etatPurgatoire = ref(null);
const envoi = ref(null); // dernier lot envoye { uids, nombre, taille }
const versionListe = ref(0);
const enListe = computed(() => niveau.value?.axe === 'message');
// Envoyable : le bloc ou le message choisi ; sans choix, le niveau affiche
// (le bloc dans lequel on est entre), sauf a la racine.
const envoyable = computed(() => {
  if (choix.value) return choix.value.message || !(choix.value.autres && !choix.value.exclure?.length);
  return !aLaRacine.value && niveau.value?.nombre > 0;
});

async function chargerPurgatoire() {
  try { etatPurgatoire.value = await EtatPurgatoire(props.compte.id); } catch (e) { erreur.value = String(e); }
}

// Avis (une information, pas une erreur), ferme a la main : bloc change avant
// son envoi, Purgatoire illisible mis de cote.
const avis = ref('');

// Purgatoire illisible mis de cote a l'ouverture du cache (souvent au
// demarrage, avant l'affichage : l'avis est garde cote Go et rendu une fois).
async function verifierMisDeCote() {
  const id = props.compte.id;
  try {
    const fichier = await PurgatoireMisDeCote(id);
    if (!fichier || props.compte.id !== id) return;
    avis.value = t('purgatoire.misDeCote', { fichier });
    apresChangement(); // le Purgatoire neuf est vide : ses messages reviennent dans la carte
  } catch (_) { /* l'avis n'est qu'une information */ }
}

function apresChangement() {
  versionListe.value++;
  charger();
  chargerPurgatoire();
}

async function envoyerUids(uids) {
  if (!uids?.length) return;
  try {
    const lot = await EnvoyerPurgatoire(props.compte.id, uids);
    if (!lot.nombre) return;
    envoi.value = lot;
    apresChangement();
  } catch (e) { erreur.value = String(e); }
}

// Envoi du bloc choisi (exactement la demande figee a sa selection) ou, sans
// choix, du niveau affiche (son contexte, jamais la racine).
async function envoyerChoix() {
  const n = choix.value;
  if (!n) {
    if (aLaRacine.value || !niveau.value) return;
    envoyerBloc({ chemin: filtres(contexte.chemin), boite: contexte.boite, lecture: contexte.lecture }, niveau.value);
    return;
  }
  if (n.message) { envoyerUids([n.uid]); return; }
  if (n.demande) envoyerBloc(n.demande, n);
}

// envoyerBloc : demande et chiffres affiches ({ nombre, taille }).
async function envoyerBloc(demande, affiche) {
  try {
    // Chiffres affiches : un bloc qui ne leur correspond plus n'est pas envoye.
    const lot = await EnvoyerBlocPurgatoire(props.compte.id, { ...demande, nombre: affiche.nombre, taille: affiche.taille });
    if (lot.ecart) {
      // La synchro en arriere-plan a change ce bloc depuis son affichage :
      // rien n'est parti, la carte est rechargee pour montrer l'etat reel.
      avis.value = t('purgatoire.ecart');
      charger();
      chargerPurgatoire();
      return;
    }
    if (!lot.nombre) return;
    envoi.value = lot;
    apresChangement();
  } catch (e) { erreur.value = String(e); }
}

async function annulerEnvoi() {
  if (!envoi.value) return;
  try {
    await RetirerPurgatoire(props.compte.id, envoi.value.uids);
    envoi.value = null;
    apresChangement();
  } catch (e) { erreur.value = String(e); }
}

// Revue du Purgatoire : retrait de lignes ou de tout, annulation du dernier
// retrait, et mise a la corbeille de Gmail de tout le Purgatoire.
const revue = ref(false);
const retrait = ref(null); // dernier lot retire { uids, nombre, taille }

function basculerRevue() {
  revue.value = !revue.value;
  envoi.value = null;
  retrait.value = null;
}

async function retirer(appel) {
  if (corbeille.value) return; // pas de retrait pendant la mise a la corbeille
  try {
    const lot = await appel();
    if (!lot.nombre) return;
    retrait.value = lot;
    apresChangement();
  } catch (e) { erreur.value = String(e); }
}
const retirerUids = (uids) => uids?.length && retirer(() => RetirerPurgatoire(props.compte.id, uids));
const viderPurgatoire = () => retirer(() => ViderPurgatoire(props.compte.id));

async function annulerRetrait() {
  if (!retrait.value) return;
  try {
    await EnvoyerPurgatoire(props.compte.id, retrait.value.uids);
    retrait.value = null;
    apresChangement();
  } catch (e) { erreur.value = String(e); }
}

// Mise a la corbeille de Gmail de tout ce que la revue affiche (le nombre
// affiche est transmis : un Purgatoire qui a change n'est pas envoye). Les
// messages partis sortent de la carte ; ce qui reste apres une interruption
// est toujours au Purgatoire.
const corbeille = ref(null); // avancement { faits, total } pendant l'operation

async function mettreALaCorbeille() {
  const id = props.compte.id;
  const attendu = etatPurgatoire.value?.nombre || 0;
  if (!attendu || corbeille.value) return;
  corbeille.value = { faits: 0, total: attendu };
  avis.value = '';
  retrait.value = null;
  let r = null;
  let echec = '';
  try { r = await MettreALaCorbeille(id, attendu); } catch (e) { echec = String(e); }
  corbeille.value = null;
  if (props.compte.id !== id) return;
  // Rechargement d'abord : il efface l'erreur affichee, pas celle qui suit.
  apresChangement();
  if (echec) { erreur.value = echec; return; }
  if (r.code === 'ecart') { avis.value = t('corbeille.ecart'); return; }
  const morceaux = [];
  if (r.deplaces) morceaux.push(t(r.deplaces > 1 ? 'corbeille.faite' : 'corbeille.faite1', { n: fmtNombre(r.deplaces), taille: fmtTaille(r.taille) }));
  if (r.absents) morceaux.push(t('corbeille.absents', { n: fmtNombre(r.absents) }));
  if (r.ambigus) morceaux.push(t('corbeille.ambigus', { n: fmtNombre(r.ambigus) }));
  avis.value = morceaux.join(' ');
  if (r.code) {
    const message = r.code === 'autre' ? t('erreur.autre', { detail: r.detail }) : t('erreur.' + r.code);
    erreur.value = t('corbeille.interrompue', { message });
  }
}

// remonter(i) : revient au niveau i du fil d'Ariane. Sur le niveau courant,
// deselectionne le bloc choisi pour que le panneau montre de nouveau le niveau.
function remonter(i) {
  if (i >= chemin.value.length) { choix.value = null; return; }
  chemin.value = chemin.value.slice(0, i);
  charger();
}

function touche(ev) {
  if (ev.key !== 'Escape') return;
  if (revue.value) basculerRevue();
  else if (choix.value) choix.value = null;
  else if (chemin.value.length) remonter(chemin.value.length - 1);
}

watch(choix, chargerDetail);
watch(axes, () => { chemin.value = []; charger(); });
watch([metrique, boite, lecture], charger);
watch(() => props.compte.id, () => { chemin.value = []; envoi.value = null; retrait.value = null; avis.value = ''; charger(); chargerPurgatoire(); verifierMisDeCote(); });
watch(chemin, () => { envoi.value = null; });

// Synchro en arriere-plan (voir synchroAuto.js) : la carte reste affichee,
// le pied du flanc montre l'avancement ; a la fin, la carte n'est rechargee
// que si quelque chose a change. Cache vide (UIDVALIDITY changee) : retour a
// l'ecran de synchro.
const fond = ref(null); // { etat: 'encours' | 'erreur', faits, total, code, detail }
const desabonnements = [];

function synchroAuto() {
  const id = props.compte.id;
  if (synchronises.has(id)) {
    // Retour a la carte pendant une synchro lancee ailleurs : on la montre.
    SynchroEnCours(id).then((v) => { if (v && props.compte.id === id && !fond.value) fond.value = { etat: 'encours', faits: 0, total: 0 }; });
    return;
  }
  relancer(id);
}

// relancer : synchro en arriere-plan du compte id ; l'indicateur du pied ne
// concerne que le compte affiche.
function relancer(id) {
  synchronises.add(id);
  if (id === props.compte.id) fond.value = { etat: 'encours', faits: 0, total: 0 };
  LancerSynchro(id).catch((e) => { if (id === props.compte.id) fond.value = { etat: 'erreur', code: 'autre', detail: String(e) }; });
}

const messageFond = computed(() => {
  const f = fond.value;
  if (!f) return '';
  if (f.etat === 'encours') return f.total ? t('synchro.fondNouveaux', { n: fmtNombre(f.faits), total: fmtNombre(f.total) }) : t('synchro.fond');
  return t('synchro.fondErreur', { message: f.code === 'autre' ? t('erreur.autre', { detail: f.detail }) : t('erreur.' + f.code) });
});

function ecouterSynchro() {
  const pourMoi = (ev) => ev.compte === props.compte.id;
  desabonnements.push(EventsOn('synchro:progression', (ev) => {
    if (pourMoi(ev) && fond.value?.etat === 'encours' && ev.etape === 'entetes') fond.value = { ...fond.value, faits: ev.faits, total: ev.total };
  }));
  desabonnements.push(EventsOn('synchro:terminee', (ev) => {
    if (!pourMoi(ev)) return;
    fond.value = null;
    if (ev.lus || ev.supprimes || ev.actualises) apresChangement();
    emit('recharger'); // date de derniere synchro
  }));
  desabonnements.push(EventsOn('synchro:erreur', (ev) => {
    if (!pourMoi(ev)) return;
    fond.value = ev.code === 'annulee' ? null : { etat: 'erreur', code: ev.code, detail: ev.detail };
  }));
  desabonnements.push(EventsOn('synchro:vidage', (ev) => { if (pourMoi(ev)) emit('recharger'); }));
  desabonnements.push(EventsOn('purgatoire:misdecote', (ev) => { if (pourMoi(ev)) verifierMisDeCote(); }));
  desabonnements.push(EventsOn('corbeille:progression', (ev) => { if (pourMoi(ev) && corbeille.value) corbeille.value = { faits: ev.faits, total: ev.total }; }));
}

watch(() => props.compte.id, () => { fond.value = null; synchroAuto(); });

onMounted(() => {
  charger();
  chargerPurgatoire();
  verifierMisDeCote();
  window.addEventListener('keydown', touche);
  ecouterSynchro();
  synchroAuto();
});
onUnmounted(() => {
  window.removeEventListener('keydown', touche);
  desabonnements.forEach((f) => f());
});

// Teinte d'un bloc du niveau affiche (meme regle que la carte).
function teinteDe(noeud) {
  if (!noeud || noeud.message) return '';
  const rang = chemin.value.length ? (affiche.value?.noeuds || []).findIndex((n) => n.cle === noeud.cle) : -1;
  return couleurBloc(noeud, rang);
}
// Teinte du panneau : le bloc choisi, sinon le bloc dans lequel on est entre.
const teinteDetail = computed(() => {
  if (choix.value) return teinteDe(choix.value);
  return chemin.value.length ? chemin.value[chemin.value.length - 1].teinte || '' : '';
});

const titreDetail = computed(() => {
  if (choix.value) return libelleValeur(niveau.value.axe, choix.value);
  return chemin.value.length ? chemin.value[chemin.value.length - 1].libelle : t('carte.tous');
});
const sousTitreDetail = computed(() => {
  if (!choix.value) return t('detail.niveau');
  return niveau.value.axe === 'message' ? t('axe.message') : t('axe.' + niveau.value.axe);
});
const part = computed(() => {
  if (!choix.value || !niveau.value) return 0;
  const total = metrique.value === 'nombre' ? niveau.value.nombre : niveau.value.taille;
  return total ? valeurMetrique(choix.value, metrique.value) / total : 0;
});
</script>

<template>
  <div class="coque">
    <nav v-show="flancVisible" id="flanc" class="flanc" :aria-label="t('compte.comptes')">
      <div class="marque"><Logo :taille="20" /><span>Purgatorius</span></div>
      <section>
        <h2>{{ t('compte.comptes') }}</h2>
        <div v-for="c in etat.comptes" :key="c.id" class="rangee" :class="{ actif: c.id === compte.id }" @contextmenu.prevent="menuCompte = c.id">
          <button type="button" class="ligne-compte" :aria-pressed="c.id === compte.id" @click="$emit('choisir', c.id)">
            <span class="point" :style="{ background: c.couleur }" aria-hidden="true" />
            <span class="textes"><span class="nom">{{ c.nom }}</span><span class="adresse">{{ c.adresse }}</span></span>
          </button>
          <button type="button" class="plus" :aria-label="t('menuCompte.ouvrir', { nom: c.nom })" :aria-expanded="menuCompte === c.id" @click="menuCompte = menuCompte === c.id ? '' : c.id">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><circle cx="5" cy="12" r="1.6" /><circle cx="12" cy="12" r="1.6" /><circle cx="19" cy="12" r="1.6" /></svg>
          </button>
          <MenuCompte v-if="menuCompte === c.id" :compte="c" @fermer="menuCompte = ''" @modifie="apresMenu" @supprime="apresMenu" />
        </div>
        <button type="button" class="ajouter" @click="$emit('ajouter')">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
          {{ t('compte.ajouter') }}
        </button>
      </section>
      <section v-if="!revue">
        <h2>{{ t('carte.surface') }}</h2>
        <div class="segments" role="radiogroup" :aria-label="t('carte.surface')">
          <button type="button" role="radio" :aria-checked="metrique === 'taille'" @click="metrique = 'taille'">{{ t('carte.taille') }}</button>
          <button type="button" role="radio" :aria-checked="metrique === 'nombre'" @click="metrique = 'nombre'">{{ t('carte.nombre') }}</button>
        </div>
      </section>
      <section v-if="!revue">
        <h2>{{ t('carte.boite') }}</h2>
        <div class="segments trois" role="radiogroup" :aria-label="t('carte.boite')">
          <button type="button" role="radio" :aria-checked="boite === ''" @click="boite = ''">{{ t('boite.tous') }}</button>
          <button type="button" role="radio" :aria-checked="boite === 'recu'" @click="boite = 'recu'">{{ t('boite.recus') }}</button>
          <button type="button" role="radio" :aria-checked="boite === 'envoye'" @click="boite = 'envoye'">{{ t('boite.envoyes') }}</button>
        </div>
      </section>
      <section v-if="!revue">
        <h2>{{ t('lecture.titre') }}</h2>
        <div class="segments trois" role="radiogroup" :aria-label="t('lecture.titre')">
          <button type="button" role="radio" :aria-checked="lecture === ''" @click="lecture = ''">{{ t('lecture.tous') }}</button>
          <button type="button" role="radio" :aria-checked="lecture === 'nonlu'" @click="lecture = 'nonlu'">{{ t('lecture.nonlus') }}</button>
          <button type="button" role="radio" :aria-checked="lecture === 'lu'" @click="lecture = 'lu'">{{ t('lecture.lus') }}</button>
        </div>
      </section>
      <PileAxes v-if="!revue" v-model="axes" />
      <div class="pied">
        <span v-if="fond?.etat === 'encours'" class="aide fond" role="status">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--acc)" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="12" r="8" stroke-dasharray="32 18" /></svg>
          {{ messageFond }}
        </span>
        <span v-else-if="fond?.etat === 'erreur'" class="aide fond-erreur" role="alert">{{ messageFond }}</span>
        <span v-else class="aide">{{ t('compte.derniere', { date: fmtDate(compte.derniereSynchro) }) }}</span>
        <button type="button" class="btn btn-secondaire" @click="$emit('synchroniser')">{{ t('compte.resynchroniser') }}</button>
      </div>
    </nav>
    <main>
      <div class="entete">
        <span v-if="!flancVisible" class="marque marque-entete"><Logo :taille="20" /><span>Purgatorius</span></span>
        <button type="button" class="bascule" aria-controls="flanc" :aria-expanded="flancVisible" :aria-label="t(flancVisible ? 'panneaux.masquerFlanc' : 'panneaux.afficherFlanc')" :title="t(flancVisible ? 'panneaux.masquerFlanc' : 'panneaux.afficherFlanc')" @click="flancVisible = !flancVisible">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="3" /><path d="M9 4v16" /><path v-if="flancVisible" d="M5.5 8h1M5.5 11h1M5.5 14h1" stroke-linecap="round" /></svg>
        </button>
        <div class="titre">
          <div v-if="revue" class="ligne-titre">
            <button type="button" class="retour" :aria-label="t('revue.retourCarte')" @click="basculerRevue">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 6l-6 6 6 6" /></svg>{{ t('revue.carte') }}
            </button>
            <h1>{{ t('purgatoire.titre') }}</h1>
          </div>
          <nav v-else class="ariane" :aria-label="t('carte.ariane')">
            <button type="button" :class="{ courant: !chemin.length }" :aria-current="chemin.length ? undefined : 'location'" :disabled="!chemin.length && !choix" @click="remonter(0)">{{ t('carte.tous') }}</button>
            <template v-for="(f, i) in chemin" :key="i">
              <span aria-hidden="true" class="sep">›</span>
              <button type="button" :class="{ courant: i === chemin.length - 1 }" :aria-current="i === chemin.length - 1 ? 'location' : undefined" :disabled="i === chemin.length - 1 && !choix" @click="remonter(i + 1)">{{ f.libelle }}</button>
            </template>
            <template v-if="niveau && niveau.axe !== 'message'">
              <span aria-hidden="true" class="sep">›</span>
              <span class="axe">{{ t('axe.' + niveau.axe) }}</span>
            </template>
          </nav>
          <span v-if="niveau && !enListe && !revue" class="mono total">{{ fmtTaille(niveau.taille) }} {{ t('carte.estimes') }} · {{ t('carte.nbMessages', { n: fmtNombre(niveau.nombre) }) }}</span>
        </div>
        <span v-if="envoi && !enListe && !revue" role="status" class="envoi">
          {{ t(envoi.nombre > 1 ? 'purgatoire.envoyes' : 'purgatoire.envoyes1', { n: fmtNombre(envoi.nombre), taille: fmtTaille(envoi.taille) }) }}
          <button type="button" class="annuler" @click="annulerEnvoi">{{ t('purgatoire.annuler') }}</button>
        </span>
        <span class="droite">
          <BoutonPurgatoire :etat="etatPurgatoire" :actif="revue" @deposer="envoyerUids" @ouvrir="basculerRevue" />
          <button type="button" class="bascule" aria-controls="detail" :aria-expanded="detailVisible" :aria-label="t(detailVisible ? 'panneaux.masquerDetail' : 'panneaux.afficherDetail')" :title="t(detailVisible ? 'panneaux.masquerDetail' : 'panneaux.afficherDetail')" @click="detailVisible = !detailVisible">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="3" /><path d="M15 4v16" /><path v-if="detailVisible" d="M17.5 8h1M17.5 11h1M17.5 14h1" stroke-linecap="round" /></svg>
          </button>
        </span>
      </div>
      <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
      <div v-if="avis" class="avis" role="status">
        <span>{{ avis }}</span>
        <button type="button" class="annuler" @click="avis = ''">{{ t('commun.compris') }}</button>
      </div>
      <div class="carte">
        <ListeMessages v-if="revue" key="revue" purgatoire :compte-id="compte.id" :chemin="[]"
          :version="versionListe" :envoi="retrait" @retirer="retirerUids" @annuler="annulerRetrait" />
        <ListeMessages v-else-if="niveau && niveau.axe === 'message'" key="liste" :compte-id="compte.id" :chemin="filtres(chemin)" :boite="boite" :lecture="lecture"
          :choisi="choix?.uid || 0" :version="versionListe" :envoi="envoi" @choisir="choisirMessage" @envoyer="envoyerUids" @annuler="annulerEnvoi" />
        <CarteCanvas v-else-if="affiche" :par-rang="chemin.length > 0" :niveau="affiche" :metrique="metrique" :choisi="choix?.cle || ''" @choisir="selectionner" @entrer="entrer" @taille="aire = $event" />
      </div>
    </main>
    <BilanPurgatoire v-if="revue" v-show="detailVisible" id="detail" :etat="etatPurgatoire" :avancement="corbeille" @vider="viderPurgatoire" @corbeille="mettreALaCorbeille" />
    <PanneauDetail v-else v-show="detailVisible" id="detail" :detail="detail" :titre="titreDetail" :sous-titre="sousTitreDetail" :part="part"
      :adresse="compte.adresse" :teinte="teinteDetail" :envoyable="envoyable" @envoyer="envoyerChoix" />
  </div>
</template>

<style scoped>
.coque { height: 100vh; display: flex; overflow: hidden; }
.flanc { flex: 0 0 248px; background: var(--flanc); border-right: 1px solid var(--trait); padding: 16px 12px; box-sizing: border-box; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; }
.marque { display: flex; align-items: center; gap: 8px; min-height: 32px; padding: 0 8px; font-size: 16px; font-weight: 600; }
section { display: flex; flex-direction: column; gap: 4px; }
h2 { margin: 0 0 4px; padding: 0 8px; font-size: 13px; font-weight: 500; color: var(--muet); }
.rangee { position: relative; display: flex; align-items: center; border-radius: 8px; }
.rangee.actif { background: var(--releve); box-shadow: inset 0 0 0 1px var(--trait); }
.rangee .ligne-compte { flex: 1 1 auto; min-width: 0; }
.plus { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; margin-right: 4px; border: 0; border-radius: 6px; background: transparent; color: var(--muet); padding: 0; opacity: 0; }
.rangee:hover .plus, .rangee.actif .plus, .plus:focus-visible, .plus[aria-expanded="true"] { opacity: 1; }
.plus:hover { background: var(--carte); color: var(--txt); }
.marque-entete { padding: 0 4px 0 0; }
.bascule { display: inline-flex; align-items: center; justify-content: center; width: 32px; height: 32px; border: 0; border-radius: 8px; background: transparent; color: var(--sec); padding: 0; flex-shrink: 0; }
.bascule:hover { background: var(--releve); color: var(--txt); }
.droite { display: flex; align-items: center; gap: 8px; margin-left: auto; }
.ligne-compte { display: flex; align-items: center; gap: 12px; min-height: 44px; padding: 4px 8px; border-radius: 8px; border: 0; background: transparent; text-align: left; color: var(--txt); }
.ligne-compte[aria-pressed="true"] { background: transparent; }
.point { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.textes { display: flex; flex-direction: column; min-width: 0; }
.nom { font-weight: 500; }
.adresse { font-size: 12px; color: var(--sec); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ajouter { display: flex; align-items: center; gap: 12px; min-height: 36px; padding: 0 8px; border-radius: 8px; border: 0; background: transparent; color: var(--sec); }
.segments.trois { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.segments { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 4px; padding: 4px; border-radius: 10px; background: var(--releve); }
.segments button { min-height: 28px; border: 0; border-radius: 6px; background: transparent; color: var(--sec); font-size: 13px; }
.segments button[aria-checked="true"] { background: var(--carte); color: var(--txt); font-weight: 500; box-shadow: var(--ombre-petite); }
.pied { margin-top: auto; display: flex; flex-direction: column; gap: 8px; padding: 0 8px; }
.fond { display: inline-flex; align-items: center; gap: 8px; }
/* Seule boucle admise (unslop) : elle signale un traitement reellement en cours. */
.fond svg { flex-shrink: 0; animation: tourner 1s linear infinite; }
@keyframes tourner { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .fond svg { animation: none; } }
.fond-erreur { color: var(--erreur); }
main { flex: 1 1 auto; min-width: 0; padding: 16px 24px 24px; box-sizing: border-box; display: flex; flex-direction: column; gap: 16px; }
.entete { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; min-height: 32px; flex-shrink: 0; }
.titre { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.ligne-titre { display: flex; align-items: center; gap: 12px; }
.ariane { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 14px; min-width: 0; }
.ariane button { border: 0; background: transparent; padding: 0; color: var(--sec); font-size: 14px; max-width: 280px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ariane button.courant { color: var(--txt); font-weight: 500; opacity: 1; }
.ariane button:disabled { cursor: default; }
.sep { color: var(--muet); }
.retour { display: inline-flex; align-items: center; gap: 4px; min-height: 32px; padding: 0 8px 0 4px; border: 0; border-radius: 8px; background: transparent; color: var(--sec); font-size: 14px; }
.retour:hover { background: var(--releve); color: var(--txt); }
h1 { margin: 0; font-size: 16px; font-weight: 500; }
.axe { color: var(--muet); }
.total { font-size: 12px; color: var(--muet); }
.envoi { display: inline-flex; align-items: center; gap: 8px; font-size: 13px; }
.annuler { min-height: 32px; padding: 0 10px; border: 0; border-radius: 8px; background: transparent; color: var(--acc); font-size: 13px; font-weight: 500; box-shadow: inset 0 0 0 1px var(--trait); }
.carte { flex: 1 1 auto; min-height: 0; }
.erreur { margin: 0; padding: 12px; border-radius: 10px; background: var(--erreur-doux); color: var(--erreur); }
.avis { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 8px 8px 12px; border-radius: 10px; background: var(--releve); color: var(--txt); font-size: 13px; line-height: 20px; }
.avis .annuler { flex-shrink: 0; }
</style>
