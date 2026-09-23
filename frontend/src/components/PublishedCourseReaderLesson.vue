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

    <section class="published-course-reader-lesson__content" aria-labelledby="published-lesson-content-title">
      <h3 id="published-lesson-content-title">Lesson content</h3>
      <p v-if="content.kind === 'unsupported-schema'" role="note">This lesson uses a content format this version of the Academy cannot display.</p>
      <p v-else-if="content.kind === 'invalid-content'" role="note">This lesson content cannot be displayed safely.</p>
      <p v-else-if="!content.blocks.length">This lesson has no published content yet.</p>
      <div v-else v-for="block in content.blocks" :key="block.key" class="published-course-reader-lesson__block">
        <LessonBlockRenderer :block="block" :published-asset-context="assetContext" :published-assessment="assessmentFor(block)" :attempt-session="attemptSessionFor(block)" />
      </div>
    </section>
  </article>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, onUpdated, ref, watch } from 'vue'
import LessonBlockRenderer from './LessonBlockRenderer.vue'
import type { PublishedCourseVersionDetail } from '../courses/courses'
import { formatDuration } from '../courses/courses'
import { createLearnerAttemptSession, type LearnerAttemptSession } from '../courses/attempts'
import { useAuth } from '../auth/auth'
import { decodeLessonContent, type RenderableBlock } from '../lesson/content'

const props = defineProps<{
	lesson: PublishedCourseVersionDetail['modules'][number]['lessons'][number] | null
	assessments?: PublishedCourseVersionDetail['assessments']
  courseId?: string
  version?: string
}>()
const auth = useAuth()
const lessonTitle = ref<HTMLHeadingElement | null>(null)
const attemptSessions = new Map<string, LearnerAttemptSession>()
let previousLessonStableKey: string | undefined
const content = computed(() => props.lesson ? decodeLessonContent(props.lesson.content) : { kind: 'invalid-content' } as const)
const assetContext = computed(() => props.courseId && props.version ? { courseID: props.courseId, version: props.version } : undefined)

function assessmentFor(block: RenderableBlock) {
  if (block.type !== 'KNOWLEDGE_CHECK') return undefined
	return props.assessments?.find((assessment) => assessment.assessmentKey === block.payload.assessmentKey)
}

function attemptSessionFor(block: RenderableBlock): LearnerAttemptSession | undefined {
  const assessment = assessmentFor(block)
  if (!assessment || !props.courseId || !props.version || auth.state.value.status !== 'authenticated') return undefined
  const key = `${props.courseId}:${props.version}:${assessment.assessmentKey}`
  let session = attemptSessions.get(key)
  if (!session) {
    session = createLearnerAttemptSession({ courseId: props.courseId, version: props.version, assessmentKey: assessment.assessmentKey }, auth)
    attemptSessions.set(key, session)
  }
  return session
}

function disposeAttemptSessions() {
  attemptSessions.forEach((session) => session.dispose())
  attemptSessions.clear()
}
watch(() => `${props.courseId ?? ''}:${props.version ?? ''}:${props.lesson?.stableKey ?? ''}`, disposeAttemptSessions)
onBeforeUnmount(disposeAttemptSessions)

onMounted(() => { previousLessonStableKey = props.lesson?.stableKey })
onUpdated(() => {
  const lessonStableKey = props.lesson?.stableKey
  if (lessonStableKey && previousLessonStableKey && lessonStableKey !== previousLessonStableKey) lessonTitle.value?.focus()
  previousLessonStableKey = lessonStableKey
})
</script>
