<template>
  <button v-bind="$attrs" class="btg-drag-handle" type="button" :disabled="disabled" :aria-label="label"><BtgIcon name="drag" decorative /></button>
</template>

<script setup lang="ts">
import BtgIcon from './BtgIcon.vue'

defineOptions({ inheritAttrs: false })
withDefaults(defineProps<{ label: string; disabled?: boolean }>(), { disabled: false })
</script>
