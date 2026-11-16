<script setup>
// Panneau de detail : le bloc survole, sinon le niveau courant. Chiffres en
// chasse fixe, repartition par annee (barres neutres, annee choisie en
// accent), principaux expediteurs, pieces jointes.
import { computed, ref, watch } from 'vue';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { t } from '../i18n.js';
import { fmtNombre, fmtTaille } from '../format.js';
import IconePurgatoire from './IconePurgatoire.vue';

const props = defineProps({
  detail: { type: Object, default: null },
  titre: { type: String, default: '' },
  sousTitre: { type: String, default: '' },
  part: { type: Number, default: 0 }, // part du total du niveau parent (0 : non affichee)
  adresse: { type: String, default: '' },
  envoyable: { type: Boolean, default: false }, // envoi au Purgatoire possible (bloc ou message choisi, ou niveau hors racine)
  teinte: { type: String, default: '' }, // couleur du bloc decrit (graphiques seulement ; textes et boutons gardent l'accent)
});
const emit = defineEmits(['envoyer']);

const anneeChoisie = ref('');
watch(() => props.detail, () => { anneeChoisie.value = ''; });

const annees = computed(() => props.detail?.parAnnee || []);
const maxAnnee = computed(() => Math.max(1, ...annees.value.map((a) => a.nombre)));
const pic = computed(() => annees.value.reduce((m, a) => (a.nombre > (m?.nombre || 0) ? a : m), null));
const anneeAffichee = computed(() => annees.value.find((a) => a.cle === anneeChoisie.value) || pic.value);

const listes = computed(() => [
  { cle: 'expediteurs', titre: 'detail.expediteurs', lignes: props.detail?.expediteurs || [] },
  { cle: 'destinataires', titre: 'detail.destinataires', lignes: props.detail?.destinataires || [] },
].filter((l) => !props.detail?.message && l.lignes.length > 1));

const piecesTotal = computed(() => (props.detail?.pieces || []).reduce((s, p) => s + p.taille, 0));
const piecesTete = computed(() => (props.detail?.pieces || []).slice(0, 3));

function an(s) { return s ? new Date(s * 1000).getFullYear() : ''; }
function date(s) { return s ? new Date(s * 1000).toLocaleDateString() : ''; }

function ouvrirDansGmail() {
  const m = props.detail?.message;
  if (!m) return;
  BrowserOpenURL('https://mail.google.com/mail/?authuser=' + encodeURIComponent(props.adresse) + '#all/' + m.gmMsgId);
}
</script>

<template>
  <aside class="panneau" :aria-label="t('detail.titre')" :style="{ '--teinte': teinte || null }">
    <div class="tete">
      <span class="aide">{{ sousTitre }}</span>
      <h2><span v-if="teinte" class="pastille" :style="{ background: teinte }" aria-hidden="true" />{{ titre }}</h2>
    </div>
    <template v-if="detail">
      <dl class="chiffres">
        <div><dt>{{ t('detail.volume') }}</dt><dd class="mono">{{ fmtTaille(detail.taille) }}</dd></div>
        <div><dt>{{ t('detail.messages') }}</dt><dd class="mono">{{ fmtNombre(detail.nombre) }}</dd></div>
        <div v-if="part"><dt>{{ t('detail.part') }}</dt><dd class="mono">{{ (part * 100).toFixed(1).replace('.', ',') }} %</dd></div>
        <div v-if="!detail.message"><dt>{{ t('detail.periode') }}</dt><dd class="mono">{{ an(detail.premiere) }} → {{ an(detail.derniere) }}</dd></div>
      </dl>

      <section v-if="detail.message" class="message">
        <p class="objet">{{ detail.message.objet || t('carte.sansObjet') }}</p>
        <dl class="lignes">
          <div><dt>{{ t('detail.de') }}</dt><dd>{{ detail.message.nom ? detail.message.nom + ' <' + detail.message.expediteur + '>' : detail.message.expediteur }}</dd></div>
          <div v-if="detail.message.destinataires?.length"><dt>{{ t('detail.a') }}</dt><dd>{{ detail.message.destinataires.slice(0, 3).join(', ') }}<template v-if="detail.message.destinataires.length > 3"> {{ t('detail.etAutres', { n: detail.message.destinataires.length - 3 }) }}</template></dd></div>
          <div><dt>{{ t('detail.boite') }}</dt><dd>{{ t('boite.' + detail.message.boite) }}</dd></div>
          <div><dt>{{ t('detail.date') }}</dt><dd class="mono">{{ date(detail.message.date) }}</dd></div>
          <div><dt>{{ t('detail.categorie') }}</dt><dd>{{ t('categorie.' + detail.message.categorie) }}</dd></div>
          <div v-if="detail.message.libelles?.length"><dt>{{ t('detail.libelles') }}</dt><dd>{{ detail.message.libelles.join(', ') }}</dd></div>
        </dl>
        <button type="button" class="btn btn-secondaire" @click="ouvrirDansGmail">{{ t('detail.ouvrirGmail') }}</button>
      </section>

      <section v-if="!detail.message && annees.length > 1" class="annees">
        <div class="titre-section">
          <h3>{{ t('detail.parAnnee') }}</h3>
          <span v-if="anneeAffichee" class="mono aide">{{ anneeAffichee.cle }} · {{ t('carte.nbMessages', { n: fmtNombre(anneeAffichee.nombre) }) }}</span>
        </div>
        <div class="barres">
          <button v-for="a in annees" :key="a.cle" type="button" class="barre"
            :class="{ choisie: a.cle === anneeAffichee?.cle }"
            :style="{ height: Math.max(3, Math.round((a.nombre / maxAnnee) * 88)) + 'px' }"
            :aria-label="a.cle + ' : ' + t('carte.nbMessages', { n: fmtNombre(a.nombre) })"
            :aria-pressed="a.cle === anneeAffichee?.cle"
            @click="anneeChoisie = a.cle" @mouseenter="anneeChoisie = a.cle" />
        </div>
        <div class="bornes mono"><span>{{ annees[0].cle }}</span><span>{{ annees[annees.length - 1].cle }}</span></div>
      </section>

      <!-- Une liste a une seule entree n'apprend rien (ex. : expediteur dans les
           envoyes) ; l'ordre suit la surface choisie (taille ou nombre). -->
      <section v-for="l in listes" :key="l.cle" class="liste">
        <div class="titre-section">
          <h3>{{ t(l.titre) }}</h3>
          <span class="aide colonnes" aria-hidden="true"><span class="chiffre">{{ t('detail.messages') }}</span><span class="chiffre">{{ t('detail.volume') }}</span></span>
        </div>
        <ol>
          <li v-for="e in l.lignes" :key="e.cle">
            <span class="nom" :title="e.cle">{{ e.libelle || e.cle }}</span>
            <span class="mono sec chiffre">{{ fmtNombre(e.nombre) }}</span>
            <span class="mono sec chiffre">{{ fmtTaille(e.taille) }}</span>
          </li>
        </ol>
      </section>

      <section v-if="piecesTotal > 0" class="liste">
        <h3>{{ t('detail.pieces') }}</h3>
        <div class="empile" role="img" :aria-label="piecesTete.map((p) => t('pj.' + p.cle) + ' ' + fmtTaille(p.taille)).join(', ')">
          <span v-for="(p, i) in piecesTete" :key="p.cle" :class="'pj' + i" :style="{ width: (p.taille / piecesTotal * 100) + '%' }" />
        </div>
        <ol>
          <li v-for="p in piecesTete" :key="p.cle"><span>{{ t('pj.' + p.cle) }}</span><span class="mono sec">{{ fmtTaille(p.taille) }}</span></li>
        </ol>
      </section>
      <button v-if="envoyable" type="button" class="btn btn-principal envoyer" @click="emit('envoyer')">
        <IconePurgatoire />{{ t('purgatoire.envoyer') }}
      </button>
    </template>
  </aside>
