<template>
  <BtgPageContainer as="section" class="published-course-reader" aria-labelledby="published-course-title">
    <div v-if="state.kind === 'loading'" class="published-course-reader__state" role="status" aria-live="polite">
      <h1 id="published-course-title" ref="courseTitle" tabindex="-1">{{ t('learner.reader.loadingTitle') }}</h1>
      <p>{{ t('learner.reader.loadingDescription') }}</p>
    </div>

    <div v-else-if="state.kind === 'not-found'" class="published-course-reader__state">
      <h1 id="published-course-title" ref="courseTitle" tabindex="-1">{{ t('learner.reader.courseNotFound') }}</h1>
      <p v-if="translationUnavailable">{{ t('learner.reader.translationUnavailable') }}</p><p v-else>{{ t('learner.reader.courseNotFoundDescription') }}</p>
      <BtgButton v-if="translationUnavailable" variant="secondary" @click="returnToSource">{{ t('learner.reader.sourceVersion') }}</BtgButton><RouterLink v-else to="/courses">{{ t('learner.reader.browse') }}</RouterLink>
    </div>

    <div v-else-if="state.kind === 'unavailable'" class="published-course-reader__state" role="alert">
      <h1 id="published-course-title" ref="courseTitle" tabindex="-1">{{ t('learner.reader.courseUnavailable') }}</h1>
      <p>{{ t('learner.reader.courseUnavailableDescription') }}</p>
      <BtgButton variant="secondary" @click="retryLoad">{{ t('common.actions.tryAgain') }}</BtgButton>
    </div>

    <template v-else>
      <header class="published-course-reader__header">
        <RouterLink class="published-course-reader__back" to="/courses">{{ t('learner.reader.browse') }}</RouterLink>
        <p class="published-course-reader__eyebrow">{{ t('learner.reader.eyebrow') }}</p>
        <h1 id="published-course-title" ref="courseTitle" tabindex="-1">{{ state.course.title }}</h1>
        <p v-if="state.course.description" class="published-course-reader__description">{{ state.course.description }}</p>
        <BtgFormField v-if="languages.length > 1" :label="t('learner.reader.courseLanguage')" :description="t('learner.reader.courseLanguageHelp')" control-id="course-language" v-slot="field"><BtgSelect id="course-language" :model-value="selectedLanguage" :aria-describedby="field.describedBy" @update:model-value="changeLanguage"><option v-for="language in languages" :key="language.language" :value="language.language">{{ languageLabel(language.language) }}</option></BtgSelect></BtgFormField>
        <p v-if="translationUnavailable" role="alert">{{ t('learner.reader.translationUnavailable') }} <BtgButton variant="tertiary" @click="returnToSource">{{ t('learner.reader.sourceVersion') }}</BtgButton></p>
        <aside v-if="lagNotice" role="status"><p>{{ t('learner.reader.sourceLag', { language: languageLabel(selectedLanguage), translated: lagNotice.translated, latest: lagNotice.latest }) }}</p><BtgButton variant="secondary" @click="openLatestSource">{{ t('learner.reader.viewLatest', { version: lagNotice.latest, language: languageLabel(sourceLanguage) }) }}</BtgButton></aside>
        <div class="published-course-reader__actions"><RouterLink :to="`/courses/by-id/${state.course.courseId}/community`">{{ t('learner.reader.discussions') }}</RouterLink><BtgButton v-if="auth.state.value.status === 'authenticated'" variant="secondary" @click="claimCertificate" :disabled="claiming">{{ claiming ? t('learner.reader.requestingCertificate') : t('learner.reader.certificate') }}</BtgButton></div><p v-if="certificateMessage" role="status">{{ certificateMessage }}</p>
        <BtgMetadataStrip :items="courseMetadata" />
        <BtgMetadataStrip v-if="progressSession?.progress.value" :items="progressMetadata" />
        <p v-if="progressSession?.error.value === 'load'" role="alert">{{ t('learner.progress.loadError') }}</p>
        <section v-if="state.course.objectives.length" aria-labelledby="published-course-objectives-title">
          <h2 id="published-course-objectives-title">{{ t('learner.reader.objectives') }}</h2>
          <ul><li v-for="objective in state.course.objectives" :key="objective">{{ objective }}</li></ul>
        </section>
        <section v-if="state.course.contributors.length" aria-labelledby="published-course-contributors-title">
          <h2 id="published-course-contributors-title">{{ t('learner.reader.contributors') }}</h2>
          <ul class="published-course-reader__contributors">
            <li v-for="contributor in state.course.contributors" :key="`${contributor.order}-${contributor.displayName}`">
              {{ contributor.displayName }} <span>({{ contributorRole(contributor.role) }})</span>
            </li>
          </ul>
        </section>
      </header>

      <div class="published-course-reader__layout">
        <PublishedCourseReaderNavigation
          :modules="state.course.modules"
          :course-path="route.path"
          :selected-lesson-key="state.selectedLesson?.stableKey"
          :completed-lesson-keys="progressSession?.progress.value?.completedLessonKeys ?? []"
          :copy="navigationCopy"
          :format-duration="duration"
        />
        <div class="published-course-reader__lesson-view">
          <PublishedCourseReaderLesson :lesson="state.selectedLesson" :assessments="state.course.assessments" :course-id="state.course.courseId" :version="state.course.version" :copy="lessonCopy" :block-copy="blockCopy" :knowledge-copy="knowledgeCopy" :format-duration="duration" :progress-session="progressSession" />
          <BtgPreviousNextNavigation v-if="state.selectedLesson" :navigation-label="t('learner.reader.lessonSequence')" :previous-label="t('learner.reader.previousLesson')" :next-label="t('learner.reader.nextLesson')" :previous-disabled="!lessonNeighbors.previous" :next-disabled="!lessonNeighbors.next" @previous="selectLesson(lessonNeighbors.previous)" @next="selectLesson(lessonNeighbors.next)" />
          <p v-if="!state.selectedLesson" class="published-course-reader__empty" role="status">{{ t('learner.reader.noLessonsCourse') }}</p>
        </div>
      </div>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import { useAuth } from '../auth/auth'
