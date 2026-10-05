<template>
  <span class="btg-chip">
    <BtgIcon v-if="icon" :name="icon" decorative size="small" />
    <slot />
  </span>
</template>

<script setup lang="ts">
import BtgIcon from './BtgIcon.vue'
import type { BtgIconName } from './btg-icon-names'

defineProps<{ icon?: BtgIconName }>()
</script>
