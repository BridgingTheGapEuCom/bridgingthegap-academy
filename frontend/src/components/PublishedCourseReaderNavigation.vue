<template>
  <nav class="published-course-reader-navigation" :aria-label="copy.navigationLabel">
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
            :class="{ 'is-complete': completedLessonKeys.includes(lesson.stableKey) }"
          >
            <span>{{ lesson.title }}</span>
            <span v-if="lessonDuration(lesson.estimatedDurationMinutes)" class="published-course-reader-navigation__duration">
              {{ lessonDuration(lesson.estimatedDurationMinutes) }}
            </span>
            <BtgBadge v-if="completedLessonKeys.includes(lesson.stableKey)" variant="success">{{ copy.completed }}</BtgBadge>
          </RouterLink>
        </li>
      </ol>
      <p v-else class="published-course-reader-navigation__empty">{{ copy.emptyModule }}</p>
    </section>
  </nav>
</template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import type { PublishedCourseVersionDetail } from '../courses/courses'
import BtgBadge from './BtgBadge.vue'

export type PublishedCourseReaderNavigationCopy = { navigationLabel: string; emptyModule: string; completed: string; lessonDuration: (title: string, duration: string) => string }

const { coursePath, copy, formatDuration: duration } = defineProps<{
  modules: PublishedCourseVersionDetail['modules']
  coursePath: string
  selectedLessonKey?: string
  completedLessonKeys: string[]
  copy: PublishedCourseReaderNavigationCopy
  formatDuration: (minutes: number) => string
}>()

function lessonLabel(title: string, minutes: number | null): string {
  const formatted = lessonDuration(minutes)
  return formatted ? copy.lessonDuration(title, formatted) : title
}

function lessonDuration(minutes: number | null): string { return minutes ? duration(minutes) : '' }

function lessonPath(lessonStableKey: string): RouteLocationRaw {
  return { path: coursePath, query: { lesson: lessonStableKey } }
}

</script>
