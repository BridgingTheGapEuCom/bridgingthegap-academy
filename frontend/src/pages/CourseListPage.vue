<template>
  <BtgPageContainer as="section" class="courses-page" width="reading" aria-labelledby="courses-title">
    <div v-if="state.kind === 'loading'" class="courses-page__state" role="status">
      <h1 id="courses-title">Courses</h1>
      <p>Loading courses…</p>
    </div>

    <div v-else-if="state.kind === 'unavailable'" class="courses-page__state">
      <h1 id="courses-title">Courses unavailable</h1>
      <p>We couldn’t load courses right now. Please try again.</p>
      <BtgButton variant="secondary" @click="load">Try again</BtgButton>
    </div>

    <div v-else class="courses-page__content">
      <p class="courses-page__eyebrow">Bridging the Gap Academy</p>
      <h1 id="courses-title">Courses</h1>
      <p class="courses-page__intro">Structured learning resources for understanding integration and the systems around it.</p>

      <p v-if="state.courses.length === 0" class="courses-page__empty">No courses are available yet.</p>
      <ol v-else class="course-list">
        <li v-for="course in state.courses" :key="course.slug" class="course-list__item">
          <article>
            <p class="course-list__meta">Version {{ course.version.version }}</p>
            <h2><RouterLink :to="`/courses/${course.slug}`">{{ course.version.title }}</RouterLink></h2>
            <p>{{ course.version.description }}</p>
            <p v-if="course.version.source_language" class="course-list__meta">Source language: {{ course.version.source_language }}</p>
            <p v-if="contributorNames(course.version.contributors)" class="course-list__meta">{{ contributorNames(course.version.contributors) }}</p>
          </article>
        </li>
      </ol>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import { listCourses, type CourseSummary, type CourseVersionSummary } from '../courses/courses'

type State =
  | { kind: 'loading' }
  | { kind: 'ready'; courses: CourseSummary[] }
  | { kind: 'unavailable' }

const state = ref<State>({ kind: 'loading' })
let requestVersion = 0
let active = true

onMounted(() => { void load() })
onBeforeUnmount(() => { active = false })

async function load() {
  const version = ++requestVersion
  state.value = { kind: 'loading' }
  try {
    const result = await listCourses()
    if (!active || version !== requestVersion) return
    state.value = { kind: 'ready', courses: result.courses }
  } catch {
    if (!active || version !== requestVersion) return
    state.value = { kind: 'unavailable' }
  }
}

function contributorNames(contributors: CourseVersionSummary['contributors']): string | undefined {
  if (!contributors?.length) return undefined
  return contributors.map((contributor) => contributor.display_name).join(', ')
}
</script>
