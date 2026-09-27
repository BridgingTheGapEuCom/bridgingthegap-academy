<template>
  <component :is="as" class="btg-page-container" :class="`btg-page-container--${spacing}`">
    <slot />
  </component>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    as?: 'div' | 'section' | 'article'
    spacing?: 'page' | 'section'
  }>(),
  { as: 'div', spacing: 'page' },
)
</script>
