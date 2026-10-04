<template>
  <aside v-if="show" class="authoring-dirty-bar" aria-label="Unsaved changes">
    <p role="status"><span aria-hidden="true"></span>{{ message }}</p>
    <div>
      <BtgButton type="button" variant="secondary" :disabled="disabled" @click="$emit('discard')">{{ discardLabel }}</BtgButton>
      <BtgButton type="submit" :disabled="disabled || saveDisabled">{{ busy ? busyLabel : saveLabel }}</BtgButton>
    </div>
  </aside>
</template>

<script setup lang="ts">
import BtgButton from './BtgButton.vue'

withDefaults(defineProps<{ show: boolean; disabled?: boolean; saveDisabled?: boolean; busy?: boolean; message?: string; discardLabel?: string; saveLabel?: string; busyLabel?: string }>(), {
  disabled: false,
  saveDisabled: false,
  busy: false,
  message: 'Unsaved changes',
  discardLabel: 'Discard changes',
  saveLabel: 'Save changes',
  busyLabel: 'Saving…',
})
defineEmits<{ discard: [] }>()
</script>
