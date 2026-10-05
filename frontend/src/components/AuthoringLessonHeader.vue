<template>
  <BtgPageHeader :title="title" :title-id="titleId" context-label="Lesson context" focusable>
    <template #back><RouterLink :to="backTo">Back to Structure</RouterLink></template>
    <template #context><BtgBadge>Lesson</BtgBadge><BtgBadge>Draft</BtgBadge></template>
    <template #navigation><AuthoringTabs label="Lesson sections" :items="tabs" /></template>
  </BtgPageHeader>
</template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import AuthoringTabs from './AuthoringTabs.vue'
import BtgBadge from './BtgBadge.vue'
import BtgPageHeader from './BtgPageHeader.vue'

withDefaults(defineProps<{ title: string; backTo: RouteLocationRaw; tabs: { label: string; to: RouteLocationRaw }[]; titleId?: string }>(), { titleId: 'authoring-lesson-title' })
</script>
