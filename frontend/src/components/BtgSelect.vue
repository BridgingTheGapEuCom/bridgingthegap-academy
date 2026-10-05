<template>
  <select
    ref="select"
    v-bind="$attrs"
    class="btg-select"
    :data-invalid="invalid || undefined"
    :aria-invalid="invalid || undefined"
    :value="modelValue"
    @change="modelValue = ($event.target as HTMLSelectElement).value"
  >
    <slot />
  </select>
</template>

<script setup lang="ts">
import { ref } from 'vue'

defineOptions({ inheritAttrs: false })

const modelValue = defineModel<string>({ default: '' })
defineProps<{ invalid?: boolean }>()
const select = ref<HTMLSelectElement>()
defineExpose({ focus: () => select.value?.focus() })
</script>
