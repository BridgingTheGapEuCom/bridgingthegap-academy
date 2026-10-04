<template>
  <nav class="authoring-tabs" :aria-label="label">
    <ul><li v-for="item in items" :key="item.label"><RouterLink :to="item.to">{{ item.label }}</RouterLink></li></ul>
  </nav>
</template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'

defineProps<{ label: string; items: { label: string; to: RouteLocationRaw }[] }>()
</script>
