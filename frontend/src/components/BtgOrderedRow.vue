<template>
  <li v-bind="$attrs" class="btg-ordered-row" :style="rowStyle" :data-selected="selected || undefined" :aria-current="selected ? 'true' : undefined">
    <div v-if="$slots.handle" class="btg-ordered-row__handle"><slot name="handle" /></div>
    <span v-if="index !== undefined" class="btg-ordered-row__index" aria-hidden="true">{{ displayIndex }}</span>
    <div class="btg-ordered-row__content"><slot /></div>
    <div v-if="$slots.actions" class="btg-ordered-row__actions"><slot name="actions" /></div>
  </li>
</template>

<script setup lang="ts">
import { computed } from 'vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ index?: number; selected?: boolean }>()
const displayIndex = computed(() => props.index === undefined ? '' : String(props.index + 1).padStart(2, '0'))
const rowStyle = {
  display: 'grid',
  gridTemplateColumns: 'auto auto minmax(0, 1fr) auto',
  alignItems: 'center',
}
</script>
