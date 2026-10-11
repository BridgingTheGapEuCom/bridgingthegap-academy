<template>
  <ol class="course-list" :aria-label="copy.listLabel">
    <li v-for="course in courses" :key="course.courseId" class="course-list__item">
      <BtgPanel as="article" class="course-list__card">
        <h2><RouterLink :to="publishedCoursePath(course.courseId)">{{ course.title }}</RouterLink></h2>
        <p v-if="course.description">{{ course.description }}</p>
        <BtgMetadataStrip :items="metadata(course)" />
      </BtgPanel>
    </li>
  </ol>
</template>

<script setup lang="ts">
import BtgMetadataStrip from './BtgMetadataStrip.vue'
import BtgPanel from './BtgPanel.vue'
import { publishedCoursePath, type PublishedCourseCatalogItem } from '../courses/courses'
import { languageLabel } from '../i18n/languages'

export interface PublishedCourseCatalogListCopy {
  listLabel: string
  version: string
  language: string
  published: string
  license: string
  contributors: string
}

const props = defineProps<{
  courses: PublishedCourseCatalogItem[]
  copy: PublishedCourseCatalogListCopy
  formatDate: (value: string) => string
}>()

function metadata(course: PublishedCourseCatalogItem) {
  const items = [
    { label: props.copy.version, value: course.version },
    { label: props.copy.language, value: languageLabel(course.sourceLanguage) },
    { label: props.copy.published, value: props.formatDate(course.publishedAt) },
  ]
  if (course.license?.displayName) items.push({ label: props.copy.license, value: course.license.displayName })
  const contributors = course.contributors?.map((contributor) => contributor.displayName).join(', ')
  if (contributors) items.push({ label: props.copy.contributors, value: contributors })
  return items
}
</script>
