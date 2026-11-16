<script setup>
defineProps({ taille: { type: Number, default: 20 } });
</script>

<template>
  <svg :width="taille" :height="taille" viewBox="0 0 44 44" aria-hidden="true">
    <rect x="1" y="1" width="20" height="42" rx="4" fill="#FBBF24" />
    <rect x="23" y="1" width="20" height="24" rx="4" fill="#60A5FA" />
    <rect x="23" y="27" width="9" height="16" rx="3" fill="#2DD4BF" />
    <rect x="34" y="27" width="9" height="16" rx="3" fill="#A78BFA" />
  </svg>
</template>
