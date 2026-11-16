<script setup>
import Logo from '../composants/Logo.vue';
import { DefinirLangue } from '../../wailsjs/go/app/App';
import { langue, choisirLangue, t } from '../i18n.js';

defineEmits(['ajouter']);

async function changerLangue(l) {
  choisirLangue(l);
  try { await DefinirLangue(l); } catch (_) { /* memorisation facultative */ }
}

// Apercu illustratif (pas les donnees de l'utilisateur) : blocs aux teintes
// douces de la carte, positions d'un vrai calcul squarified.
const blocs = [
  { nom: 'gmail.com', taille: '4,2 Go', p: 'left:0;top:0;width:31%;height:100%', c: 'var(--t1)' },
  { nom: 'travail-exemple.fr', taille: '3,1 Go', p: 'left:31%;top:0;width:33%;height:69%', c: 'var(--t2)' },
  { nom: '', taille: '', p: 'left:31%;top:69%;width:33%;height:31%', c: 'var(--t3)' },
  { nom: '', taille: '910 Mo', p: 'left:64%;top:0;width:19%;height:35%', c: 'var(--t4)' },
  { nom: '', taille: '780 Mo', p: 'left:83%;top:0;width:17%;height:35%', c: 'var(--t5)' },
  { nom: '', taille: '', p: 'left:64%;top:35%;width:19%;height:27%', c: 'var(--t6)' },
  { nom: '', taille: '', p: 'left:83%;top:35%;width:17%;height:27%', c: 'var(--t7)' },
  { nom: '', taille: '', p: 'left:64%;top:62%;width:18%;height:38%', c: 'var(--t8)' },
  { nom: '', taille: '', p: 'left:82%;top:62%;width:9%;height:38%', c: 'var(--t9)' },
  { nom: '', taille: '', p: 'left:91%;top:62%;width:9%;height:38%', c: 'var(--t10)' },
];
</script>

<template>
  <div class="accueil">
    <section class="presentation">
      <div class="marque"><Logo :taille="32" /><span>Purgatorius</span></div>
      <div class="titres">
        <h1>{{ t('accueil.titre') }}</h1>
        <p>{{ t('accueil.texte') }}</p>
      </div>
      <figure class="apercu" :aria-label="t('accueil.apercu')">
        <div class="apercu-tete"><span>Tous les messages › Domaines</span><span class="mono">13,6 Go</span></div>
        <div class="apercu-carte">
          <div v-for="(b, i) in blocs" :key="i" class="bloc" :style="b.p + ';background:' + b.c">
            <div v-if="b.nom" class="bloc-nom">{{ b.nom }}</div>
            <div v-if="b.taille" class="mono bloc-taille">{{ b.taille }}</div>
          </div>
        </div>
      </figure>
    </section>
    <aside class="commencer">
      <div class="entete">
        <h2>{{ t('accueil.commencer') }}</h2>
        <div class="langues" role="group" :aria-label="t('commun.langue')">
          <button type="button" :aria-pressed="langue === 'fr'" :class="{ actif: langue === 'fr' }" @click="changerLangue('fr')">FR</button>
          <button type="button" :aria-pressed="langue === 'en'" :class="{ actif: langue === 'en' }" @click="changerLangue('en')">EN</button>
        </div>
      </div>
      <button type="button" class="btn btn-principal grand" @click="$emit('ajouter')">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('accueil.ajouter') }}
      </button>
      <ul class="promesses">
        <li><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="4" y="10" width="16" height="11" rx="2" /><path d="M8 10V7a4 4 0 0 1 8 0v3" /></svg>{{ t('accueil.local') }}</li>
        <li><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>{{ t('accueil.entetes') }}</li>
        <li><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" /></svg>{{ t('accueil.corbeille') }}</li>
      </ul>
      <p class="aide">{{ t('accueil.autres') }}</p>
    </aside>
  </div>
</template>

<style scoped>
.accueil {
  min-height: 100vh; display: flex; flex-wrap: wrap;
  --t1: #60a5fa; --t2: #34d399; --t3: #c9ced6; --t4: #fbbf24; --t5: #f87171; --t6: #a78bfa; --t7: #f472b6; --t8: #2dd4bf; --t9: #fb923c; --t10: #818cf8;
}
.presentation { flex: 999 1 560px; min-width: 0; padding: 64px 56px; box-sizing: border-box; display: flex; flex-direction: column; justify-content: center; gap: 32px; }
.marque { display: flex; align-items: center; gap: 12px; font-size: 18px; font-weight: 600; }
.titres { display: flex; flex-direction: column; gap: 16px; max-width: 600px; }
h1 { margin: 0; font-size: 44px; line-height: 1.08; font-weight: 500; letter-spacing: -0.02em; }
.titres p { margin: 0; font-size: 18px; line-height: 28px; color: var(--sec); max-width: 36rem; }
.apercu { margin: 0; width: 100%; max-width: 640px; background: var(--carte); border-radius: 16px; padding: 12px; box-sizing: border-box; box-shadow: inset 0 0 0 1px var(--trait), var(--ombre); }
.apercu-tete { display: flex; justify-content: space-between; padding: 4px 4px 12px; font-size: 13px; color: var(--sec); }
.apercu-carte { position: relative; height: 200px; border-radius: 8px; overflow: hidden; }
.bloc { position: absolute; box-sizing: border-box; border: 2px solid var(--carte); border-radius: 8px; padding: 8px; color: #171717; overflow: hidden; }
.bloc-nom { font-size: 12px; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.bloc-taille { font-size: 12px; }
.commencer { flex: 1 1 360px; min-width: 0; background: var(--carte); border-left: 1px solid var(--trait); padding: 40px; box-sizing: border-box; display: flex; flex-direction: column; justify-content: center; gap: 32px; }
.entete { display: flex; justify-content: space-between; align-items: center; }
h2 { margin: 0; font-size: 20px; font-weight: 500; }
.langues { display: flex; gap: 4px; padding: 4px; border-radius: 10px; background: var(--releve); }
.langues button { min-height: 28px; padding: 0 12px; border-radius: 6px; border: 0; background: transparent; color: var(--sec); font-size: 13px; }
.langues button.actif { background: var(--carte); color: var(--txt); font-weight: 500; box-shadow: var(--ombre-petite); }
.grand { min-height: 48px; font-size: 15px; }
.promesses { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 16px; font-size: 14px; line-height: 20px; color: var(--sec); }
.promesses li { display: flex; gap: 12px; align-items: flex-start; }
.promesses svg { flex-shrink: 0; margin-top: 2px; color: var(--muet); }
.aide { margin: 0; line-height: 20px; }
</style>
