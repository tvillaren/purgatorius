<script setup>
// Liste des messages du dernier niveau (lignes sur deux niveaux).
//
// Defilement virtualise, a la maniere de UITableView : lignes de hauteur fixe,
// un jeu constant d'elements DOM (cle = rang modulo la taille du jeu) que l'on
// repositionne et remplit a nouveau en defilant. L'index (UID et taille de
// chaque message, dans l'ordre) arrive d'un coup ; le contenu des lignes est
// demande par pages, a l'approche de la zone visible.
//
// Mode purgatoire (revue du Purgatoire) : les messages du Purgatoire ; un clic
// sur une ligne la selectionne (le panneau de droite montre le bilan, pas un
// message) et l'action d'une ligne ou de la selection les en retire.
import { computed, onMounted, onUnmounted, ref, shallowRef, triggerRef, watch } from 'vue';
import { IndexListe, LignesListe } from '../../wailsjs/go/app/App';
import { t } from '../i18n.js';
import { fmtJour, fmtNombre, fmtTaille } from '../format.js';
import { TYPE_GLISSE } from '../glisser.js';
import IconePurgatoire from './IconePurgatoire.vue';
import IconeRetirer from './IconeRetirer.vue';

const props = defineProps({
  compteId: { type: String, required: true },
  chemin: { type: Array, required: true }, // filtres { axe, valeur, exclure }
  boite: { type: String, default: '' },
  lecture: { type: String, default: '' },
  choisi: { type: Number, default: 0 }, // UID du message affiche dans le detail
  version: { type: Number, default: 0 }, // change quand le Purgatoire change : liste relue
  envoi: { type: Object, default: null }, // dernier envoi au Purgatoire (ou retrait, en revue) { nombre, taille }, annulable
  purgatoire: { type: Boolean, default: false }, // revue du Purgatoire
});
const emit = defineEmits(['choisir', 'envoyer', 'retirer', 'annuler']);

const HAUTEUR = 60; // px par ligne
const MARGE = 6; // lignes rendues au-dela de la zone visible, de chaque cote
const PAGE = 100; // lignes demandees par lot

const tri = ref('taille');
const desc = ref(true);
const index = shallowRef({ uids: [], tailles: [] });
const lignes = shallowRef(new Map()); // uid -> ligne chargee
const pagesDemandees = new Set();
const erreur = ref('');
const zone = ref(null);
const fantome = ref(null); // image glissee : « 12 messages »
const haut = ref(0); // scrollTop
const vue = ref(600); // hauteur visible
const coches = ref(new Set());
let ancre = -1; // dernier rang coche, pour Maj+clic
let jeton = 0;

const envoyes = computed(() => props.boite === 'envoye');
const tris = computed(() => ['taille', 'date', envoyes.value ? 'destinataire' : 'expediteur', 'pj']);
const total = computed(() => index.value.uids.length);
const tailleTotale = computed(() => index.value.tailles.reduce((s, x) => s + x, 0));
const tailleMax = computed(() => {
  let m = 1;
  for (const x of index.value.tailles) if (x > m) m = x;
  return m;
});
const tailleParUid = computed(() => {
  const m = new Map();
  index.value.uids.forEach((u, i) => m.set(u, index.value.tailles[i]));
  return m;
});

// garder : la meme liste, relue (envoi au Purgatoire) ; on reste a la meme
// hauteur au lieu de revenir en haut.
async function chargerIndex(garder = false) {
  const n = ++jeton;
  erreur.value = '';
  try {
    const r = await IndexListe(props.compteId, {
      chemin: props.purgatoire ? [] : props.chemin, boite: props.boite, lecture: props.lecture, tri: tri.value, desc: desc.value, purgatoire: props.purgatoire,
    });
    if (n !== jeton) return;
    index.value = r;
    lignes.value = new Map();
    pagesDemandees.clear();
    if (!garder) {
      if (zone.value) zone.value.scrollTop = 0;
      haut.value = 0;
    }
    demanderVisibles();
  } catch (e) {
    erreur.value = String(e);
  }
}