import { issueCertificate, certificatePath } from '../courses/certificates'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgMetadataStrip from '../components/BtgMetadataStrip.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPreviousNextNavigation from '../components/BtgPreviousNextNavigation.vue'
import BtgSelect from '../components/BtgSelect.vue'
import { languageLabel } from '../i18n/languages'
import PublishedCourseReaderLesson from '../components/PublishedCourseReaderLesson.vue'
import PublishedCourseReaderNavigation from '../components/PublishedCourseReaderNavigation.vue'
import {
  getLatestPublishedCourse,
  getPublishedCourseVersionByID,
  InvalidCourseRouteError,
  isCourseVersion,
  publishedLessonKeyFromRoute,
  type PublishedCourseVersionDetail,
} from '../courses/courses'
import { getCourseLanguages, getTranslatedCourse, type CourseLanguages, type TranslatedCourse } from '../translations/learner'
import { useApplicationFormatters } from '../i18n/formatters'
import { useLearnerContentCopy } from '../learner/copy'
import { createLearnerProgressSession, type LearnerProgressError, type LearnerProgressSession } from '../courses/progress'

type PublishedLesson = PublishedCourseVersionDetail['modules'][number]['lessons'][number]
type State =
  | { kind: 'loading' }
  | { kind: 'ready'; course: PublishedCourseVersionDetail; selectedLesson: PublishedLesson | null }
  | { kind: 'not-found' }
  | { kind: 'unavailable' }

