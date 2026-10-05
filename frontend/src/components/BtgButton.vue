<template>
  <button
    class="btg-button"
    :class="[`btg-button--${variant}`, `btg-button--${size}`]"
    :type="type"
    :disabled="disabled || loading"
    :aria-busy="loading || undefined"
  >
    <BtgIcon v-if="leadingIcon && !loading" :name="leadingIcon" decorative />
    <span v-if="loading" class="btg-button__loading"><span class="btg-button__spinner" aria-hidden="true" /></span>
    <slot />
    <BtgIcon v-if="trailingIcon" :name="trailingIcon" decorative />
  </button>
</template>

<script setup lang="ts">
import BtgIcon from './BtgIcon.vue'
import type { BtgIconName } from './btg-icon-names'

withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'tertiary' | 'danger' | 'danger-secondary' | 'quiet' | 'destructive' | 'destructive-secondary'
    size?: 'small' | 'medium'
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
    loading?: boolean
    leadingIcon?: BtgIconName
    trailingIcon?: BtgIconName
  }>(),
  { variant: 'primary', size: 'medium', type: 'button', disabled: false, loading: false, leadingIcon: undefined, trailingIcon: undefined },
)
</script>
