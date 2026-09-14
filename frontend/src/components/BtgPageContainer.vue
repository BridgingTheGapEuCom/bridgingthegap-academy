<template>
  <component :is="as" class="btg-page-container" :class="`btg-page-container--${width}`">
    <slot />
  </component>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    as?: 'div' | 'section' | 'article'
    width?: 'application' | 'reading' | 'form'
  }>(),
  { as: 'div', width: 'application' },
)
</script>
