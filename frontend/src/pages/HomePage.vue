<template>
  <div class="home-page">
    <BtgPageContainer as="section" class="home-page__hero" width="application" aria-labelledby="home-title">
      <div class="home-page__hero-layout">
        <div class="home-page__hero-content">
          <p class="home-page__eyebrow">Bridging the Gap Academy</p>
          <h1 id="home-title">Learn integration architecture with structure and clarity.</h1>
          <p class="home-page__intro">Practical, self-paced learning for integration architects and engineers.</p>
          <div class="home-page__actions">
            <RouterLink class="btg-button btg-button--primary" to="/courses">Browse courses</RouterLink>
            <RouterLink v-if="auth.state.value.status === 'unauthenticated'" class="btg-button btg-button--secondary" to="/login">Sign in</RouterLink>
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

    <BtgPageContainer as="section" class="home-page__section home-page__topics" width="application" aria-labelledby="learning-areas-title">
      <header class="home-page__section-heading">
        <p class="home-page__eyebrow">Core concepts</p>
        <h2 id="learning-areas-title">What you can learn</h2>
      </header>
      <ul class="home-page__topic-list">
        <li>
          <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M4 5h7v6H4zM13 13h7v6h-7zM7.5 11v2h9" /></svg>
          <h3>Integration Architecture</h3>
          <p>Boundaries, coupling, integration styles, architecture.</p>
        </li>
        <li>
          <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="6" cy="12" r="2" /><circle cx="18" cy="6" r="2" /><circle cx="18" cy="18" r="2" /><path d="m8 11 8-4M8 13l8 4" /></svg>
          <h3>Event-Driven Architecture</h3>
          <p>Events, brokers, delivery semantics, event design.</p>
        </li>
        <li>
          <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M4 6h16v4H4zM4 14h16v4H4zM8 10v4M16 10v4" /></svg>
          <h3>Messaging &amp; Integration Patterns</h3>
          <p>Queues, topics, routing, orchestration, transformation.</p>
        </li>
      </ul>
    </BtgPageContainer>

    <section class="home-page__course-band" aria-labelledby="featured-courses-title">
      <BtgPageContainer width="application">
        <header class="home-page__section-heading home-page__section-heading--split">
          <div>
            <p class="home-page__eyebrow">Start here</p>
            <h2 id="featured-courses-title">Featured courses</h2>
          </div>
          <RouterLink to="/courses">View all <span aria-hidden="true">→</span></RouterLink>
        </header>
        <p v-if="courses.kind === 'loading'" class="home-page__status" role="status">Loading published courses…</p>
        <div v-else-if="courses.kind === 'ready'" class="home-page__course-grid">
          <article v-for="course in courses.items" :key="course.courseId" class="home-page__course-card">
            <p class="home-page__course-meta">{{ course.sourceLanguage }} · Version {{ course.version }}</p>
            <h3><RouterLink :to="publishedCoursePath(course.courseId)">{{ course.title }}</RouterLink></h3>
            <p v-if="course.description">{{ course.description }}</p>
            <p class="home-page__course-published"><time :datetime="course.publishedAt">Published {{ publishedDate(course.publishedAt) }}</time></p>
          </article>
        </div>
        <div v-else-if="courses.kind === 'empty'" class="home-page__empty" role="status">
          <svg aria-hidden="true" focusable="false" viewBox="0 0 24 24"><path d="M5 4h14v16H5zM8 8h8M8 12h8M8 16h5" /></svg>
          <div>
            <p>Courses are being prepared.</p>
            <p>Browse the catalog for newly published learning material.</p>
            <RouterLink to="/courses">Browse courses <span aria-hidden="true">→</span></RouterLink>
          </div>
        </div>
        <div v-else class="home-page__course-error" role="alert">
          <p>Published courses are unavailable right now.</p>
          <BtgButton variant="secondary" @click="loadCourses">Try again</BtgButton>
        </div>
      </BtgPageContainer>
    </section>

    <BtgPageContainer as="section" class="home-page__section home-page__benefits" width="application" aria-labelledby="focused-learning-title">
      <header class="home-page__section-heading">
        <p class="home-page__eyebrow">A calm way to learn</p>
        <h2 id="focused-learning-title">Designed for focused learning</h2>
      </header>
      <ul class="home-page__benefit-list">
        <li><svg aria-hidden="true" focusable="false" viewBox="0 0 24 24"><path d="M5 4h14v16H5zM8 8h8M8 12h8M8 16h5" /></svg><h3>Structured</h3><p>Small, focused lessons</p></li>
        <li><svg aria-hidden="true" focusable="false" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8" /><path d="m8.5 12 2.25 2.25L15.5 9.5" /></svg><h3>Accessible</h3><p>Clear and inclusive by design</p></li>
        <li><svg aria-hidden="true" focusable="false" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8" /><path d="M4 12h16M12 4c2.3 2.2 3.5 4.9 3.5 8S14.3 17.8 12 20M12 4C9.7 6.2 8.5 8.9 8.5 12s1.2 5.8 3.5 8" /></svg><h3>Vendor-neutral</h3><p>Concepts before products</p></li>
        <li><svg aria-hidden="true" focusable="false" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8" /><path d="M12 7v5l3 2" /></svg><h3>Self-paced</h3><p>Learn on your own schedule</p></li>
      </ul>
    </BtgPageContainer>

    <BtgPageContainer as="section" class="home-page__final-cta" width="application" aria-labelledby="start-title">
      <div>
        <h2 id="start-title">Ready to start?</h2>
        <p>Browse the available courses and choose where to begin.</p>
      </div>
      <RouterLink class="btg-button btg-button--primary" to="/courses">Browse courses</RouterLink>
    </BtgPageContainer>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useAuth } from '../auth/auth'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import { listPublishedCourseCatalog, publishedCoursePath, type PublishedCourseCatalogItem } from '../courses/courses'

type CourseState =
  | { kind: 'loading' }
  | { kind: 'ready'; items: PublishedCourseCatalogItem[] }
  | { kind: 'empty' }
  | { kind: 'unavailable' }

const auth = useAuth()
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
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeZone: 'UTC' }).format(date)
}
</script>
