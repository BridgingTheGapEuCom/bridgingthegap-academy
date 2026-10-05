<template>
  <span class="btg-badge" :class="`btg-badge--${variant}`">
    <BtgIcon v-if="icon" :name="icon" decorative size="small" />
    <slot />
  </span>
</template>

<script setup lang="ts">
import BtgIcon from './BtgIcon.vue'
import type { BtgIconName } from './btg-icon-names'

withDefaults(defineProps<{ variant?: 'neutral' | 'success' | 'warning' | 'danger'; icon?: BtgIconName }>(), { variant: 'neutral', icon: undefined })
</script>
