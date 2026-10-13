<template>
  <BtgFormField :label="label" :description="description" :error="error" :required="required" v-slot="{ controlId, describedBy, invalid }">
    <input
      :id="controlId"
      ref="input"
      class="btg-file-field__control"
      type="file"
      :accept="accept"
      :disabled="disabled"
      :aria-describedby="describedBy"
      :aria-invalid="invalid || undefined"
      @change="select"
    >
    <div class="btg-file-field__selection" :data-empty="!modelValue || undefined">
      <span>
        <strong>{{ modelValue?.name ?? emptyText }}</strong>
        <small v-if="modelValue && selectedDescription">{{ selectedDescription }}</small>
      </span>
      <BtgButton v-if="modelValue" variant="tertiary" size="small" :disabled="disabled" @click="clear">{{ removeLabel }}</BtgButton>
    </div>
  </BtgFormField>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import BtgButton from './BtgButton.vue'
import BtgFormField from './BtgFormField.vue'

withDefaults(defineProps<{
  label: string
  description?: string
  error?: string
  accept?: string
  disabled?: boolean
  required?: boolean
  emptyText: string
  removeLabel: string
  selectedDescription?: string
}>(), {
  description: undefined,
  error: undefined,
  accept: undefined,
  disabled: false,
  required: false,
  selectedDescription: undefined,
})

const modelValue = defineModel<File | undefined>({ default: undefined })
const input = ref<HTMLInputElement>()

function select(event: Event) {
  modelValue.value = (event.target as HTMLInputElement).files?.[0]
}

function clear() {
  modelValue.value = undefined
  if (input.value) input.value.value = ''
  input.value?.focus()
}

defineExpose({ clear, focus: () => input.value?.focus() })
</script>
