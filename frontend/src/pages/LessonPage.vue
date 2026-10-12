<template>
  <BtgPageContainer as="article" class="lesson-page" aria-labelledby="lesson-title">
    <div v-if="state.kind === 'loading'" class="lesson-page__state" role="status">
      <h1 id="lesson-title">{{ t('learner.reader.lessonLoadingTitle') }}</h1>
      <p>{{ t('learner.reader.lessonLoadingDescription') }}</p>
    </div>

    <div v-else-if="state.kind === 'not-found'" class="lesson-page__state">
      <h1 id="lesson-title">{{ t('learner.reader.lessonNotFound') }}</h1>
      <p>{{ t('learner.reader.lessonNotFoundDescription') }}</p>
      <RouterLink to="/courses">{{ t('learner.reader.browse') }}</RouterLink>
    </div>

    <div v-else-if="state.kind === 'unavailable'" class="lesson-page__state">
      <h1 id="lesson-title">{{ t('learner.reader.lessonUnavailable') }}</h1>
      <p>{{ t('learner.reader.lessonUnavailableDescription') }}</p>
      <BtgButton variant="secondary" @click="load">{{ t('common.actions.tryAgain') }}</BtgButton>
    </div>

    <div v-else-if="state.kind === 'unsupported'" class="lesson-page__state" role="alert">
      <h1 id="lesson-title">{{ t('learner.reader.formatUnavailable') }}</h1>
      <p>{{ t('learner.reader.formatUnavailableDescription') }}</p>
      <RouterLink to="/courses">{{ t('learner.reader.browse') }}</RouterLink>
    </div>

    <div v-else class="lesson-page__content">
      <nav :aria-label="t('learner.reader.lessonSequence')"><RouterLink :to="`/courses/${encodeURIComponent(state.lesson.course.slug)}`">{{ t('learner.reader.backToCourse') }}</RouterLink></nav>
      <header class="lesson-page__header">
        <p class="lesson-page__eyebrow">{{ state.lesson.module.title }}</p>
        <h1 id="lesson-title">{{ state.lesson.lesson.title }}</h1>
        <p v-if="state.lesson.lesson.description" class="lesson-page__description">{{ state.lesson.lesson.description }}</p>
        <BtgMetadataStrip :items="lessonMetadata" />
        <p v-if="state.lesson.version.status === 'DEPRECATED'" class="lesson-page__notice" role="status">{{ t('learner.reader.deprecated') }}</p>
        <p v-else-if="state.lesson.version.status === 'ARCHIVED'" class="lesson-page__notice lesson-page__notice--archived" role="status">{{ t('learner.reader.archived') }}</p>
      </header>

      <section v-if="state.lesson.lesson.objectives?.length" aria-labelledby="lesson-objectives-title">
        <h2 id="lesson-objectives-title">{{ t('learner.reader.lessonObjectives') }}</h2>
        <ul><li v-for="objective in state.lesson.lesson.objectives" :key="objective">{{ objective }}</li></ul>
      </section>

      <section v-if="state.lesson.lesson.recommended_prerequisite_keys?.length" aria-labelledby="lesson-prerequisites-title">
        <h2 id="lesson-prerequisites-title">{{ t('learner.reader.prerequisites') }}</h2>
        <ul><li v-for="key in state.lesson.lesson.recommended_prerequisite_keys" :key="key">{{ prerequisiteName(key) }}</li></ul>
      </section>

      <fieldset v-if="state.content.blocks.length" class="lesson-mode-control">
        <legend>{{ t('learner.reader.readingMode') }}</legend>
        <BtgRadio v-model="mode" name="lesson-reading-mode" value="continuous" :label="t('learner.reader.continuous')" />
        <BtgRadio v-model="mode" name="lesson-reading-mode" value="focus" :label="t('learner.reader.focus')" />
      </fieldset>

      <section v-if="!state.content.blocks.length" class="lesson-content" :aria-label="t('learner.reader.lessonContent')">
        <p>{{ t('learner.reader.emptyContent') }}</p>
      </section>

      <section v-else-if="mode === 'continuous'" class="lesson-content" :aria-label="t('learner.reader.lessonContent')">
        <div v-for="block in state.content.blocks" :id="`lesson-block-${block.key}`" :key="block.key" class="lesson-content__block">
          <LessonBlockRenderer :block="block" :copy="blockCopy" :knowledge-copy="knowledgeCopy" :asset-context="assetContext" :course-id="state.lesson.course.courseId" :course-version="state.lesson.version.version" :lesson-key="state.lesson.lesson.key" />
        </div>
      </section>

      <section v-else class="lesson-focus" aria-labelledby="focus-title">
        <h2 id="focus-title">{{ t('learner.reader.focusMode') }}</h2>
        <p class="lesson-focus__position" aria-live="polite">{{ t('learner.reader.blockPosition', { current: focusIndex + 1, total: state.content.blocks.length }) }}</p>
        <div class="lesson-focus__block" tabindex="-1">
          <LessonBlockRenderer :block="state.content.blocks[focusIndex]" :copy="blockCopy" :knowledge-copy="knowledgeCopy" :asset-context="assetContext" :course-id="state.lesson.course.courseId" :course-version="state.lesson.version.version" :lesson-key="state.lesson.lesson.key" />
        </div>
        <div class="lesson-focus__actions">
          <BtgButton variant="secondary" :disabled="focusIndex === 0" @click="previousBlock">{{ t('learner.reader.previousBlock') }}</BtgButton>
          <BtgButton :disabled="focusIndex === state.content.blocks.length - 1" @click="nextBlock">{{ t('learner.reader.nextBlock') }}</BtgButton>
        </div>
      </section>

      <BtgPreviousNextNavigation v-if="neighbors.previous || neighbors.next" :navigation-label="t('learner.reader.lessonSequence')" :previous-label="t('learner.reader.previousLesson')" :next-label="t('learner.reader.nextLesson')" :previous-disabled="!neighbors.previous" :next-disabled="!neighbors.next" @previous="goToNeighbor(neighbors.previous)" @next="goToNeighbor(neighbors.next)" />
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter, type RouteLocationRaw } from 'vue-router'
import { APIProblemError } from '../api/client'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgMetadataStrip from '../components/BtgMetadataStrip.vue'
import BtgPreviousNextNavigation from '../components/BtgPreviousNextNavigation.vue'
import BtgRadio from '../components/BtgRadio.vue'
import LessonBlockRenderer from '../components/LessonBlockRenderer.vue'
import { getCourseVersion, getLesson, InvalidCourseRouteError, lessonPath, type CourseDetail, type LessonDetail } from '../courses/courses'
import { decodeLessonContent, type DecodedLessonContent } from '../lesson/content'
import { useApplicationFormatters } from '../i18n/formatters'
import { useLearnerContentCopy } from '../learner/copy'

