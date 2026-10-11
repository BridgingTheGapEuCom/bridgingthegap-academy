<template>
  <div class="home-page">
    <BtgPageContainer as="section" class="home-page__hero" spacing="section" aria-labelledby="home-title">
      <div class="home-page__hero-layout">
        <div class="home-page__hero-content">
          <p class="home-page__eyebrow">{{ t('courses.home.eyebrow') }}</p>
          <h1 id="home-title">{{ t('courses.home.title') }}</h1>
          <p class="home-page__intro">{{ t('courses.home.description') }}</p>
          <div class="home-page__actions">
            <RouterLink class="btg-button btg-button--primary" to="/courses">{{ t('courses.home.browse') }}</RouterLink>
            <RouterLink v-if="auth.state.value.status === 'unauthenticated'" class="btg-button btg-button--secondary" to="/login">{{ t('courses.home.signIn') }}</RouterLink>
          </div>
        </div>
        <svg class="home-page__hero-motif" aria-hidden="true" focusable="false" viewBox="0 0 360 240">
          <path d="M16 174C78 30 140 226 204 101s88-35 140-102" />
          <path d="M28 207c75-34 92-128 156-128 66 0 71 86 149 72" />
          <path d="M74 34c37 52 79 56 119 25 41-32 63-18 108 29" />
          <circle cx="16" cy="174" r="4" />
          <circle cx="204" cy="101" r="4" />
          <circle cx="344" cy="72" r="4" />
          <circle cx="28" cy="207" r="4" />
          <circle cx="184" cy="79" r="4" />
          <circle cx="333" cy="151" r="4" />
        </svg>
      </div>
    </BtgPageContainer>

    <BtgPageContainer as="section" class="home-page__section home-page__topics" spacing="section" aria-labelledby="learning-areas-title">
      <header class="home-page__section-heading">
        <p class="home-page__eyebrow">{{ t('courses.home.conceptsEyebrow') }}</p>
        <h2 id="learning-areas-title">{{ t('courses.home.conceptsTitle') }}</h2>
      </header>
      <ul class="home-page__topic-list">
        <li>
          <BtgIcon name="layers" size="large" decorative />
          <h3>{{ t('courses.home.topics.integrationTitle') }}</h3>
          <p>{{ t('courses.home.topics.integrationDescription') }}</p>
        </li>
        <li>
          <BtgIcon name="move" size="large" decorative />
          <h3>{{ t('courses.home.topics.eventsTitle') }}</h3>
          <p>{{ t('courses.home.topics.eventsDescription') }}</p>
        </li>
        <li>
          <BtgIcon name="module" size="large" decorative />
          <h3>{{ t('courses.home.topics.messagingTitle') }}</h3>
          <p>{{ t('courses.home.topics.messagingDescription') }}</p>
        </li>
      </ul>
    </BtgPageContainer>

    <section class="home-page__course-band" aria-labelledby="featured-courses-title">
      <BtgPageContainer spacing="section">
        <header class="home-page__section-heading home-page__section-heading--split">
          <div>
            <p class="home-page__eyebrow">{{ t('courses.home.featuredEyebrow') }}</p>
            <h2 id="featured-courses-title">{{ t('courses.home.featuredTitle') }}</h2>
          </div>
          <RouterLink to="/courses">{{ t('courses.home.viewAll') }} <BtgIcon name="next" size="small" decorative /></RouterLink>
        </header>
        <BtgPanel v-if="courses.kind === 'loading'" variant="subtle" role="status">{{ t('courses.home.loading') }}</BtgPanel>
        <div v-else-if="courses.kind === 'ready'" class="home-page__course-grid">
          <BtgPanel v-for="course in courses.items" :key="course.courseId" as="article" class="home-page__course-card">
            <p class="home-page__course-meta">{{ t('courses.home.courseMetadata', { language: languageLabel(course.sourceLanguage), version: course.version }) }}</p>
            <h3><RouterLink :to="publishedCoursePath(course.courseId)">{{ course.title }}</RouterLink></h3>
            <p v-if="course.description">{{ course.description }}</p>
            <p class="home-page__course-published"><time :datetime="course.publishedAt">{{ t('courses.home.published', { date: publishedDate(course.publishedAt) }) }}</time></p>
          </BtgPanel>
        </div>
        <BtgPanel v-else-if="courses.kind === 'empty'" class="home-page__empty" variant="subtle" role="status">
          <BtgIcon name="document" size="large" decorative />
          <div>
            <p>{{ t('courses.home.emptyTitle') }}</p>
            <p>{{ t('courses.home.emptyDescription') }}</p>
            <RouterLink to="/courses">{{ t('courses.home.browse') }} <BtgIcon name="next" size="small" decorative /></RouterLink>
          </div>
        </BtgPanel>
        <BtgPanel v-else class="home-page__course-error" variant="subtle" role="alert">
          <p>{{ t('courses.home.unavailable') }}</p>
          <BtgButton variant="secondary" @click="loadCourses">{{ t('common.actions.tryAgain') }}</BtgButton>
        </BtgPanel>
      </BtgPageContainer>
    </section>

    <BtgPageContainer as="section" class="home-page__section home-page__benefits" spacing="section" aria-labelledby="focused-learning-title">
      <header class="home-page__section-heading">
        <p class="home-page__eyebrow">{{ t('courses.home.benefitsEyebrow') }}</p>
        <h2 id="focused-learning-title">{{ t('courses.home.benefitsTitle') }}</h2>
      </header>
      <ul class="home-page__benefit-list">
        <li><BtgIcon name="document" size="large" decorative /><h3>{{ t('courses.home.benefits.structuredTitle') }}</h3><p>{{ t('courses.home.benefits.structuredDescription') }}</p></li>
        <li><BtgIcon name="target" size="large" decorative /><h3>{{ t('courses.home.benefits.accessibleTitle') }}</h3><p>{{ t('courses.home.benefits.accessibleDescription') }}</p></li>
        <li><BtgIcon name="info" size="large" decorative /><h3>{{ t('courses.home.benefits.neutralTitle') }}</h3><p>{{ t('courses.home.benefits.neutralDescription') }}</p></li>
        <li><BtgIcon name="content" size="large" decorative /><h3>{{ t('courses.home.benefits.pacedTitle') }}</h3><p>{{ t('courses.home.benefits.pacedDescription') }}</p></li>
      </ul>
    </BtgPageContainer>

    <BtgPageContainer as="section" class="home-page__final-cta" spacing="section" aria-labelledby="start-title">
      <div>
        <h2 id="start-title">{{ t('courses.home.finalTitle') }}</h2>
        <p>{{ t('courses.home.finalDescription') }}</p>
      </div>
      <RouterLink class="btg-button btg-button--primary" to="/courses">{{ t('courses.home.browse') }}</RouterLink>
    </BtgPageContainer>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuth } from '../auth/auth'
