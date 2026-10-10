<template>
  <header class="btg-page-title">
    <h1 :id="id" :tabindex="focusable ? -1 : undefined">{{ title }}</h1>
    <p v-if="description">{{ description }}</p>
  </header>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ id: string; title: string; description?: string; focusable?: boolean }>(), {
  description: undefined,
  focusable: false,
})
</script>
