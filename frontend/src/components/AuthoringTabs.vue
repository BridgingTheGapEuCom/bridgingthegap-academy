<template><BtgPageTabs :label="label" :items="items" /></template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import BtgPageTabs from './BtgPageTabs.vue'

defineProps<{ label: string; items: { label: string; to: RouteLocationRaw }[] }>()
</script>
