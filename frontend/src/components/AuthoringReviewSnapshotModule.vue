<template>
  <section class="authoring-review-snapshot__module" :aria-labelledby="`review-snapshot-module-${module.id}`">
    <header>
      <p class="authoring-review-snapshot__eyebrow">Module <code>{{ module.stableKey }}</code></p>
      <h3 :id="`review-snapshot-module-${module.id}`">{{ module.title }}</h3>
      <p v-if="module.description">{{ module.description }}</p>
    </header>
    <p v-if="!module.lessons.length">No lessons were captured in this Module.</p>
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
import AuthoringReviewSnapshotLesson from './AuthoringReviewSnapshotLesson.vue'

defineProps<{
  module: AuthoringReviewSnapshotModule
  lessonTitles: Readonly<Record<string, string>>
}>()
</script>
