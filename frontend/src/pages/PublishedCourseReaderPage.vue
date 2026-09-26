<template>
  <BtgPageContainer as="section" class="published-course-reader" width="application" aria-labelledby="published-course-title">
    <div v-if="state.kind === 'loading'" class="published-course-reader__state" role="status" aria-live="polite">
      <h1 id="published-course-title" ref="courseTitle" tabindex="-1">Loading course…</h1>
      <p>One moment while we load this published course.</p>
    </div>

    <div v-else-if="state.kind === 'not-found'" class="published-course-reader__state">
      <h1 id="published-course-title" ref="courseTitle" tabindex="-1">Course not found</h1>
      <p v-if="translationUnavailable">This translation is not available.</p><p v-else>This published course is not available.</p>
      <button v-if="translationUnavailable" type="button" @click="returnToSource">View the source version</button><RouterLink v-else to="/courses">Browse courses</RouterLink>
    </div>

    <div v-else-if="state.kind === 'unavailable'" class="published-course-reader__state" role="alert">
      <h1 id="published-course-title" ref="courseTitle" tabindex="-1">Course unavailable</h1>
      <p>We couldn’t load this course right now. Please try again.</p>
      <BtgButton variant="secondary" @click="retryLoad">Try again</BtgButton>
    </div>

    <template v-else>
      <header class="published-course-reader__header">
        <RouterLink class="published-course-reader__back" to="/courses">Browse courses</RouterLink>
        <p class="published-course-reader__eyebrow">Bridging the Gap Academy</p>
        <h1 id="published-course-title" ref="courseTitle" tabindex="-1">{{ state.course.title }}</h1>
        <p v-if="state.course.description" class="published-course-reader__description">{{ state.course.description }}</p>
        <label v-if="languages.length > 1" for="course-language">Course language</label>
        <select v-if="languages.length > 1" id="course-language" :value="selectedLanguage" @change="changeLanguage(($event.target as HTMLSelectElement).value)"><option v-for="language in languages" :key="language.language" :value="language.language">{{ language.language }}</option></select>
        <p v-if="translationUnavailable" role="alert">This translation is not available. <button type="button" @click="returnToSource">View the source version</button></p>
        <aside v-if="lagNotice" role="status"><p>You are reading the {{ selectedLanguage }} translation of version {{ lagNotice.translated }}. A newer source version, {{ lagNotice.latest }}, is available.</p><button type="button" @click="openLatestSource">View version {{ lagNotice.latest }} in {{ sourceLanguage }}</button></aside>
        <RouterLink :to="`/courses/by-id/${state.course.courseId}/community`">Course discussions</RouterLink>
        <button v-if="auth.state.value.status === 'authenticated'" type="button" @click="claimCertificate" :disabled="claiming">{{ claiming ? 'Requesting certificate…' : 'Get certificate' }}</button><p v-if="certificateMessage" role="status">{{ certificateMessage }}</p>
        <dl class="published-course-reader__metadata">
          <div><dt>Version</dt><dd>{{ state.course.version }}</dd></div>
          <div v-if="state.course.sourceLanguage"><dt>Source language</dt><dd>{{ state.course.sourceLanguage }}</dd></div>
          <div><dt>Published</dt><dd><time :datetime="state.course.publishedAt">{{ formatPublishedDate(state.course.publishedAt) }}</time></dd></div>
          <div v-if="state.course.license.displayName"><dt>License</dt><dd>{{ state.course.license.displayName }}</dd></div>
        </dl>
        <section v-if="state.course.objectives.length" aria-labelledby="published-course-objectives-title">
          <h2 id="published-course-objectives-title">What you’ll learn</h2>
          <ul><li v-for="objective in state.course.objectives" :key="objective">{{ objective }}</li></ul>
        </section>
        <section v-if="state.course.contributors.length" aria-labelledby="published-course-contributors-title">
          <h2 id="published-course-contributors-title">Contributors</h2>
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
        />
        <div class="published-course-reader__lesson-view">
          <PublishedCourseReaderLesson :lesson="state.selectedLesson" :assessments="state.course.assessments" :course-id="state.course.courseId" :version="state.course.version" />
          <p v-if="!state.selectedLesson" class="published-course-reader__empty" role="status">This course does not contain any lessons yet.</p>
        </div>
      </div>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import { useAuth } from '../auth/auth'
import { issueCertificate, certificatePath } from '../courses/certificates'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
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

type PublishedLesson = PublishedCourseVersionDetail['modules'][number]['lessons'][number]
type State =
  | { kind: 'loading' }
  | { kind: 'ready'; course: PublishedCourseVersionDetail; selectedLesson: PublishedLesson | null }
  | { kind: 'not-found' }
  | { kind: 'unavailable' }

const route = useRoute()
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

watch([() => route.params.courseId, () => route.params.version, () => route.query.lang], () => { void load() }, { immediate: true })
watch(() => route.query.lesson, () => {
  if (state.value.kind !== 'ready') return
  reconcileLessonSelection(state.value.course)
}, { flush: 'post' })

onBeforeUnmount(() => { active = false })

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
  catch (error) { certificateMessage.value = error instanceof APIProblemError && error.problem?.code === 'certificate_not_eligible' ? 'Complete this course before requesting a certificate.' : 'Your certificate could not be requested. Please try again.' }
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
function formatPublishedDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('en', { dateStyle: 'long' }).format(date)
}

function contributorRole(role: 'AUTHOR' | 'MAINTAINER'): string {
  return role === 'AUTHOR' ? 'Author' : 'Maintainer'
}
</script>
