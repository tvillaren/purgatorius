import assert from 'node:assert/strict';
import { test } from 'node:test';
import { lireReglagesCarte, ecrireReglagesCarte } from './preferencesCarte.js';

const defauts = { axes: ['domaine', 'expediteur'], metrique: 'taille', boite: '', lecture: '' };

// Remplace seulement le stockage du navigateur, sans environnement DOM.
function remplacerStockage(t, lire) {
  const avant = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, get: lire });
  t.after(() => {
    if (avant) Object.defineProperty(globalThis, 'localStorage', avant);
    else delete globalThis.localStorage;
  });
}

test('restaure les reglages et l’ordre des axes apres chaque sauvegarde', (t) => {
  const valeurs = new Map();
  remplacerStockage(t, () => ({
    getItem: (cle) => valeurs.get(cle) ?? null,
    setItem: (cle, valeur) => valeurs.set(cle, valeur),
  }));
  assert.deepEqual(lireReglagesCarte(), defauts);

  const reglages = { axes: ['libelle', 'annee', 'expediteur'], metrique: 'nombre', boite: 'envoye', lecture: 'nonlu' };
  ecrireReglagesCarte({ ...reglages, chemin: ['Factures'], selection: 42 });
  assert.deepEqual(lireReglagesCarte(), reglages);
  assert.deepEqual(JSON.parse(valeurs.get('purgatorius.carte')), reglages);

  reglages.axes = ['expediteur', 'libelle'];
  reglages.metrique = 'taille';
  reglages.boite = 'recu';
  reglages.lecture = 'lu';
  ecrireReglagesCarte(reglages);
  assert.deepEqual(lireReglagesCarte(), reglages);

  reglages.boite = '';
  reglages.lecture = '';
  ecrireReglagesCarte(reglages);
  assert.deepEqual(lireReglagesCarte(), reglages);
});

test('retire les axes inconnus et les doublons en conservant leur ordre', (t) => {
  remplacerStockage(t, () => ({ getItem: () => JSON.stringify({
    axes: ['ancien', 'libelle', null, 'annee', 'libelle', 12, 'message'],
    metrique: 'nombre', boite: 'inconnue', lecture: 'lu',
  }) }));
  assert.deepEqual(lireReglagesCarte(), { axes: ['libelle', 'annee'], metrique: 'nombre', boite: '', lecture: 'lu' });
});

test('reprend les valeurs par defaut pour des donnees absentes ou invalides', (t) => {
  let valeur;
  remplacerStockage(t, () => ({ getItem: () => valeur }));
  for (valeur of [null, '{', 'null', 'false', '42', '"texte"', '[]', '{}',
    '{"axes":[]}', '{"axes":["ancien"]}',
    '{"axes":"libelle","metrique":{},"boite":true,"lecture":12}']) {
    assert.deepEqual(lireReglagesCarte(), defauts, String(valeur));
  }
  // Modifier les axes relus ne doit pas modifier les valeurs par defaut.
  lireReglagesCarte().axes.reverse();
  assert.deepEqual(lireReglagesCarte(), defauts);
});

test('supporte un acces au stockage refuse par le navigateur', (t) => {
  remplacerStockage(t, () => { throw new Error('Acces refuse'); });
  assert.deepEqual(lireReglagesCarte(), defauts);
  assert.doesNotThrow(() => ecrireReglagesCarte(defauts));
});

test('supporte un echec de lecture ou un quota de stockage depasse', (t) => {
  remplacerStockage(t, () => ({
    getItem: () => { throw new Error('Lecture impossible'); },
    setItem: () => { throw new Error('Quota depasse'); },
  }));
  assert.deepEqual(lireReglagesCarte(), defauts);
  assert.doesNotThrow(() => ecrireReglagesCarte(defauts));
});