import BtgButton from '../components/BtgButton.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPanel from '../components/BtgPanel.vue'
import { listPublishedCourseCatalog, publishedCoursePath, type PublishedCourseCatalogItem } from '../courses/courses'
import { languageLabel } from '../i18n/languages'
import { useApplicationFormatters } from '../i18n/formatters'

type CourseState =
  | { kind: 'loading' }
  | { kind: 'ready'; items: PublishedCourseCatalogItem[] }
  | { kind: 'empty' }
  | { kind: 'unavailable' }

const auth = useAuth()
const { t } = useI18n()
const { date } = useApplicationFormatters()
const courses = ref<CourseState>({ kind: 'loading' })
let active = true
let requestVersion = 0

void loadCourses()
onBeforeUnmount(() => { active = false; requestVersion += 1 })

async function loadCourses() {
  const request = ++requestVersion
  courses.value = { kind: 'loading' }
  try {
    const page = await listPublishedCourseCatalog({ limit: 2, offset: 0 })
    if (!active || request !== requestVersion) return
    courses.value = page.items.length ? { kind: 'ready', items: page.items.slice(0, 2) } : { kind: 'empty' }
  } catch {
    if (active && request === requestVersion) courses.value = { kind: 'unavailable' }
  }
}

function publishedDate(value: string): string {
  return date(value, 'date')
}
</script>
