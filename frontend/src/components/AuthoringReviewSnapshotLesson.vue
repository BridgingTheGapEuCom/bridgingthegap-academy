<template>
  <article class="authoring-review-snapshot__lesson" :aria-labelledby="`review-snapshot-lesson-${lesson.id}`">
    <header>
      <p class="authoring-review-snapshot__eyebrow">{{ t('authoring.review.snapshot.lesson', { key: lesson.stableKey }) }}</p>
      <h4 :id="`review-snapshot-lesson-${lesson.id}`">{{ lesson.title }}</h4>
      <p v-if="lesson.description">{{ lesson.description }}</p>
      <p v-if="lesson.estimatedDurationMinutes">{{ t('authoring.review.snapshot.estimatedDuration', { minutes: lesson.estimatedDurationMinutes }) }}</p>
    </header>

    <div v-if="lesson.objectives.length" :aria-labelledby="`review-snapshot-objectives-${lesson.id}`">
      <h5 :id="`review-snapshot-objectives-${lesson.id}`">{{ t('authoring.review.snapshot.objectives') }}</h5>
      <ul><li v-for="objective in lesson.objectives" :key="objective">{{ objective }}</li></ul>
    </div>

    <div v-if="lesson.prerequisiteStableKeys.length" :aria-labelledby="`review-snapshot-prerequisites-${lesson.id}`">
      <h5 :id="`review-snapshot-prerequisites-${lesson.id}`">{{ t('authoring.review.snapshot.prerequisites') }}</h5>
      <p>{{ t('authoring.review.snapshot.prerequisitesDescription') }}</p>
      <ol><li v-for="key in lesson.prerequisiteStableKeys" :key="key">{{ prerequisiteLabel(key) }}</li></ol>
    </div>

    <div class="authoring-review-snapshot__content" :aria-labelledby="`review-snapshot-content-${lesson.id}`">
      <h5 :id="`review-snapshot-content-${lesson.id}`">{{ t('authoring.review.snapshot.lessonContent') }}</h5>
      <p v-if="content.kind === 'unsupported-schema'" role="note">{{ t('authoring.review.snapshot.unsupportedContent') }}</p>
      <p v-else-if="content.kind === 'invalid-content'" role="note">{{ t('authoring.review.snapshot.invalidContent') }}</p>
      <p v-else-if="!content.blocks.length">{{ t('authoring.review.snapshot.emptyContent') }}</p>
      <div v-else v-for="block in content.blocks" :key="block.key" class="authoring-review-snapshot__block">
        <LessonBlockRenderer :block="block" />
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AuthoringReviewSnapshotLesson } from '../authoring/authoring'
import LessonBlockRenderer from './LessonBlockRenderer.vue'
import { decodeLessonContent } from '../lesson/content'

const props = defineProps<{
  lesson: AuthoringReviewSnapshotLesson
  lessonTitles: Readonly<Record<string, string>>
}>()
const { t } = useI18n()

const content = computed(() => decodeLessonContent(props.lesson.content))

function prerequisiteLabel(key: string): string {
  const title = props.lessonTitles[key]
  return title ? t('authoring.review.snapshot.prerequisite', { title, key }) : key
}
</script>
