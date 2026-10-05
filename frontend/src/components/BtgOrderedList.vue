<template><component :is="as" class="btg-ordered-list"><slot /></component></template>

<script setup lang="ts">
withDefaults(defineProps<{ as?: 'ol' | 'ul' }>(), { as: 'ol' })
</script>
