<template>
  <BtgPageContainer as="article" class="course-overview" width="reading" aria-labelledby="course-title">
    <div v-if="state.kind === 'loading'" class="course-overview__state" role="status">
      <h1 id="course-title">Loading course…</h1>
      <p>One moment while we load this course.</p>
    </div>

    <div v-else-if="state.kind === 'not-found'" class="course-overview__state">
      <h1 id="course-title">Course not found</h1>
      <p>This course is not available.</p>
      <RouterLink to="/courses">Browse courses</RouterLink>
    </div>

    <div v-else-if="state.kind === 'unavailable'" class="course-overview__state">
      <h1 id="course-title">Course unavailable</h1>
      <p>We couldn’t load this course right now. Please try again.</p>
      <BtgButton variant="secondary" @click="load">Try again</BtgButton>
    </div>

    <div v-else class="course-overview__content">
      <header class="course-overview__header">
        <p class="course-overview__eyebrow">Bridging the Gap Academy</p>
        <h1 id="course-title">{{ state.course.version.title }}</h1>
        <p class="course-overview__description">{{ state.course.version.description }}</p>
        <dl class="course-overview__metadata">
          <div><dt>Version</dt><dd>{{ state.course.version.version }}</dd></div>
          <div v-if="state.course.version.source_language"><dt>Source language</dt><dd>{{ state.course.version.source_language }}</dd></div>
        </dl>
        <p v-if="state.course.version.status && state.course.version.status !== 'PUBLISHED'" class="course-overview__notice" role="status">This course version is {{ statusLabel(state.course.version.status) }}.</p>
      </header>

      <section v-if="state.course.version.objectives?.length" aria-labelledby="objectives-title">
        <h2 id="objectives-title">What you’ll learn</h2>
        <ul><li v-for="objective in state.course.version.objectives" :key="objective">{{ objective }}</li></ul>
      </section>

      <section aria-labelledby="outline-title">
        <h2 id="outline-title">Course outline</h2>
        <div class="course-outline">
          <section v-for="structure in state.course.modules" :key="structure.module.key" class="course-outline__module" :aria-labelledby="`module-${structure.module.key}`">
            <h3 :id="`module-${structure.module.key}`">{{ structure.module.title }}</h3>
            <p v-if="structure.module.description" class="course-outline__module-description">{{ structure.module.description }}</p>
            <ol class="course-outline__lessons">
              <li v-for="lesson in structure.lessons" :key="lesson.key" class="course-outline__lesson">
                <article>
                  <h4><RouterLink :to="lessonPath(state.course.course.slug, versionText, lesson.key)">{{ lesson.title }}</RouterLink></h4>
                  <p v-if="lesson.description">{{ lesson.description }}</p>
                  <p v-if="formatDuration(lesson.estimated_duration_minutes)" class="course-outline__meta">Estimated duration: {{ formatDuration(lesson.estimated_duration_minutes) }}</p>
                  <p v-if="lesson.recommended_prerequisite_keys.length" class="course-outline__prerequisites">
                    Recommended before this lesson: {{ prerequisiteNames(lesson.recommended_prerequisite_keys).join(', ') }}
                  </p>
                </article>
              </li>
            </ol>
          </section>
        </div>
      </section>

      <footer class="course-overview__footer">
        <section v-if="state.course.version.contributors?.length" aria-labelledby="contributors-title">
          <h2 id="contributors-title">Contributors</h2>
          <ul class="course-overview__contributors"><li v-for="contributor in state.course.version.contributors" :key="`${contributor.order}-${contributor.display_name}`">{{ contributor.display_name }} <span>({{ contributorRole(contributor.role) }})</span></li></ul>
        </section>
        <section v-if="state.course.version.license" aria-labelledby="license-title">
          <h2 id="license-title">Course content license</h2>
          <p>
            <a v-if="safeLicenseURL(state.course.version.license.url)" :href="safeLicenseURL(state.course.version.license.url)">{{ state.course.version.license.display_name }}</a>
            <span v-else>{{ state.course.version.license.display_name }}</span>
            <template v-if="state.course.version.license.identifier"> ({{ state.course.version.license.identifier }})</template>
          </p>
        </section>
      </footer>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { APIProblemError } from '../api/client'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import { formatDuration, getCourse, InvalidCourseSlugError, lessonPath, type CourseDetail } from '../courses/courses'

type State = { kind: 'loading' } | { kind: 'ready'; course: CourseDetail } | { kind: 'not-found' } | { kind: 'unavailable' }

const route = useRoute()
const state = ref<State>({ kind: 'loading' })
const versionText = computed(() => state.value.kind === 'ready' ? state.value.course.version.version ?? '' : '')
let requestVersion = 0
let active = true

watch(() => route.params.slug, () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false })

async function load() {
  const version = ++requestVersion
  const slug = typeof route.params.slug === 'string' ? route.params.slug : ''
  state.value = { kind: 'loading' }
  try {
    const course = await getCourse(slug)
    if (!active || version !== requestVersion) return
    state.value = { kind: 'ready', course }
  } catch (error) {
    if (!active || version !== requestVersion) return
    state.value = error instanceof InvalidCourseSlugError || (error instanceof APIProblemError && error.status === 404) ? { kind: 'not-found' } : { kind: 'unavailable' }
  }
}

function prerequisiteNames(keys: string[]): string[] {
  if (state.value.kind !== 'ready') return keys
  const titles = new Map(state.value.course.modules.flatMap((module) => module.lessons.map((lesson) => [lesson.key, lesson.title])))
  return keys.map((key) => titles.get(key) ?? key)
}

function contributorRole(role: 'AUTHOR' | 'MAINTAINER'): string {
  return role === 'AUTHOR' ? 'Author' : 'Maintainer'
}

function statusLabel(status: string | undefined): string {
  return status === 'DEPRECATED' ? 'deprecated' : 'archived'
}

function safeLicenseURL(value: string | undefined): string | undefined {
  if (!value) return undefined
  try {
    return new URL(value).protocol === 'https:' ? value : undefined
  } catch {
    return undefined
  }
}
</script>
