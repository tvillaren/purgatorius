<script setup>
// Treemap dessine en Canvas : disposition squarified a rangees horizontales
// (disposition.js), blocs aux coins arrondis, nom et valeur quand la place le
// permet. Une liste de boutons masquee visuellement double le dessin pour le
// clavier et les lecteurs d'ecran.
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue';
import { hierarchy, treemap } from 'd3-hierarchy';
import { tuileHorizontale } from '../disposition.js';
import { couleurBloc, encreSur } from '../couleurs.js';
import { libelleValeur, valeurMetrique } from '../axes.js';
import { fmtNombre, fmtTaille } from '../format.js';
import { langue } from '../i18n.js';
import { t } from '../i18n.js';

const props = defineProps({
  niveau: { type: Object, default: null },
  metrique: { type: String, default: 'taille' },
  choisi: { type: String, default: '' }, // cle du bloc selectionne
  parRang: { type: Boolean, default: false }, // sous-niveau : teintes selon le rang (voir couleurs.js)
});
// survol : mise en evidence legere ; choisir (clic) : selection, le panneau
// de detail s'y fixe ; entrer (double-clic) : zoom dans le bloc.
const emit = defineEmits(['survol', 'choisir', 'entrer', 'taille']);

const boite = ref(null);
const toile = ref(null);
const largeur = ref(0);
const hauteur = ref(0);
const survole = ref(-1);
const bulle = ref(null); // infobulle : { x, y, gauche, noeud }
let blocs = [];
let observateur = null;
let media = null;

const noeuds = computed(() => props.niveau?.noeuds || []);

function texteValeur(n) {
  return props.metrique === 'nombre'
    ? t('carte.nbMessages', { n: fmtNombre(n.nombre) })
    : fmtTaille(n.taille);
}

function disposer() {
  const racine = hierarchy({ children: noeuds.value })
    .sum((d) => (d.children ? 0 : Math.max(valeurMetrique(d, props.metrique), 0)));
  treemap().tile(tuileHorizontale).size([largeur.value, hauteur.value]).paddingInner(2).round(true)(racine);
  const rangs = new Map(noeuds.value.map((n, i) => [n, i]));
  blocs = racine.leaves().filter((f) => f.x1 - f.x0 > 0 && f.y1 - f.y0 > 0).map((f) => ({
    x: f.x0, y: f.y0, l: f.x1 - f.x0, h: f.y1 - f.y0, noeud: f.data,
    fond: couleurBloc(f.data, props.parRang ? rangs.get(f.data) : -1),
  }));
}

function tronquer(ctx, texte, max) {
  if (ctx.measureText(texte).width <= max) return texte;
  let bas = 0;
  let haut = texte.length;
  while (bas < haut) {
    const m = Math.ceil((bas + haut) / 2);
    if (ctx.measureText(texte.slice(0, m) + '…').width <= max) bas = m; else haut = m - 1;
  }
  return bas > 0 ? texte.slice(0, bas) + '…' : '';
}

function dessiner() {
  const c = toile.value;
  if (!c || !largeur.value || !hauteur.value) return;
  const ratio = window.devicePixelRatio || 1;
  c.width = Math.round(largeur.value * ratio);
  c.height = Math.round(hauteur.value * ratio);
  const ctx = c.getContext('2d');
  ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
  ctx.clearRect(0, 0, largeur.value, hauteur.value);
  const css = getComputedStyle(document.documentElement);
  const accent = css.getPropertyValue('--acc').trim();
  const filet = css.getPropertyValue('--champ-bord').trim();
  blocs.forEach((b, i) => {
    const r = Math.min(8, b.l / 4, b.h / 4);
    ctx.beginPath();
    ctx.roundRect(b.x, b.y, b.l, b.h, r);
    const fond = b.fond;
    ctx.fillStyle = fond;
    ctx.fill();
    const choisi = b.noeud.cle === props.choisi;
    if (choisi || i === survole.value) {
      ctx.save();
      ctx.beginPath();
      ctx.roundRect(b.x + 1, b.y + 1, b.l - 2, b.h - 2, Math.max(0, r - 1));
      ctx.lineWidth = 2;
      ctx.strokeStyle = choisi ? accent : filet;
      ctx.stroke();
      ctx.restore();
    }
    if (b.l < 48 || b.h < 26) return;
    ctx.fillStyle = encreSur(fond);
    ctx.textBaseline = 'top';
    ctx.font = '500 13px "Inter Variable", Inter, system-ui, sans-serif';
    const nom = tronquer(ctx, libelleValeur(props.niveau.axe, b.noeud), b.l - 24);
    ctx.fillText(nom, b.x + 12, b.y + 8);
    if (b.h >= 46) {
      ctx.font = '400 12px "IBM Plex Mono", ui-monospace, monospace';
      ctx.globalAlpha = 0.85;
      ctx.fillText(tronquer(ctx, texteValeur(b.noeud), b.l - 24), b.x + 12, b.y + 26);
      ctx.globalAlpha = 1;
    }
  });
}

