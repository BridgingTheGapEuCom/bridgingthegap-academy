<template>
  <section class="authoring-review-snapshot__module" :aria-labelledby="`review-snapshot-module-${module.id}`">
    <header>
      <p class="authoring-review-snapshot__eyebrow">{{ t('authoring.review.snapshot.module', { key: module.stableKey }) }}</p>
      <h3 :id="`review-snapshot-module-${module.id}`">{{ module.title }}</h3>
      <p v-if="module.description">{{ module.description }}</p>
    </header>
    <p v-if="!module.lessons.length">{{ t('authoring.review.snapshot.emptyModule') }}</p>
    <template v-else>
      <AuthoringReviewSnapshotLesson
        v-for="lesson in module.lessons"
        :key="lesson.id"
        :lesson="lesson"
        :lesson-titles="lessonTitles"
      />
    </template>
  </section>
</template>

<script setup lang="ts">
import type { AuthoringReviewSnapshotModule } from '../authoring/authoring'
import { useI18n } from 'vue-i18n'
import AuthoringReviewSnapshotLesson from './AuthoringReviewSnapshotLesson.vue'

defineProps<{
  module: AuthoringReviewSnapshotModule
  lessonTitles: Readonly<Record<string, string>>
}>()
const { t } = useI18n()
</script>
