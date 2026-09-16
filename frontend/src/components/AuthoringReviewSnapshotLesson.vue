<template>
  <article class="authoring-review-snapshot__lesson" :aria-labelledby="`review-snapshot-lesson-${lesson.id}`">
    <header>
      <p class="authoring-review-snapshot__eyebrow">Lesson <code>{{ lesson.stableKey }}</code></p>
      <h4 :id="`review-snapshot-lesson-${lesson.id}`">{{ lesson.title }}</h4>
      <p v-if="lesson.description">{{ lesson.description }}</p>
      <p v-if="lesson.estimatedDurationMinutes">Estimated duration: {{ lesson.estimatedDurationMinutes }} minutes</p>
    </header>

    <div v-if="lesson.objectives.length" :aria-labelledby="`review-snapshot-objectives-${lesson.id}`">
      <h5 :id="`review-snapshot-objectives-${lesson.id}`">Objectives</h5>
      <ul><li v-for="objective in lesson.objectives" :key="objective">{{ objective }}</li></ul>
    </div>

    <div v-if="lesson.prerequisiteStableKeys.length" :aria-labelledby="`review-snapshot-prerequisites-${lesson.id}`">
      <h5 :id="`review-snapshot-prerequisites-${lesson.id}`">Recommended prerequisites</h5>
      <p>These recommendations are advisory and do not restrict access.</p>
      <ol><li v-for="key in lesson.prerequisiteStableKeys" :key="key">{{ prerequisiteLabel(key) }}</li></ol>
    </div>

    <div class="authoring-review-snapshot__content" :aria-labelledby="`review-snapshot-content-${lesson.id}`">
      <h5 :id="`review-snapshot-content-${lesson.id}`">Lesson content</h5>
      <p v-if="content.kind === 'unsupported-schema'" role="note">This historical LessonContent format is not supported by this version of the Academy.</p>
      <p v-else-if="content.kind === 'invalid-content'" role="note">This historical LessonContent cannot be displayed safely.</p>
      <p v-else-if="!content.blocks.length">No lesson content was captured in this snapshot.</p>
      <div v-else v-for="block in content.blocks" :key="block.key" class="authoring-review-snapshot__block">
        <LessonBlockRenderer :block="block" />
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AuthoringReviewSnapshotLesson } from '../authoring/authoring'
import LessonBlockRenderer from './LessonBlockRenderer.vue'
import { decodeLessonContent } from '../lesson/content'

const props = defineProps<{
  lesson: AuthoringReviewSnapshotLesson
  lessonTitles: Readonly<Record<string, string>>
}>()

const content = computed(() => decodeLessonContent(props.lesson.content))

function prerequisiteLabel(key: string): string {
  const title = props.lessonTitles[key]
  return title ? `${title} (${key})` : key
}
</script>
