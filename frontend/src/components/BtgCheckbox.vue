<template>
  <div class="btg-checkbox">
    <input
      :id="controlId"
      ref="checkbox"
      v-bind="$attrs"
      class="btg-checkbox__control"
      type="checkbox"
      :checked="modelValue"
      :disabled="disabled"
      :aria-invalid="Boolean(error) || undefined"
      :aria-describedby="describedBy"
      @change="modelValue = ($event.target as HTMLInputElement).checked"
    >
    <label class="btg-checkbox__label" :for="controlId">{{ label }}</label>
    <span v-if="description" :id="descriptionId" class="btg-checkbox__description">{{ description }}</span>
    <span v-if="error" :id="errorId" class="btg-checkbox__error" role="alert">{{ error }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, useId, watch } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  label: string
  id?: string
  description?: string
  error?: string
  disabled?: boolean
  indeterminate?: boolean
}>(), { id: undefined, description: undefined, error: undefined, disabled: false, indeterminate: false })

const modelValue = defineModel<boolean>({ default: false })
const generatedId = `btg-checkbox-${useId()}`
const controlId = computed(() => props.id || generatedId)
const descriptionId = computed(() => `${controlId.value}-description`)
const errorId = computed(() => `${controlId.value}-error`)
const describedBy = computed(() => [props.description ? descriptionId.value : '', props.error ? errorId.value : ''].filter(Boolean).join(' ') || undefined)
const checkbox = ref<HTMLInputElement>()

function syncIndeterminate() {
  if (checkbox.value) checkbox.value.indeterminate = props.indeterminate
}

onMounted(syncIndeterminate)
watch(() => props.indeterminate, syncIndeterminate)
</script>