</template>

<style scoped>
.panneau { flex: 0 0 312px; min-width: 0; background: var(--carte); border-left: 1px solid var(--trait); padding: 24px 20px; box-sizing: border-box; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; }
.tete { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
h2 { margin: 0; font-size: 20px; font-weight: 500; overflow-wrap: anywhere; }
h3 { margin: 0; font-size: 14px; font-weight: 500; }
.chiffres, .lignes { margin: 0; display: flex; flex-direction: column; }
.chiffres div, .lignes div { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; min-height: 32px; border-bottom: 1px solid var(--trait); }
.chiffres div + div, .lignes div + div { padding-top: 8px; }
dt { color: var(--sec); }
dd { margin: 0; text-align: right; overflow-wrap: anywhere; }
.message { display: flex; flex-direction: column; gap: 16px; }
.objet { margin: 0; font-size: 15px; line-height: 22px; }
.annees, .liste { display: flex; flex-direction: column; gap: 8px; }
.titre-section { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; }
.barres { display: flex; align-items: flex-end; gap: 2px; height: 88px; border-bottom: 1px solid var(--trait); }
.barre { flex: 1 1 0; min-width: 0; padding: 0; border: 0; border-radius: 2px 2px 0 0; background: var(--barre, #d4d4d8); }
.barre.choisie { background: var(--teinte, var(--acc)); }
.pastille { display: inline-block; width: 12px; height: 12px; margin-right: 8px; border-radius: 3px; vertical-align: 1px; }
@media (prefers-color-scheme: dark) { .barre { --barre: #3a3a3e; } }
.bornes { display: flex; justify-content: space-between; font-size: 12px; color: var(--muet); }
.liste ol { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; font-size: 13px; }
.liste li { display: flex; justify-content: space-between; gap: 8px; min-height: 24px; align-items: center; }
.nom { flex: 1 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.chiffre { flex: 0 0 auto; min-width: 56px; text-align: right; font-variant-numeric: tabular-nums; }
.chiffre + .chiffre { min-width: 64px; }
.colonnes { display: flex; gap: 8px; font-size: 12px; }
.sec { color: var(--sec); }
.empile { display: flex; gap: 2px; height: 8px; border-radius: 4px; overflow: hidden; }
.pj0 { background: var(--teinte, var(--acc)); }
.pj1 { background: #a1a1aa; }
.pj2 { background: #d4d4d8; }
@media (prefers-color-scheme: dark) { .pj1 { background: #5a5a5f; } .pj2 { background: #404044; } }
.envoyer { margin-top: auto; display: inline-flex; align-items: center; justify-content: center; gap: 8px; min-height: 40px; }
</style>
