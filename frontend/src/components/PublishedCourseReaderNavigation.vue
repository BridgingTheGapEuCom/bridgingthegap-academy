<template>
  <nav class="published-course-reader-navigation" aria-label="Course lessons">
    <section
      v-for="module in modules"
      :key="module.stableKey"
      class="published-course-reader-navigation__module"
      :aria-labelledby="`published-module-${module.stableKey}`"
    >
      <h2 :id="`published-module-${module.stableKey}`">{{ module.title }}</h2>
      <p v-if="module.description">{{ module.description }}</p>
      <ol v-if="module.lessons.length" class="published-course-reader-navigation__lessons">
        <li v-for="lesson in module.lessons" :key="lesson.stableKey">
          <RouterLink
            :to="lessonPath(lesson.stableKey)"
            :aria-current="lesson.stableKey === selectedLessonKey ? 'page' : undefined"
            :aria-label="lessonLabel(lesson.title, lesson.estimatedDurationMinutes)"
          >
            <span>{{ lesson.title }}</span>
            <span v-if="formatDuration(lesson.estimatedDurationMinutes)" class="published-course-reader-navigation__duration">
              {{ formatDuration(lesson.estimatedDurationMinutes) }}
            </span>
          </RouterLink>
        </li>
      </ol>
      <p v-else class="published-course-reader-navigation__empty">No lessons in this module yet.</p>
    </section>
  </nav>
</template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import type { PublishedCourseVersionDetail } from '../courses/courses'
import { formatDuration } from '../courses/courses'

const { coursePath } = defineProps<{
  modules: PublishedCourseVersionDetail['modules']
  coursePath: string
  selectedLessonKey?: string
}>()

function lessonLabel(title: string, minutes: number | null): string {
  const duration = formatDuration(minutes)
  return duration ? `${title}, estimated duration ${duration}` : title
}

function lessonPath(lessonStableKey: string): RouteLocationRaw {
  return { path: coursePath, query: { lesson: lessonStableKey } }
}

</script>