type ReadyState = { kind: 'ready'; lesson: LessonDetail; structure?: CourseDetail; content: Extract<DecodedLessonContent, { kind: 'ready' }> }
type State = { kind: 'loading' } | ReadyState | { kind: 'not-found' } | { kind: 'unavailable' } | { kind: 'unsupported' }

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const { duration } = useApplicationFormatters()
const { blockCopy, knowledgeCopy } = useLearnerContentCopy()
const state = ref<State>({ kind: 'loading' })
const mode = ref<'continuous' | 'focus'>('continuous')
const focusIndex = ref(0)
let requestVersion = 0
let active = true

const assetContext = computed(() => {
  const ready = state.value
  if (ready.kind !== 'ready' || !ready.lesson.course.courseId || !ready.lesson.version.version) return undefined
  return { kind: 'published' as const, courseID: ready.lesson.course.courseId, version: ready.lesson.version.version }
})
const lessonMetadata = computed(() => {
  if (state.value.kind !== 'ready') return []
  const minutes = state.value.lesson.lesson.estimated_duration_minutes
  return [
    { label: t('learner.reader.version'), value: state.value.lesson.version.version },
    ...(minutes ? [{ label: t('learner.reader.estimatedDurationLabel'), value: duration(minutes) }] : []),
  ]
})

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
function goToNeighbor(target?: RouteLocationRaw) { if (target) void router.push(target) }

function prerequisiteName(key: string): string {
  if (state.value.kind !== 'ready' || !state.value.structure) return key
  return state.value.structure.modules.flatMap((module) => module.lessons).find((lesson) => lesson.key === key)?.title ?? key
}
</script>
