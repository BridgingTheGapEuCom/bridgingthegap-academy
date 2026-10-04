<template>
  <header class="authoring-page-title">
    <component :is="as" :id="id" :tabindex="focusable ? -1 : undefined">{{ title }}</component>
    <p v-if="description">{{ description }}</p>
  </header>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ id: string; title: string; description?: string; as?: 'h1' | 'h2' | 'h3'; focusable?: boolean }>(), {
  description: undefined,
  as: 'h2',
  focusable: false,
})
</script>
