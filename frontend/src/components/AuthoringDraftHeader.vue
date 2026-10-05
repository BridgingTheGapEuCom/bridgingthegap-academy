<template>
  <BtgPageHeader :title="title" :title-id="titleId" eyebrow="Authoring Draft">
    <template #back><RouterLink to="/authoring">Back to Authoring</RouterLink></template>
    <template #metadata><BtgMetadataStrip :items="metadata" /></template>
    <template #navigation><AuthoringTabs label="Draft sections" :items="tabs" /></template>
  </BtgPageHeader>
</template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import AuthoringTabs from './AuthoringTabs.vue'
import BtgMetadataStrip from './BtgMetadataStrip.vue'
import BtgPageHeader from './BtgPageHeader.vue'

withDefaults(defineProps<{ title: string; metadata: { label: string; value: string }[]; tabs: { label: string; to: RouteLocationRaw }[]; titleId?: string }>(), { titleId: 'authoring-draft-title' })
</script>
