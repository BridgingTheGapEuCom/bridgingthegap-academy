<template>
  <section class="authoring-ui-section" :aria-labelledby="headingId">
    <header class="authoring-ui-section__header">
      <span v-if="number" class="authoring-ui-section__number" aria-hidden="true">{{ number }}</span>
      <div>
        <h3 :id="headingId">{{ title }}</h3>
        <p v-if="description">{{ description }}</p>
      </div>
    </header>
    <div class="authoring-ui-section__body"><slot /></div>
  </section>
</template>

<script setup lang="ts">
defineProps<{ headingId: string; title: string; description?: string; number?: string }>()
</script>
