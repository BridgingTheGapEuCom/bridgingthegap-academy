<template>
  <div class="btg-radio">
    <input
      :id="controlId"
      ref="radio"
      v-bind="$attrs"
      class="btg-radio__control"
      type="radio"
      :name="name"
      :value="value"
      :checked="modelValue === value"
      :disabled="disabled"
      @change="modelValue = value"
    >
    <label class="btg-radio__label" :for="controlId">{{ label }}</label>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, useId } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{ id?: string; name: string; value: string; label: string; disabled?: boolean }>(), { id: undefined, disabled: false })
const modelValue = defineModel<string>({ default: '' })
const radio = ref<HTMLInputElement>()
const controlId = computed(() => props.id || `btg-radio-${useId()}`)
defineExpose({ focus: () => radio.value?.focus() })
</script>