const route = useRoute()
const { t } = useI18n()
const { date, duration, number } = useApplicationFormatters()
const { blockCopy, knowledgeCopy } = useLearnerContentCopy()
const auth = useAuth()
const router = useRouter()
const state = ref<State>({ kind: 'loading' })
const courseTitle = ref<HTMLHeadingElement | null>(null)
const languages = ref<CourseLanguages['languages']>([])
const translation = ref<TranslatedCourse>()
const translationUnavailable = ref(false)
const selectedLanguage = computed(() => typeof route.query.lang === 'string' ? route.query.lang : '')
const sourceLanguage = computed(() => translation.value?.translation.sourceLanguage ?? (state.value.kind === 'ready' ? state.value.course.sourceLanguage : ''))
const lagNotice = computed(() => translation.value?.sourceLag.isLatest === false && translation.value.sourceLag.latestSourceVersion ? { translated: translation.value.sourceLag.translatedSourceVersion, latest: translation.value.sourceLag.latestSourceVersion } : undefined)
let requestVersion = 0
let active = true
const claiming = ref(false)
const certificateMessage = ref('')
const progressSession = shallowRef<LearnerProgressSession>()
const lessons = computed(() => state.value.kind === 'ready' ? state.value.course.modules.flatMap((module) => module.lessons) : [])
const lessonNeighbors = computed(() => {
  const index = lessons.value.findIndex((lesson) => lesson.stableKey === (state.value.kind === 'ready' ? state.value.selectedLesson?.stableKey : undefined))
  return { previous: index > 0 ? lessons.value[index - 1]?.stableKey : undefined, next: index >= 0 ? lessons.value[index + 1]?.stableKey : undefined }
})
const navigationCopy = computed(() => ({ navigationLabel: t('learner.reader.lessonsNavigation'), emptyModule: t('learner.reader.noLessonsModule'), completed: t('learner.progress.completed'), lessonDuration: (title: string, formatted: string) => t('learner.reader.lessonLabelDuration', { title, duration: formatted }) }))
const lessonCopy = computed(() => ({
  lesson: t('learner.reader.lesson'), objectives: t('learner.reader.objectives'), content: t('learner.reader.lessonContent'), unsupported: t('learner.reader.unsupportedContent'), invalid: t('learner.reader.invalidContent'), empty: t('learner.reader.emptyContent'),
  complete: t('learner.progress.lessonComplete'), markComplete: t('learner.progress.markComplete'), completing: t('learner.progress.markingComplete'), estimatedDuration: (formatted: string) => t('learner.reader.estimatedDuration', { duration: formatted }), progressError: (code: LearnerProgressError) => t(`learner.progress.${code === 'load' ? 'loadError' : code === 'save' ? 'saveError' : code}`),
}))
const courseMetadata = computed(() => state.value.kind === 'ready' ? [
  { label: t('learner.reader.version'), value: state.value.course.version },
  ...(state.value.course.sourceLanguage ? [{ label: t('learner.reader.sourceLanguage'), value: languageLabel(state.value.course.sourceLanguage) }] : []),
  { label: t('learner.reader.published'), value: date(state.value.course.publishedAt, 'date') },
  ...(state.value.course.license.displayName ? [{ label: t('learner.reader.license'), value: state.value.course.license.displayName }] : []),
] : [])
const progressMetadata = computed(() => [{ label: t('learner.progress.label'), value: t('learner.progress.summary', { count: number(progressSession.value?.completedCount.value ?? 0, 'count'), total: number(lessons.value.length, 'count') }, progressSession.value?.completedCount.value ?? 0) }])

watch([() => route.params.courseId, () => route.params.version, () => route.query.lang], () => { void load() }, { immediate: true })
watch(() => route.query.lesson, () => {
  if (state.value.kind !== 'ready') return
  reconcileLessonSelection(state.value.course)
}, { flush: 'post' })

onBeforeUnmount(() => { active = false; progressSession.value?.dispose() })

