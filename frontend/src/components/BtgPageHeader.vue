<template>
  <div class="btg-page-header">
    <slot name="back" />
    <header class="btg-page-header__main">
      <p v-if="eyebrow" class="btg-page-header__eyebrow">{{ eyebrow }}</p>
      <h1 :id="resolvedTitleId" :tabindex="focusable ? -1 : undefined">{{ title }}</h1>
      <div v-if="$slots.context" class="btg-page-header__context" :aria-label="contextLabel"><slot name="context" /></div>
      <p v-if="description" class="btg-page-header__description">{{ description }}</p>
      <slot name="metadata" />
    </header>
    <slot name="navigation" />
  </div>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'

const props = withDefaults(defineProps<{ title: string; titleId?: string; eyebrow?: string; description?: string; contextLabel?: string; focusable?: boolean }>(), {
  titleId: undefined,
  eyebrow: undefined,
  description: undefined,
  contextLabel: undefined,
  focusable: false,
})
const resolvedTitleId = computed(() => props.titleId || `btg-page-header-${useId()}`)
</script>
