<template>
  <div class="btg-split-workspace" :class="`btg-split-workspace--${variant}`">
    <section class="btg-split-workspace__primary" :aria-label="primaryLabel || undefined"><slot name="primary" /></section>
    <section class="btg-split-workspace__secondary" :aria-label="secondaryLabel || undefined"><slot name="secondary" /></section>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ variant?: 'balanced' | 'outline-inspector'; primaryLabel?: string; secondaryLabel?: string }>(), {
  variant: 'balanced',
  primaryLabel: undefined,
  secondaryLabel: undefined,
})
</script>