// Pages couvrant la zone visible (marge comprise) et pas encore demandees.
function demanderVisibles() {
  const { debut, fin } = fenetre.value;
  for (let p = Math.floor(debut / PAGE); p * PAGE < fin; p++) {
    if (pagesDemandees.has(p)) continue;
    pagesDemandees.add(p);
    chargerPage(p, jeton);
  }
}

async function chargerPage(p, n) {
  const uids = index.value.uids.slice(p * PAGE, (p + 1) * PAGE);
  try {
    const r = await LignesListe(props.compteId, uids);
    if (n !== jeton) return;
    for (const l of r) lignes.value.set(l.uid, l);
    triggerRef(lignes);
  } catch (e) {
    pagesDemandees.delete(p);
    erreur.value = String(e);
  }
}

const fenetre = computed(() => {
  const debut = Math.max(0, Math.floor(haut.value / HAUTEUR) - MARGE);
  const fin = Math.min(total.value, Math.ceil((haut.value + vue.value) / HAUTEUR) + MARGE);
  return { debut, fin };
});
// Taille du jeu d'elements : constante pour une hauteur de vue donnee.
const jeu = computed(() => Math.ceil(vue.value / HAUTEUR) + 2 * MARGE + 1);

const visibles = computed(() => {
  const out = [];
  const { debut, fin } = fenetre.value;
  for (let i = debut; i < fin; i++) {
    const uid = index.value.uids[i];
    out.push({ rang: i, cle: i % jeu.value, uid, taille: index.value.tailles[i], l: lignes.value.get(uid) || null });
  }
  return out;
});

function defiler() {
  haut.value = zone.value.scrollTop;
  demanderVisibles();
}

let observateur = null;
onMounted(() => {
  observateur = new ResizeObserver(() => { if (zone.value) { vue.value = zone.value.clientHeight; demanderVisibles(); } });
  observateur.observe(zone.value);
  chargerIndex();
});
onUnmounted(() => observateur?.disconnect());

// Changement de filtre : nouvelle liste, selection videe. Changement de tri :
// meme ensemble, la selection reste.
watch(() => JSON.stringify([props.compteId, props.chemin, props.boite, props.lecture]), () => { coches.value = new Set(); ancre = -1; chargerIndex(); });
watch([tri, desc], () => { ancre = -1; chargerIndex(); });
watch(() => props.version, () => { coches.value = new Set(); ancre = -1; chargerIndex(true); });
watch(envoyes, (v) => { if (tri.value === (v ? 'expediteur' : 'destinataire')) tri.value = v ? 'destinataire' : 'expediteur'; });

function trier(cle) {
  if (tri.value === cle) desc.value = !desc.value;
  else { tri.value = cle; desc.value = cle !== 'expediteur' && cle !== 'destinataire'; }
}

// Selection
const nbSel = computed(() => coches.value.size);
const tailleSel = computed(() => { let s = 0; for (const u of coches.value) s += tailleParUid.value.get(u) || 0; return s; });
const tout = computed(() => total.value > 0 && nbSel.value === total.value);
const partiel = computed(() => nbSel.value > 0 && !tout.value);

function cocher(rang, ev) {
  const uids = index.value.uids;
  const s = new Set(coches.value);
  const val = !s.has(uids[rang]);
  if (ev.shiftKey && ancre >= 0 && ancre < uids.length) {
    for (let i = Math.min(ancre, rang); i <= Math.max(ancre, rang); i++) {
      if (val) s.add(uids[i]); else s.delete(uids[i]);
    }
  } else if (val) s.add(uids[rang]); else s.delete(uids[rang]);
  ancre = rang;
  coches.value = s;
}
function toutCocher() { coches.value = new Set(index.value.uids); }
function effacer() { coches.value = new Set(); ancre = -1; }
function basculerTout() { if (tout.value) effacer(); else toutCocher(); }

function agirSelection() { emit(props.purgatoire ? 'retirer' : 'envoyer', [...coches.value]); }
function agirLigne(v) { emit(props.purgatoire ? 'retirer' : 'envoyer', [v.uid]); }
const texteAnnonce = computed(() => {
  const e = props.envoi;
  if (!e) return '';
  const cle = (props.purgatoire ? 'revue.retires' : 'purgatoire.envoyes') + (e.nombre > 1 ? '' : '1');
  return t(cle, { n: fmtNombre(e.nombre), taille: fmtTaille(e.taille) });
});

