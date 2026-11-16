<script setup>
// Pictogramme du Purgatoire : une corbeille et une horloge (l'attente avant
// la corbeille Gmail).
defineProps({ taille: { type: Number, default: 16 } });
</script>

<template>
  <svg :width="taille" :height="taille" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" /><path d="M12 10v4l2 1" /></svg>
</template>
