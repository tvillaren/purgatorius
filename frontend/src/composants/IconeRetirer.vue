<script setup>
// Pictogramme du retrait du Purgatoire : une fleche de retour.
defineProps({ taille: { type: Number, default: 16 } });
</script>

<template>
  <svg :width="taille" :height="taille" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 14l-5-5 5-5" /><path d="M4 9h10.5a5.5 5.5 0 010 11H11" /></svg>
</template>
