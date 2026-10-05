<template>
  <div class="btg-disclosure">
    <BtgButton class="btg-disclosure__trigger" variant="tertiary" :aria-expanded="modelValue" :aria-controls="panelId" @click="modelValue = !modelValue">
      <BtgIcon :name="modelValue ? 'chevronDown' : 'chevronRight'" decorative />
      <slot name="label">{{ label }}</slot>
    </BtgButton>
    <div v-if="modelValue" :id="panelId" class="btg-disclosure__content" role="region" :aria-label="label"><slot /></div>
  </div>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'
import BtgButton from './BtgButton.vue'
import BtgIcon from './BtgIcon.vue'

const props = defineProps<{ label: string; id?: string }>()
const modelValue = defineModel<boolean>({ default: false })
const panelId = computed(() => props.id || `btg-disclosure-${useId()}`)
</script>
