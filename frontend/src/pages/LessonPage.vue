<template>
  <BtgPageContainer as="article" class="lesson-page" width="reading" aria-labelledby="lesson-title">
    <div v-if="state.kind === 'loading'" class="lesson-page__state" role="status">
      <h1 id="lesson-title">Loading lesson…</h1>
      <p>One moment while we load this lesson.</p>
    </div>

    <div v-else-if="state.kind === 'not-found'" class="lesson-page__state">
      <h1 id="lesson-title">Lesson not found</h1>
      <p>This lesson is not available.</p>
      <RouterLink to="/courses">Browse courses</RouterLink>
    </div>

    <div v-else-if="state.kind === 'unavailable'" class="lesson-page__state">
      <h1 id="lesson-title">Lesson unavailable</h1>
      <p>We couldn’t load this lesson right now. Please try again.</p>
      <BtgButton variant="secondary" @click="load">Try again</BtgButton>
    </div>

    <div v-else-if="state.kind === 'unsupported'" class="lesson-page__state" role="alert">
      <h1 id="lesson-title">Lesson format unavailable</h1>
      <p>This lesson uses a content format this version of the Academy cannot display.</p>
      <RouterLink to="/courses">Browse courses</RouterLink>
    </div>

    <div v-else class="lesson-page__content">
      <nav aria-label="Lesson navigation"><RouterLink :to="`/courses/${encodeURIComponent(state.lesson.course.slug)}`">Back to course</RouterLink></nav>
      <header class="lesson-page__header">
        <p class="lesson-page__eyebrow">{{ state.lesson.module.title }}</p>
        <h1 id="lesson-title">{{ state.lesson.lesson.title }}</h1>
        <p v-if="state.lesson.lesson.description" class="lesson-page__description">{{ state.lesson.lesson.description }}</p>
        <dl class="lesson-page__metadata">
          <div><dt>Version</dt><dd>{{ state.lesson.version.version }}</dd></div>
          <div v-if="formatDuration(state.lesson.lesson.estimated_duration_minutes)"><dt>Estimated duration</dt><dd>{{ formatDuration(state.lesson.lesson.estimated_duration_minutes) }}</dd></div>
        </dl>
        <p v-if="state.lesson.version.status === 'DEPRECATED'" class="lesson-page__notice" role="status">This lesson is from a deprecated course version.</p>
        <p v-else-if="state.lesson.version.status === 'ARCHIVED'" class="lesson-page__notice lesson-page__notice--archived" role="status">This lesson is from an archived historical course version.</p>
      </header>

      <section v-if="state.lesson.lesson.objectives?.length" aria-labelledby="lesson-objectives-title">
        <h2 id="lesson-objectives-title">Lesson objectives</h2>
        <ul><li v-for="objective in state.lesson.lesson.objectives" :key="objective">{{ objective }}</li></ul>
      </section>

      <section v-if="state.lesson.lesson.recommended_prerequisite_keys?.length" aria-labelledby="lesson-prerequisites-title">
        <h2 id="lesson-prerequisites-title">Recommended before this lesson</h2>
        <ul><li v-for="key in state.lesson.lesson.recommended_prerequisite_keys" :key="key">{{ prerequisiteName(key) }}</li></ul>
      </section>

      <fieldset v-if="state.content.blocks.length" class="lesson-mode-control">
        <legend>Reading mode</legend>
        <label><input v-model="mode" type="radio" value="continuous" /> Continuous</label>
        <label><input v-model="mode" type="radio" value="focus" /> Focus</label>
      </fieldset>

      <section v-if="!state.content.blocks.length" class="lesson-content" aria-label="Lesson content">
        <p>This lesson has no published content yet.</p>
      </section>

      <section v-else-if="mode === 'continuous'" class="lesson-content" aria-label="Lesson content">
        <div v-for="block in state.content.blocks" :id="`lesson-block-${block.key}`" :key="block.key" class="lesson-content__block">
          <LessonBlockRenderer :block="block" :course-id="state.lesson.course.courseId" :course-version="state.lesson.version.version" :lesson-key="state.lesson.lesson.key" />
        </div>
      </section>

      <section v-else class="lesson-focus" aria-labelledby="focus-title">
        <h2 id="focus-title">Focus mode</h2>
        <p class="lesson-focus__position" aria-live="polite">Block {{ focusIndex + 1 }} of {{ state.content.blocks.length }}</p>
        <div class="lesson-focus__block" tabindex="-1">
          <LessonBlockRenderer :block="state.content.blocks[focusIndex]" :course-id="state.lesson.course.courseId" :course-version="state.lesson.version.version" :lesson-key="state.lesson.lesson.key" />
        </div>
        <div class="lesson-focus__actions">
          <BtgButton variant="secondary" :disabled="focusIndex === 0" @click="previousBlock">Previous block</BtgButton>
          <BtgButton :disabled="focusIndex === state.content.blocks.length - 1" @click="nextBlock">Next block</BtgButton>
        </div>
      </section>

      <nav v-if="neighbors.previous || neighbors.next" class="lesson-page__neighbors" aria-label="Lesson sequence">
        <RouterLink v-if="neighbors.previous" :to="neighbors.previous">Previous lesson</RouterLink>
        <RouterLink v-if="neighbors.next" :to="neighbors.next">Next lesson</RouterLink>
      </nav>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { APIProblemError } from '../api/client'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import LessonBlockRenderer from '../components/LessonBlockRenderer.vue'
