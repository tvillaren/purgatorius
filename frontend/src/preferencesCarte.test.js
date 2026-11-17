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

test('restaure les reglages et l’ordre des axes sauvegardes', (t) => {
  // Etant donne
  const valeurs = new Map();
  remplacerStockage(t, () => ({
    getItem: (cle) => valeurs.get(cle) ?? null,
    setItem: (cle, valeur) => valeurs.set(cle, valeur),
  }));
  const attendus = { axes: ['libelle', 'annee', 'expediteur'], metrique: 'nombre', boite: 'envoye', lecture: 'nonlu' };

  // Quand
  ecrireReglagesCarte({ ...attendus, chemin: ['Factures'], selection: 42 });
  const reglages = lireReglagesCarte();

  // Alors
  assert.deepEqual(reglages, attendus);
  assert.deepEqual(JSON.parse(valeurs.get('purgatorius.carte')), attendus);
});

test('remplace les anciens reglages par les nouveaux', (t) => {
  // Etant donne
  const anciens = { axes: ['libelle', 'annee', 'expediteur'], metrique: 'nombre', boite: 'envoye', lecture: 'nonlu' };
  const valeurs = new Map([['purgatorius.carte', JSON.stringify(anciens)]]);
  remplacerStockage(t, () => ({
    getItem: (cle) => valeurs.get(cle) ?? null,
    setItem: (cle, valeur) => valeurs.set(cle, valeur),
  }));
  const attendus = { axes: ['expediteur', 'libelle'], metrique: 'taille', boite: 'recu', lecture: 'lu' };

  // Quand
  ecrireReglagesCarte(attendus);
  const reglages = lireReglagesCarte();

  // Alors
  assert.deepEqual(reglages, attendus);
});

test('memorise le retrait des filtres de boite et de lecture', (t) => {
  // Etant donne
  const anciens = { axes: ['expediteur', 'libelle'], metrique: 'taille', boite: 'recu', lecture: 'lu' };
  const valeurs = new Map([['purgatorius.carte', JSON.stringify(anciens)]]);
  remplacerStockage(t, () => ({
    getItem: (cle) => valeurs.get(cle) ?? null,
    setItem: (cle, valeur) => valeurs.set(cle, valeur),
  }));
  const attendus = { ...anciens, boite: '', lecture: '' };

  // Quand
  ecrireReglagesCarte(attendus);
  const reglages = lireReglagesCarte();

  // Alors
  assert.deepEqual(reglages, attendus);
});

test('retire les axes inconnus et les doublons en conservant leur ordre', (t) => {
  // Etant donne
  remplacerStockage(t, () => ({ getItem: () => JSON.stringify({
    axes: ['ancien', 'libelle', null, 'annee', 'libelle', 12, 'message'],
    metrique: 'nombre', boite: 'inconnue', lecture: 'lu',
  }) }));

  // Quand
  const reglages = lireReglagesCarte();

  // Alors
  assert.deepEqual(reglages, { axes: ['libelle', 'annee'], metrique: 'nombre', boite: '', lecture: 'lu' });
});

for (const [nom, valeur] of [
  ['stockage vide', null],
  ['JSON mal forme', '{'],
  ['valeur nulle', 'null'],
  ['booleen', 'false'],
  ['nombre', '42'],
  ['texte', '\"texte\"'],
  ['tableau', '[]'],
  ['objet vide', '{}'],
  ['axes vides', '{"axes":[]}'],
  ['axes obsoletes', '{"axes":["ancien"]}'],
  ['types incorrects', '{"axes":"libelle","metrique":{},"boite":true,"lecture":12}'],
]) {
  test(`reprend les valeurs par defaut : ${nom}`, (t) => {
    // Etant donne
    remplacerStockage(t, () => ({ getItem: () => valeur }));

    // Quand
    const reglages = lireReglagesCarte();

    // Alors
    assert.deepEqual(reglages, defauts);
  });
}

test('modifier les axes relus ne change pas les valeurs par defaut', (t) => {
  // Etant donne
  remplacerStockage(t, () => ({ getItem: () => null }));
  const precedents = lireReglagesCarte();

  // Quand
  precedents.axes.reverse();
  const reglages = lireReglagesCarte();

  // Alors
  assert.deepEqual(reglages, defauts);
});

test('supporte un acces au stockage refuse par le navigateur', (t) => {
  // Etant donne
  remplacerStockage(t, () => { throw new Error('Acces refuse'); });

  // Quand
  ecrireReglagesCarte(defauts);
  const reglages = lireReglagesCarte();

  // Alors
  assert.deepEqual(reglages, defauts);
});

test('supporte un echec de lecture ou un quota de stockage depasse', (t) => {
  // Etant donne
  remplacerStockage(t, () => ({
    getItem: () => { throw new Error('Lecture impossible'); },
    setItem: () => { throw new Error('Quota depasse'); },
  }));

  // Quand
  ecrireReglagesCarte(defauts);
  const reglages = lireReglagesCarte();

  // Alors
  assert.deepEqual(reglages, defauts);
});
