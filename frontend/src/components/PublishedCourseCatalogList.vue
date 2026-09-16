<template>
  <ol class="course-list" aria-label="Published courses">
    <li v-for="course in courses" :key="course.courseId" class="course-list__item">
      <article>
        <h2><RouterLink :to="publishedCoursePath(course.courseId)">{{ course.title }}</RouterLink></h2>
        <p v-if="course.description">{{ course.description }}</p>
        <dl class="course-list__metadata">
          <div><dt>Version</dt><dd>{{ course.version }}</dd></div>
          <div><dt>Language</dt><dd>{{ course.sourceLanguage }}</dd></div>
          <div><dt>Published</dt><dd><time :datetime="course.publishedAt">{{ publishedDate(course.publishedAt) }}</time></dd></div>
          <div v-if="course.license?.displayName"><dt>License</dt><dd>{{ course.license.displayName }}</dd></div>
        </dl>
        <p v-if="contributorNames(course.contributors)" class="course-list__meta">{{ contributorNames(course.contributors) }}</p>
      </article>
    </li>
  </ol>
</template>

<script setup lang="ts">
import { publishedCoursePath, type PublishedCourseCatalogItem } from '../courses/courses'

defineProps<{ courses: PublishedCourseCatalogItem[] }>()

function contributorNames(contributors: PublishedCourseCatalogItem['contributors']): string | undefined {
  if (!contributors?.length) return undefined
  return contributors.map((contributor) => contributor.displayName).join(', ')
}

function publishedDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeZone: 'UTC' }).format(date)
}
</script>
