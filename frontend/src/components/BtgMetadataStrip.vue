<template>
  <dl class="btg-metadata-strip">
    <div v-for="item in items" :key="item.label" class="btg-metadata-strip__item">
      <dt>{{ item.label }}</dt>
      <dd><BtgIcon v-if="item.icon" :name="item.icon" decorative size="small" />{{ item.value }}</dd>
    </div>
  </dl>
</template>

<script setup lang="ts">
import BtgIcon from './BtgIcon.vue'
import type { BtgIconName } from './btg-icon-names'

defineProps<{ items: { label: string; value: string; icon?: BtgIconName }[] }>()
</script>
