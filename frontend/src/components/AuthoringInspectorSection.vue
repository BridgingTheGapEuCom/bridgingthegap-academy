<template>
  <section class="authoring-inspector-section" :aria-labelledby="headingId">
    <header class="authoring-inspector-section__header">
      <div class="authoring-inspector-section__identity">
        <span v-if="icon" class="authoring-inspector-section__icon"><BtgIcon :name="icon" decorative /></span>
        <h3 :id="headingId">{{ title }}</h3>
      </div>
      <div v-if="$slots.action" class="authoring-inspector-section__action"><slot name="action" /></div>
    </header>
    <div class="authoring-inspector-section__body"><slot /></div>
  </section>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'
import BtgIcon from './BtgIcon.vue'
import type { BtgIconName } from './btg-icon-names'

const props = defineProps<{ title: string; icon?: BtgIconName; headingId?: string }>()
const headingId = computed(() => props.headingId || `authoring-inspector-section-${useId()}`)
</script>
