<template>
  <aside v-if="show" class="authoring-dirty-bar" aria-label="Unsaved changes">
    <BtgToolbar>
      <template #leading><p role="status"><span aria-hidden="true"></span>{{ message }}</p></template>
      <template #actions>
      <BtgButton type="button" variant="secondary" :disabled="disabled" @click="$emit('discard')">{{ discardLabel }}</BtgButton>
      <BtgButton type="submit" :disabled="disabled || saveDisabled">{{ busy ? busyLabel : saveLabel }}</BtgButton>
      </template>
    </BtgToolbar>
  </aside>
</template>

<script setup lang="ts">
import BtgButton from './BtgButton.vue'
import BtgToolbar from './BtgToolbar.vue'

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
