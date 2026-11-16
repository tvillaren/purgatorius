// Synchro automatique : une synchro incrementale (nouveaux messages, drapeaux
// et libelles rattrapes) par compte et par session de l'app, lancee en
// arriere-plan quand sa carte s'affiche. Une synchro menee a l'ecran de
// synchro compte aussi.
export const synchronises = new Set();
