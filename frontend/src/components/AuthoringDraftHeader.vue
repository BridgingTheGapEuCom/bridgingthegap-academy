<template>
  <BtgPageHeader :title="title" :title-id="titleId" :eyebrow="t('authoring.draftHeader.eyebrow')">
    <template #back><RouterLink to="/authoring">{{ t('authoring.draftHeader.back') }}</RouterLink></template>
    <template #metadata><BtgMetadataStrip :items="metadata" /></template>
    <template #navigation><AuthoringTabs :label="t('authoring.draftHeader.sections')" :items="tabs" /></template>
  </BtgPageHeader>
</template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AuthoringTabs from './AuthoringTabs.vue'
import BtgMetadataStrip from './BtgMetadataStrip.vue'
import BtgPageHeader from './BtgPageHeader.vue'

withDefaults(defineProps<{ title: string; metadata: { label: string; value: string }[]; tabs: { label: string; to: RouteLocationRaw }[]; titleId?: string }>(), { titleId: 'authoring-draft-title' })
const { t } = useI18n()
</script>
