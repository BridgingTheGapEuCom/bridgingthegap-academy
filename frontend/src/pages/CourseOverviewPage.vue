<template>
  <BtgPageContainer as="article" class="course-overview" aria-labelledby="course-title">
    <BtgPanel v-if="state.kind === 'loading'" class="course-overview__state" variant="subtle" role="status">
      <BtgPageTitle id="course-title" :title="t('courses.overview.loadingTitle')" :description="t('courses.overview.loadingDescription')" />
    </BtgPanel>

    <BtgPanel v-else-if="state.kind === 'not-found'" class="course-overview__state" variant="subtle">
      <BtgPageTitle id="course-title" :title="t('courses.overview.notFoundTitle')" :description="t('courses.overview.notFoundDescription')" />
      <RouterLink to="/courses">{{ t('courses.overview.browse') }}</RouterLink>
    </BtgPanel>

    <BtgPanel v-else-if="state.kind === 'unavailable'" class="course-overview__state" variant="subtle" role="alert">
      <BtgPageTitle id="course-title" :title="t('courses.overview.unavailableTitle')" :description="t('courses.overview.unavailableDescription')" />
      <BtgButton variant="secondary" @click="load">{{ t('common.actions.tryAgain') }}</BtgButton>
    </BtgPanel>

    <div v-else class="course-overview__content">
      <header class="course-overview__header">
        <p class="course-overview__eyebrow">{{ t('courses.overview.eyebrow') }}</p>
        <BtgPageTitle id="course-title" :title="state.course.version.title" :description="state.course.version.description" />
        <BtgMetadataStrip :items="courseMetadata" />
        <BtgBadge v-if="state.course.version.status && state.course.version.status !== 'PUBLISHED'" variant="warning">{{ statusLabel(state.course.version.status) }}</BtgBadge>
      </header>

      <section v-if="state.course.version.objectives?.length" aria-labelledby="objectives-title">
        <h2 id="objectives-title">{{ t('courses.overview.objectives') }}</h2>
        <ul><li v-for="objective in state.course.version.objectives" :key="objective">{{ objective }}</li></ul>
      </section>

      <section aria-labelledby="outline-title">
        <h2 id="outline-title">{{ t('courses.overview.outline') }}</h2>
        <div class="course-outline">
          <BtgPanel v-for="structure in state.course.modules" :key="structure.module.key" as="section" class="course-outline__module" :aria-labelledby="`module-${structure.module.key}`">
            <header class="course-outline__module-header"><BtgIcon name="module" decorative /><h3 :id="`module-${structure.module.key}`">{{ structure.module.title }}</h3><BtgBadge>{{ lessonCount(structure.lessons.length) }}</BtgBadge></header>
            <p v-if="structure.module.description" class="course-outline__module-description">{{ structure.module.description }}</p>
            <ol class="course-outline__lessons">
              <li v-for="lesson in structure.lessons" :key="lesson.key" class="course-outline__lesson">
                <article>
                  <h4><RouterLink :to="lessonPath(state.course.course.slug, versionText, lesson.key)">{{ lesson.title }}</RouterLink></h4>
                  <p v-if="lesson.description">{{ lesson.description }}</p>
                  <p v-if="durationText(lesson.estimated_duration_minutes)" class="course-outline__meta">{{ t('courses.overview.estimatedDuration', { duration: durationText(lesson.estimated_duration_minutes) }) }}</p>
                  <p v-if="lesson.recommended_prerequisite_keys.length" class="course-outline__prerequisites">
                    {{ t('courses.overview.prerequisites', { lessons: prerequisiteNames(lesson.recommended_prerequisite_keys).join(', ') }) }}
                  </p>
                </article>
              </li>
            </ol>
            <p v-if="!structure.lessons.length" class="course-outline__empty">{{ t('courses.overview.noLessons') }}</p>
          </BtgPanel>
        </div>
      </section>

      <footer class="course-overview__footer">
        <section v-if="state.course.version.contributors?.length" aria-labelledby="contributors-title">
          <h2 id="contributors-title">{{ t('courses.overview.contributors') }}</h2>
          <ul class="course-overview__contributors"><li v-for="contributor in state.course.version.contributors" :key="`${contributor.order}-${contributor.display_name}`">{{ contributor.display_name }} <span>({{ contributorRole(contributor.role) }})</span></li></ul>
        </section>
        <section v-if="state.course.version.license" aria-labelledby="license-title">
          <h2 id="license-title">{{ t('courses.overview.license') }}</h2>
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
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { APIProblemError } from '../api/client'
import BtgButton from '../components/BtgButton.vue'
import BtgBadge from '../components/BtgBadge.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgMetadataStrip from '../components/BtgMetadataStrip.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import { languageLabel } from '../i18n/languages'
import { getCourse, InvalidCourseSlugError, lessonPath, type CourseDetail } from '../courses/courses'
import { useApplicationFormatters } from '../i18n/formatters'

type State = { kind: 'loading' } | { kind: 'ready'; course: CourseDetail } | { kind: 'not-found' } | { kind: 'unavailable' }

const route = useRoute()
const { t } = useI18n()
const { duration } = useApplicationFormatters()
const state = ref<State>({ kind: 'loading' })
const versionText = computed(() => state.value.kind === 'ready' ? state.value.course.version.version ?? '' : '')
const courseMetadata = computed(() => {
  if (state.value.kind !== 'ready') return []
  const course = state.value.course
  const lessons = course.modules.reduce((total, module) => total + module.lessons.length, 0)
  return [
    { label: t('courses.overview.version'), value: course.version.version },
    ...(course.version.source_language ? [{ label: t('courses.overview.sourceLanguage'), value: languageLabel(course.version.source_language) }] : []),
    { label: t('courses.overview.modulesLabel'), value: t('courses.overview.modules', { count: course.modules.length }, course.modules.length) },
    { label: t('courses.overview.lessonsLabel'), value: lessonCount(lessons) },
  ]
})
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
  return role === 'AUTHOR' ? t('courses.overview.author') : t('courses.overview.maintainer')
}

function statusLabel(status: string | undefined): string {
  return status === 'DEPRECATED' ? t('courses.overview.deprecated') : t('courses.overview.archived')
}

function lessonCount(count: number): string { return t('courses.overview.lessons', { count }, count) }
function durationText(minutes: number | null | undefined): string { return duration(minutes ?? 0) }

function safeLicenseURL(value: string | undefined): string | undefined {
  if (!value) return undefined
  try {
    return new URL(value).protocol === 'https:' ? value : undefined
  } catch {
    return undefined
  }
}
</script>
