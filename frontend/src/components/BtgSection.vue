<template>
  <section class="btg-section" :aria-labelledby="headingId">
    <header class="btg-section__header">
      <component :is="headingAs" :id="headingId" class="btg-section__heading"><slot name="heading" /></component>
      <div v-if="$slots.action" class="btg-section__action"><slot name="action" /></div>
    </header>
    <div class="btg-section__body"><slot /></div>
  </section>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'

const props = withDefaults(defineProps<{ headingAs?: 'h2' | 'h3' | 'h4'; headingId?: string }>(), { headingAs: 'h2', headingId: undefined })
const headingId = computed(() => props.headingId || `btg-section-${useId()}`)
</script>
