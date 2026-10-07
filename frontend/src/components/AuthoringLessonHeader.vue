<template>
  <BtgPageHeader :title="title" :title-id="titleId" :context-label="labels.context" focusable>
    <template #back><RouterLink :to="backTo">{{ labels.back }}</RouterLink></template>
    <template #context><BtgBadge>{{ labels.lesson }}</BtgBadge><BtgBadge>{{ labels.draft }}</BtgBadge></template>
    <template #navigation><AuthoringTabs :label="labels.sections" :items="tabs" /></template>
  </BtgPageHeader>
</template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import AuthoringTabs from './AuthoringTabs.vue'
import BtgBadge from './BtgBadge.vue'
import BtgPageHeader from './BtgPageHeader.vue'

type LessonHeaderLabels = {
  context: string
  back: string
  lesson: string
  draft: string
  sections: string
}

withDefaults(defineProps<{ title: string; backTo: RouteLocationRaw; tabs: { label: string; to: RouteLocationRaw }[]; labels: LessonHeaderLabels; titleId?: string }>(), { titleId: 'authoring-lesson-title' })
</script>
