<template>
  <BtgCheckbox :id="controlId" :model-value="modelValue" :label="label" :description="description" :disabled="disabled" @update:model-value="$emit('update:modelValue', $event)" />
</template>

<script setup lang="ts">
import { useId } from 'vue'
import BtgCheckbox from './BtgCheckbox.vue'

const props = withDefaults(defineProps<{ modelValue: boolean; label: string; description?: string; disabled?: boolean; id?: string }>(), { description: undefined, disabled: false, id: undefined })
defineEmits<{ 'update:modelValue': [value: boolean] }>()
const controlId = props.id ?? `authoring-checkbox-${useId()}`
</script>
