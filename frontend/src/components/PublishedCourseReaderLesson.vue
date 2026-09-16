<template>
  <article v-if="lesson" class="published-course-reader-lesson" aria-labelledby="published-lesson-title">
    <header>
      <p class="published-course-reader-lesson__eyebrow">Lesson</p>
      <h2 id="published-lesson-title" ref="lessonTitle" tabindex="-1">{{ lesson.title }}</h2>
      <p v-if="lesson.description">{{ lesson.description }}</p>
      <p v-if="formatDuration(lesson.estimatedDurationMinutes)" class="published-course-reader-lesson__duration">
        Estimated duration: {{ formatDuration(lesson.estimatedDurationMinutes) }}
      </p>
    </header>

    <div v-if="lesson.objectives.length">
      <h3 id="published-lesson-objectives-title">What you’ll learn</h3>
      <ul><li v-for="objective in lesson.objectives" :key="objective">{{ objective }}</li></ul>
    </div>

    <section class="published-course-reader-lesson__placeholder" aria-labelledby="published-lesson-content-title">
      <h3 id="published-lesson-content-title">Lesson content</h3>
      <p>Lesson content will be available here.</p>
    </section>
  </article>
</template>

<script setup lang="ts">
import { onMounted, onUpdated, ref } from 'vue'
import type { PublishedCourseVersionDetail } from '../courses/courses'
import { formatDuration } from '../courses/courses'

const props = defineProps<{ lesson: PublishedCourseVersionDetail['modules'][number]['lessons'][number] | null }>()
const lessonTitle = ref<HTMLHeadingElement | null>(null)
let previousLessonStableKey: string | undefined

onMounted(() => { previousLessonStableKey = props.lesson?.stableKey })
onUpdated(() => {
  const lessonStableKey = props.lesson?.stableKey
  if (lessonStableKey && previousLessonStableKey && lessonStableKey !== previousLessonStableKey) lessonTitle.value?.focus()
  previousLessonStableKey = lessonStableKey
})
</script>