// Glisser une ligne : toute la selection si la ligne en fait partie.
function glisser(v, ev) {
  const uids = coches.value.has(v.uid) ? [...coches.value] : [v.uid];
  ev.dataTransfer.setData(TYPE_GLISSE, JSON.stringify(uids));
  ev.dataTransfer.effectAllowed = 'move';
  if (uids.length > 1 && fantome.value) {
    fantome.value.textContent = t('purgatoire.glisses', { n: fmtNombre(uids.length) });
    ev.dataTransfer.setDragImage(fantome.value, 16, 16);
  }
}

function choisir(v, ev) {
  if (!v.l) return;
  if (props.purgatoire) cocher(v.rang, ev);
  else emit('choisir', v.l);
}

function personne(l) {
  if (l.boite === 'envoye') return { nom: t('liste.a', { dest: l.destinataire || '?' }), adresse: '' };
  return { nom: l.nom || l.expediteur, adresse: l.nom ? l.expediteur : '' };
}
const largeurBarre = (taille) => Math.max(4, (taille / tailleMax.value) * 100) + '%';

defineExpose({ effacer });
</script>

<template>
  <section class="liste" :aria-label="t('liste.titre')">
    <div class="cadre">
      <div class="barre" :class="{ selection: nbSel > 0 }">
        <span class="case"><input type="checkbox" :checked="tout" :indeterminate="partiel" :aria-label="t('liste.caseTout')" @change="basculerTout"></span>
        <template v-if="nbSel > 0">
          <div class="infos">
            <span class="fort">{{ t(nbSel > 1 ? 'liste.selection' : 'liste.selection1', { n: fmtNombre(nbSel) }) }}</span>
            <span class="mono sec">{{ fmtTaille(tailleSel) }}</span>
            <button v-if="!tout" type="button" class="lien" @click="toutCocher">{{ t('liste.toutSelectionner', { n: fmtNombre(total) }) }}</button>
            <button type="button" class="lien discret" @click="effacer">{{ t('liste.deselectionner') }}</button>
          </div>
          <button v-if="purgatoire" type="button" class="btn btn-secondaire envoyer" @click="agirSelection">
            <IconeRetirer />{{ t('revue.retirer') }}
          </button>
          <button v-else type="button" class="btn btn-principal envoyer" @click="agirSelection">
            <IconePurgatoire />{{ t('purgatoire.envoyer') }}
          </button>
        </template>
        <template v-else>
          <div class="infos">
            <span v-if="envoi" role="status" class="envoi">
              <span>{{ texteAnnonce }}</span>
              <button type="button" class="annuler" @click="emit('annuler')">{{ t('purgatoire.annuler') }}</button>
            </span>
            <span v-else class="mono sec">{{ t('carte.nbMessages', { n: fmtNombre(total) }) }} · {{ fmtTaille(tailleTotale) }}</span>
          </div>
        </template>
        <div v-if="nbSel === 0" class="tri">
          <span class="muet">{{ t('liste.trierPar') }}</span>
          <div class="segments" role="radiogroup" :aria-label="t('liste.trierPar')">
            <button v-for="c in tris" :key="c" type="button" role="radio" :aria-checked="tri === c" @click="trier(c)">
              {{ t('liste.tri.' + c) }}<span v-if="tri === c" class="mono" aria-hidden="true">{{ desc ? ' ↓' : ' ↑' }}</span>
            </button>
          </div>
        </div>
      </div>
      <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
      <div ref="zone" class="zone" role="list" :aria-label="t('liste.titre')" @scroll.passive="defiler">
        <p v-if="!total && !erreur" class="vide">{{ t(purgatoire ? 'revue.vide' : 'liste.vide') }}</p>
        <div class="plan" :style="{ height: total * HAUTEUR + 'px' }">
          <div v-for="v in visibles" :key="v.cle" class="ligne" role="listitem" :draggable="!!v.l && !purgatoire" @dragstart="glisser(v, $event)" :aria-setsize="total" :aria-posinset="v.rang + 1"
            :class="{ choisie: v.uid === choisi, cochee: coches.has(v.uid) }" :style="{ transform: 'translateY(' + v.rang * HAUTEUR + 'px)' }">
            <span class="case"><input type="checkbox" :checked="coches.has(v.uid)" :aria-label="t('liste.selectionner', { objet: v.l?.objet || '' })" @click="cocher(v.rang, $event)"></span>
            <span class="point"><span v-if="v.l && !v.l.lu" aria-hidden="true" /></span>
            <button v-if="v.l" type="button" class="corps" @click="choisir(v, $event)">
              <span class="haut">
                <span class="qui" :class="{ nonlu: !v.l.lu }">{{ personne(v.l).nom }}</span>
                <span v-if="personne(v.l).adresse" class="adresse">{{ personne(v.l).adresse }}</span>
              </span>
              <span class="date">{{ fmtJour(v.l.date) }}</span>
              <span class="bas">
                <span class="objet">{{ v.l.objet || t('carte.sansObjet') }}</span>
                <span v-for="lb in v.l.libelles.slice(0, 3)" :key="lb" class="libelle">{{ lb }}</span>
              </span>
              <span class="poids">
                <span v-if="v.l.pj" class="pj mono">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 11.5l-8.2 8.2a5 5 0 01-7.1-7.1l8.6-8.6a3.4 3.4 0 014.8 4.8l-8.6 8.6a1.7 1.7 0 01-2.4-2.4l7.9-7.9" /></svg>{{ fmtTaille(v.l.pj) }}
                </span>
                <span class="jauge" aria-hidden="true"><span :style="{ width: largeurBarre(v.taille) }" /></span>
                <span class="mono taille">{{ fmtTaille(v.taille) }}</span>
              </span>
            </button>
            <span v-else class="corps attente" aria-hidden="true"><span /><span /></span>
            <button v-if="v.l" type="button" class="action" :title="t(purgatoire ? 'revue.retirer' : 'purgatoire.envoyer')"
              :aria-label="t(purgatoire ? 'revue.retirerLigne' : 'purgatoire.envoyerLigne', { objet: v.l.objet || t('carte.sansObjet') })" @click="agirLigne(v)">
              <IconeRetirer v-if="purgatoire" :taille="18" />
              <IconePurgatoire v-else :taille="18" />
            </button>
          </div>
        </div>
      </div>
    </div>
    <p class="aide">{{ t(purgatoire ? 'revue.aide' : 'liste.aide') }}</p>
    <div ref="fantome" class="fantome" aria-hidden="true" />
  </section>