function rafraichir() {
  disposer();
  dessiner();
}

function blocSous(ev) {
  const rect = toile.value.getBoundingClientRect();
  const x = ev.clientX - rect.left;
  const y = ev.clientY - rect.top;
  return blocs.findIndex((b) => x >= b.x && x < b.x + b.l && y >= b.y && y < b.y + b.h);
}

function bouger(ev) {
  const i = blocSous(ev);
  const rect = toile.value.getBoundingClientRect();
  const x = ev.clientX - rect.left;
  const y = ev.clientY - rect.top;
  bulle.value = i >= 0 ? { x, y, gauche: x > largeur.value - 240, haut: y > hauteur.value - 80, noeud: blocs[i].noeud } : null;
  if (i === survole.value) return;
  survole.value = i;
  dessiner();
  emit('survol', i >= 0 ? blocs[i].noeud : null);
}

function quitter() {
  survole.value = -1;
  bulle.value = null;
  dessiner();
  emit('survol', null);
}

function cliquer(ev) {
  const i = blocSous(ev);
  emit('choisir', i >= 0 ? blocs[i].noeud : null);
}

function doubleCliquer(ev) {
  const i = blocSous(ev);
  if (i >= 0) emit('entrer', blocs[i].noeud);
}

function dateCourte(s) {
  return s ? new Date(s * 1000).toLocaleDateString(langue.value === 'fr' ? 'fr-FR' : 'en-GB', { dateStyle: 'medium' }) : '';
}

const curseur = computed(() => (props.niveau?.axe === 'message' ? 'default' : 'pointer'));

onMounted(async () => {
  observateur = new ResizeObserver(([e]) => {
    largeur.value = Math.floor(e.contentRect.width);
    hauteur.value = Math.floor(e.contentRect.height);
    emit('taille', largeur.value * hauteur.value);
    rafraichir();
  });
  observateur.observe(boite.value);
  media = window.matchMedia('(prefers-color-scheme: dark)');
  media.addEventListener('change', dessiner);
  await document.fonts.ready;
  await nextTick();
  rafraichir();
});

onUnmounted(() => {
  observateur?.disconnect();
  media?.removeEventListener('change', dessiner);
});

watch(() => [props.niveau, props.metrique], () => { survole.value = -1; rafraichir(); });
watch(() => props.choisi, dessiner);
</script>

<template>
  <div ref="boite" class="boite">
    <canvas ref="toile" aria-hidden="true" :style="{ cursor: curseur }" @mousemove="bouger" @mouseleave="quitter" @click="cliquer" @dblclick="doubleCliquer" />
    <div v-if="bulle" class="bulle" aria-hidden="true"
      :style="{ left: bulle.x + 'px', top: bulle.y + 'px', transform: 'translate(' + (bulle.gauche ? 'calc(-100% - 12px)' : '12px') + ',' + (bulle.haut ? 'calc(-100% - 12px)' : '12px') + ')' }">
      <span class="bulle-nom">{{ libelleValeur(niveau.axe, bulle.noeud) }}</span>
      <span class="mono bulle-valeurs">{{ fmtTaille(bulle.noeud.taille) }} · {{ bulle.noeud.message ? dateCourte(bulle.noeud.date) : t('carte.nbMessages', { n: fmtNombre(bulle.noeud.nombre) }) }}</span>
    </div>
    <ul class="sr-only" :aria-label="t('carte.blocs')">
      <li v-for="n in noeuds" :key="n.cle">
        <button type="button" :aria-pressed="n.cle === choisi" @click="emit('choisir', n)" @keydown.enter.prevent="emit('entrer', n)">
          {{ libelleValeur(niveau.axe, n) }}, {{ fmtTaille(n.taille) }}, {{ t('carte.nbMessages', { n: fmtNombre(n.nombre) }) }}
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.boite { position: relative; width: 100%; height: 100%; min-height: 320px; }
canvas { position: absolute; inset: 0; width: 100%; height: 100%; display: block; }
.bulle { position: absolute; z-index: 2; pointer-events: none; max-width: 280px; display: flex; flex-direction: column; gap: 2px; padding: 8px 12px; border-radius: 8px; background: var(--carte); color: var(--txt); box-shadow: inset 0 0 0 1px var(--trait), 0 8px 24px rgba(0, 0, 0, 0.12); }
.bulle-nom { font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
.bulle-valeurs { font-size: 12px; color: var(--sec); white-space: nowrap; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
.sr-only:focus-within { position: absolute; left: 8px; top: 8px; width: auto; height: auto; clip: auto; margin: 0; padding: 8px; background: var(--carte); border-radius: 8px; box-shadow: var(--ombre); z-index: 1; }
.sr-only button { background: transparent; border: 0; color: var(--txt); }
</style>
