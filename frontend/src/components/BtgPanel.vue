<template>
  <component :is="as" class="btg-panel" :class="[`btg-panel--${variant}`, `btg-panel--${padding}`]"><slot /></component>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  as?: 'div' | 'section' | 'article' | 'aside'
  variant?: 'default' | 'subtle' | 'elevated'
  padding?: 'none' | 'compact' | 'default'
}>(), { as: 'div', variant: 'default', padding: 'default' })
</script>
