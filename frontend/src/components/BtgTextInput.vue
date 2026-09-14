<template>
  <input
    v-bind="$attrs"
    class="btg-text-input"
    :data-invalid="invalid || undefined"
    :aria-invalid="invalid || undefined"
    :value="modelValue"
    @input="modelValue = ($event.target as HTMLInputElement).value"
  />
</template>

<script setup lang="ts">
defineOptions({ inheritAttrs: false })

const modelValue = defineModel<string>({ default: '' })
defineProps<{ invalid?: boolean }>()
</script>
