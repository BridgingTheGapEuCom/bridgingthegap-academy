<template>
  <div class="authoring-checkbox-field">
    <label :for="controlId"><input :id="controlId" type="checkbox" :checked="modelValue" :disabled="disabled" :aria-describedby="description ? descriptionId : undefined" @change="$emit('update:modelValue', ($event.target as HTMLInputElement).checked)" />{{ label }}</label>
    <p v-if="description" :id="descriptionId">{{ description }}</p>
  </div>
</template>

<script setup lang="ts">
import { useId } from 'vue'

const props = withDefaults(defineProps<{ modelValue: boolean; label: string; description?: string; disabled?: boolean; id?: string }>(), { description: undefined, disabled: false, id: undefined })
defineEmits<{ 'update:modelValue': [value: boolean] }>()
const controlId = props.id ?? `authoring-checkbox-${useId()}`
const descriptionId = `${controlId}-description`
</script>
