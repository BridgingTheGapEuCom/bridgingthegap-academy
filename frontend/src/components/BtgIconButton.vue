<template>
  <BtgButton
    class="btg-icon-button"
    :class="`btg-icon-button--${size}`"
    :variant="variant"
    :size="size"
    :type="type"
    :disabled="disabled"
    :loading="loading"
    :aria-label="label"
  >
    <BtgIcon v-if="!loading" :name="icon" decorative />
    <span class="sr-only">{{ label }}</span>
  </BtgButton>
</template>

<script setup lang="ts">
import BtgButton from './BtgButton.vue'
import BtgIcon from './BtgIcon.vue'
import type { BtgIconName } from './btg-icon-names'

withDefaults(defineProps<{
  icon: BtgIconName
  label: string
  variant?: 'primary' | 'secondary' | 'tertiary' | 'danger' | 'danger-secondary' | 'quiet' | 'destructive' | 'destructive-secondary'
  size?: 'small' | 'medium'
  type?: 'button' | 'submit' | 'reset'
  disabled?: boolean
  loading?: boolean
}>(), { variant: 'secondary', size: 'medium', type: 'button', disabled: false, loading: false })
</script>
