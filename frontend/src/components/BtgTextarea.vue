<template>
  <textarea
    ref="textarea"
    v-bind="$attrs"
    class="btg-textarea"
    :data-invalid="invalid || undefined"
    :aria-invalid="invalid || undefined"
    :value="modelValue"
    @input="modelValue = ($event.target as HTMLTextAreaElement).value"
  />
</template>

<script setup lang="ts">
import { ref } from 'vue'

defineOptions({ inheritAttrs: false })

const modelValue = defineModel<string>({ default: '' })
defineProps<{ invalid?: boolean }>()
const textarea = ref<HTMLTextAreaElement>()
defineExpose({ focus: () => textarea.value?.focus() })
</script>
