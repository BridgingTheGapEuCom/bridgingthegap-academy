<template>
  <div class="btg-search-field">
    <label :for="controlId" class="btg-search-field__label">{{ label }}</label>
    <BtgIcon class="btg-search-field__icon" name="search" decorative />
    <BtgTextInput
      :id="controlId"
      v-model="modelValue"
      v-bind="$attrs"
      type="search"
      :placeholder="placeholder"
      :invalid="invalid"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'
import BtgIcon from './BtgIcon.vue'
import BtgTextInput from './BtgTextInput.vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  id?: string
  label: string
  placeholder?: string
  invalid?: boolean
}>(), { id: undefined, placeholder: undefined, invalid: false })

const modelValue = defineModel<string>({ default: '' })
const controlId = computed(() => props.id || `btg-search-${useId()}`)
</script>