async function load(): Promise<boolean> {
  const generation = ++requestVersion
  const courseID = routeParameter('courseId')
  const version = routeParameter('version')
  state.value = { kind: 'loading' }

  try {
    translationUnavailable.value = false
    translation.value = undefined
    const discovered = version ? getCourseLanguages(courseID, version).catch(() => undefined) : Promise.resolve(undefined)
    const requestedLanguage = typeof route.query.lang === 'string' ? route.query.lang : ''
    const course = requestedLanguage && version
      ? await getTranslatedCourse(courseID, version, requestedLanguage).then((value) => { translation.value = value; return value.course })
      : version ? await getPublishedCourseVersionByID(courseID, version) : await getLatestPublishedCourse(courseID)
    const languageResult = await discovered
    if (languageResult) languages.value = languageResult.languages
    else languages.value = []
    if (!active || generation !== requestVersion || !isCurrentRoute(courseID, version)) return false
    if (!courseMatchesRoute(course, courseID, version)) throw new Error('published course response does not match route')
    state.value = { kind: 'ready', course, selectedLesson: null }
    reconcileLessonSelection(course)
    progressSession.value?.dispose()
    progressSession.value = undefined
    if (auth.state.value.status === 'authenticated') {
      const session = createLearnerProgressSession({ courseId: course.courseId, version: course.version }, auth)
      progressSession.value = session
      void session.load()
    }
    return true
  } catch (error) {
    if (!active || generation !== requestVersion || !isCurrentRoute(courseID, version)) return false
    translationUnavailable.value = typeof route.query.lang === 'string' && error instanceof APIProblemError && error.status === 404
    state.value = error instanceof InvalidCourseRouteError || (error instanceof APIProblemError && error.status === 404)
      ? { kind: 'not-found' }
      : { kind: 'unavailable' }
    return true
  }
}

function reconcileLessonSelection(course: PublishedCourseVersionDetail) {
  if (state.value.kind !== 'ready') return
  const lessons = course.modules.flatMap((module) => module.lessons)
  const requestedLessonKey = publishedLessonKeyFromRoute(route.query.lesson)
  const selectedLesson = lessons.find((lesson) => lesson.stableKey === requestedLessonKey) ?? lessons[0] ?? null
  state.value = { kind: 'ready', course, selectedLesson }

  const normalizedLessonKey = selectedLesson?.stableKey
  if (route.query.lesson !== normalizedLessonKey) {
    void router.replace({ path: route.path, query: normalizedLessonKey ? { lesson: normalizedLessonKey } : {} })
  }
}

async function retryLoad() {
  const applied = await load()
  if (!active || !applied) return
  await nextTick()
  courseTitle.value?.focus()
}


async function claimCertificate() {
  if (claiming.value || state.value.kind !== 'ready') return
  claiming.value = true; certificateMessage.value = ''
  try { const certificate = await issueCertificate(state.value.course.courseId, state.value.course.version); await router.push(certificatePath(certificate.certificateId)) }
  catch (error) { certificateMessage.value = error instanceof APIProblemError && error.problem?.code === 'certificate_not_eligible' ? t('learner.reader.certificateIncomplete') : t('learner.reader.certificateError') }
  finally { claiming.value = false }
}

function routeParameter(name: 'courseId' | 'version'): string {
  return typeof route.params[name] === 'string' ? route.params[name] : ''
}

function isCurrentRoute(courseID: string, version: string): boolean {
  return routeParameter('courseId') === courseID && routeParameter('version') === version
}

function courseMatchesRoute(course: PublishedCourseVersionDetail, courseID: string, version: string): boolean {
  return course.courseId.toLowerCase() === courseID.toLowerCase()
    && isCourseVersion(course.version)
    && (!version || course.version === version)
}

function changeLanguage(language: string) { const query = { ...route.query }; if (!language || language === sourceLanguage.value) delete query.lang; else query.lang = language; void router.push({ path: route.path, query }) }
function returnToSource() { changeLanguage(sourceLanguage.value) }
function openLatestSource() { if (!lagNotice.value || state.value.kind !== 'ready') return; void router.push({ path: `/courses/by-id/${state.value.course.courseId}/versions/${lagNotice.value.latest}` }) }
function contributorRole(role: 'AUTHOR' | 'MAINTAINER'): string {
  return role === 'AUTHOR' ? t('learner.reader.author') : t('learner.reader.maintainer')
}
function selectLesson(key?: string) { if (key) void router.push({ path: route.path, query: { ...route.query, lesson: key } }) }
</script>
