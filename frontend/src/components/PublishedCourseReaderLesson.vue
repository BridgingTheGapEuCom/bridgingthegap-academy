<template>
  <article v-if="lesson" class="published-course-reader-lesson" aria-labelledby="published-lesson-title">
    <header>
      <p class="published-course-reader-lesson__eyebrow">{{ copy.lesson }}</p>
      <h2 id="published-lesson-title" ref="lessonTitle" tabindex="-1">{{ lesson.title }}</h2>
      <p v-if="lesson.description">{{ lesson.description }}</p>
      <p v-if="lessonDuration" class="published-course-reader-lesson__duration">
        {{ copy.estimatedDuration(lessonDuration) }}
      </p>
    </header>

    <div v-if="lesson.objectives.length">
      <h3 id="published-lesson-objectives-title">{{ copy.objectives }}</h3>
      <ul><li v-for="objective in lesson.objectives" :key="objective">{{ objective }}</li></ul>
    </div>

    <section class="published-course-reader-lesson__content" aria-labelledby="published-lesson-content-title">
      <h3 id="published-lesson-content-title">{{ copy.content }}</h3>
      <p v-if="content.kind === 'unsupported-schema'" role="note">{{ copy.unsupported }}</p>
      <p v-else-if="content.kind === 'invalid-content'" role="note">{{ copy.invalid }}</p>
      <p v-else-if="!content.blocks.length">{{ copy.empty }}</p>
      <div v-else v-for="block in content.blocks" :key="block.key" class="published-course-reader-lesson__block">
        <LessonBlockRenderer :block="block" :copy="blockCopy" :knowledge-copy="knowledgeCopy" :asset-context="assetContext" :published-assessment="assessmentFor(block)" :attempt-session="attemptSessionFor(block)" :course-id="courseId" :course-version="version" :lesson-key="lesson.stableKey" />
      </div>
    </section>
    <footer v-if="progressSession" class="published-course-reader-lesson__completion">
      <BtgBadge v-if="lessonComplete" variant="success">{{ copy.complete }}</BtgBadge>
      <BtgButton v-else :disabled="progressSession.pending.value" @click="completeLesson">{{ progressSession.pending.value ? copy.completing : copy.markComplete }}</BtgButton>
      <p v-if="progressSession.error.value" role="alert">{{ copy.progressError(progressSession.error.value) }}</p>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, onUpdated, ref, watch } from 'vue'
import LessonBlockRenderer from './LessonBlockRenderer.vue'
import type { PublishedCourseVersionDetail } from '../courses/courses'
import type { LessonBlockRendererCopy } from './LessonBlockRenderer.vue'
import type { LearnerKnowledgeCheckCopy } from './LearnerKnowledgeCheck.vue'
import { createLearnerAttemptSession, type LearnerAttemptSession } from '../courses/attempts'
import type { LearnerProgressError, LearnerProgressSession } from '../courses/progress'
import { useAuth } from '../auth/auth'
import { decodeLessonContent, type RenderableBlock } from '../lesson/content'
import BtgBadge from './BtgBadge.vue'
import BtgButton from './BtgButton.vue'

export type PublishedCourseReaderLessonCopy = {
  lesson: string; objectives: string; content: string; unsupported: string; invalid: string; empty: string
  complete: string; markComplete: string; completing: string
  estimatedDuration: (duration: string) => string
  progressError: (code: LearnerProgressError) => string
}

const props = defineProps<{
	lesson: PublishedCourseVersionDetail['modules'][number]['lessons'][number] | null
	assessments?: PublishedCourseVersionDetail['assessments']
  courseId?: string
  version?: string
  copy: PublishedCourseReaderLessonCopy
  blockCopy: LessonBlockRendererCopy
  knowledgeCopy: LearnerKnowledgeCheckCopy
  formatDuration: (minutes: number) => string
  progressSession?: LearnerProgressSession
}>()
const auth = useAuth()
const lessonTitle = ref<HTMLHeadingElement | null>(null)
const attemptSessions = new Map<string, LearnerAttemptSession>()
let previousLessonStableKey: string | undefined
const content = computed(() => props.lesson ? decodeLessonContent(props.lesson.content) : { kind: 'invalid-content' } as const)
const assetContext = computed(() => props.courseId && props.version ? { kind: 'published' as const, courseID: props.courseId, version: props.version } : undefined)
const lessonDuration = computed(() => props.lesson?.estimatedDurationMinutes ? props.formatDuration(props.lesson.estimatedDurationMinutes) : '')
const lessonComplete = computed(() => Boolean(props.lesson && props.progressSession?.progress.value?.completedLessonKeys.includes(props.lesson.stableKey)))

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
    void session.reload()
  }
  return session
}
async function completeLesson() { if (props.lesson) await props.progressSession?.complete(props.lesson.stableKey) }

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