</template>

<style scoped>
.liste { height: 100%; display: flex; flex-direction: column; gap: 8px; min-height: 0; }
.cadre { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: column; border-radius: 12px; background: var(--carte); box-shadow: inset 0 0 0 1px var(--trait); overflow: hidden; }
.barre { display: flex; align-items: center; gap: 8px 12px; min-height: 52px; padding: 8px 12px 8px 0; box-sizing: border-box; border-bottom: 1px solid var(--trait); flex-wrap: wrap; flex-shrink: 0; }
.barre .tri { margin-left: auto; }
.barre.selection { background: var(--acc-selection); }
.envoyer { display: inline-flex; align-items: center; gap: 8px; min-height: 36px; padding: 0 14px; white-space: nowrap; }
.envoi { display: inline-flex; align-items: center; gap: 8px; min-width: 0; }
.envoi > span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.annuler { min-height: 32px; padding: 0 10px; border: 0; border-radius: 8px; background: transparent; color: var(--acc); font-size: 13px; font-weight: 500; box-shadow: inset 0 0 0 1px var(--trait); }
.action { flex: 0 0 36px; height: 36px; margin: 0 6px; display: flex; align-items: center; justify-content: center; border: 0; border-radius: 8px; background: transparent; color: var(--sec); padding: 0; opacity: 0; }
.ligne:hover .action, .action:focus-visible { opacity: 1; }
.action:hover { background: var(--carte); color: var(--txt); }
.ligne[draggable="true"] { cursor: grab; }
.fantome { position: fixed; top: -100px; left: 0; padding: 6px 12px; border-radius: 8px; background: var(--txt); color: var(--bg); font-size: 13px; font-weight: 500; white-space: nowrap; }
.case { flex: 0 0 40px; display: flex; justify-content: center; }
.case input { width: 16px; height: 16px; margin: 0; accent-color: var(--acc); }
.infos { flex: 1 1 auto; max-width: calc(100% - 52px); min-width: 0; display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.fort { font-weight: 500; }
.sec { color: var(--sec); font-size: 12px; }
.muet { color: var(--muet); font-size: 12px; }
.lien { border: 0; background: transparent; padding: 0 4px; min-height: 32px; font-size: 13px; color: var(--acc); }
.lien.discret { color: var(--sec); }
.tri { display: flex; align-items: center; gap: 8px; }
.segments { display: flex; gap: 2px; padding: 2px; border-radius: 8px; background: var(--releve); }
.segments button { min-height: 28px; padding: 0 8px; border: 0; border-radius: 6px; background: transparent; color: var(--sec); font-size: 12px; white-space: nowrap; }
.segments button[aria-checked="true"] { background: var(--carte); color: var(--txt); font-weight: 500; box-shadow: var(--ombre-petite); }
.zone { position: relative; flex: 1 1 auto; min-height: 0; overflow-y: auto; }
.plan { position: relative; }
.vide { margin: 0; padding: 24px; color: var(--sec); }
.ligne { position: absolute; top: 0; left: 0; right: 0; height: 60px; display: flex; align-items: center; border-bottom: 1px solid var(--trait); box-sizing: border-box; will-change: transform; }
.ligne:hover { background: var(--releve); }
.ligne.cochee, .ligne.choisie { background: var(--acc-doux); }
.point { flex: 0 0 12px; display: flex; justify-content: center; }
.point span { width: 8px; height: 8px; border-radius: 50%; background: var(--acc); }
.corps { flex: 1 1 auto; min-width: 0; height: 100%; display: grid; grid-template-columns: minmax(0, 1fr) auto; grid-template-rows: 20px 20px; column-gap: 24px; row-gap: 2px; align-content: center; padding: 0 8px; border: 0; background: transparent; color: inherit; font: inherit; text-align: left; cursor: default; }
.corps:focus-visible { outline: 2px solid var(--acc); outline-offset: -2px; }
.haut, .bas { display: flex; align-items: baseline; gap: 8px; min-width: 0; overflow: hidden; white-space: nowrap; }
.bas { align-items: center; }
.qui { flex-shrink: 0; max-width: 60%; overflow: hidden; text-overflow: ellipsis; }
.qui.nonlu { font-weight: 600; }
.adresse { font-size: 12px; color: var(--muet); overflow: hidden; text-overflow: ellipsis; }
.date { font-size: 12px; color: var(--sec); text-align: right; white-space: nowrap; }
.objet { overflow: hidden; text-overflow: ellipsis; color: var(--sec); }
.libelle { flex-shrink: 0; padding: 0 6px; border-radius: 4px; background: var(--releve); font-size: 11px; line-height: 18px; color: var(--sec); }
.poids { display: flex; align-items: center; justify-content: flex-end; gap: 8px; white-space: nowrap; }
.pj { display: inline-flex; align-items: center; gap: 4px; font-size: 12px; color: var(--sec); }
.jauge { display: block; width: 64px; height: 4px; border-radius: 2px; background: var(--releve); overflow: hidden; }
.jauge span { display: block; height: 100%; border-radius: 2px; background: var(--acc); }
.taille { min-width: 56px; font-size: 12px; text-align: right; }
.attente { display: flex; flex-direction: column; justify-content: center; gap: 8px; }
.attente span { display: block; height: 10px; width: 40%; border-radius: 4px; background: var(--releve); }
.attente span + span { width: 65%; }
.erreur { margin: 8px; padding: 12px; border-radius: 10px; background: var(--erreur-doux); color: var(--erreur); }
.aide { margin: 0; }
</style>