import { formatDuration, getCourseVersion, getLesson, InvalidCourseRouteError, lessonPath, type CourseDetail, type LessonDetail } from '../courses/courses'
import { decodeLessonContent, type DecodedLessonContent } from '../lesson/content'

type ReadyState = { kind: 'ready'; lesson: LessonDetail; structure?: CourseDetail; content: Extract<DecodedLessonContent, { kind: 'ready' }> }
type State = { kind: 'loading' } | ReadyState | { kind: 'not-found' } | { kind: 'unavailable' } | { kind: 'unsupported' }

const route = useRoute()
const state = ref<State>({ kind: 'loading' })
const mode = ref<'continuous' | 'focus'>('continuous')
const focusIndex = ref(0)
let requestVersion = 0
let active = true

const neighbors = computed(() => {
  const ready = state.value
  if (ready.kind !== 'ready' || !ready.structure || !ready.lesson.version.version) return {}
  const lessons = ready.structure.modules.flatMap((module) => module.lessons)
  const index = lessons.findIndex((lesson) => lesson.key === ready.lesson.lesson.key)
  if (index < 0) return {}
  const { slug } = ready.lesson.course
  const { version } = ready.lesson.version
  return {
    previous: lessons[index - 1] ? lessonPath(slug, version, lessons[index - 1].key) : undefined,
    next: lessons[index + 1] ? lessonPath(slug, version, lessons[index + 1].key) : undefined,
  }
})

watch(() => [route.params.slug, route.params.version, route.params.lessonKey], () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false })

async function load() {
  const generation = ++requestVersion
  const slug = routeParameter('slug')
  const version = routeParameter('version')
  const lessonKey = routeParameter('lessonKey')
  state.value = { kind: 'loading' }
  mode.value = 'continuous'
  focusIndex.value = 0
  try {
    const lesson = await getLesson(slug, version, lessonKey)
    if (!active || generation !== requestVersion) return
    const content = decodeLessonContent(lesson.lesson.content)
    state.value = content.kind === 'ready' ? { kind: 'ready', lesson, content } : { kind: 'unsupported' }
    if (content.kind === 'ready') {
      void getCourseVersion(slug, version).then((structure) => {
        if (active && generation === requestVersion && state.value.kind === 'ready') {
          state.value = { ...state.value, structure }
        }
      }).catch(() => { /* Optional sequence context must not hide a loaded lesson. */ })
    }
  } catch (error) {
    if (!active || generation !== requestVersion) return
    state.value = error instanceof InvalidCourseRouteError || (error instanceof APIProblemError && error.status === 404) ? { kind: 'not-found' } : { kind: 'unavailable' }
  }
}

function routeParameter(name: string): string {
  return typeof route.params[name] === 'string' ? route.params[name] : ''
}

function previousBlock() { if (focusIndex.value > 0) focusIndex.value -= 1 }
function nextBlock() {
  if (state.value.kind === 'ready' && focusIndex.value < state.value.content.blocks.length - 1) focusIndex.value += 1
}

function prerequisiteName(key: string): string {
  if (state.value.kind !== 'ready' || !state.value.structure) return key
  return state.value.structure.modules.flatMap((module) => module.lessons).find((lesson) => lesson.key === key)?.title ?? key
}
</script>
