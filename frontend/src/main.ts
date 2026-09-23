import { createApp, nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import { auth } from './auth/auth'
import { loginLocation, routeReturnPath } from './auth/navigation'
import HomePage from './pages/HomePage.vue'
import LoginPage from './pages/LoginPage.vue'
import AdminPage from './pages/AdminPage.vue'
import CourseListPage from './pages/CourseListPage.vue'
import CourseOverviewPage from './pages/CourseOverviewPage.vue'
import LessonPage from './pages/LessonPage.vue'
import PublishedCourseReaderPage from './pages/PublishedCourseReaderPage.vue'
import CourseCommunityPage from './pages/CourseCommunityPage.vue'
import CourseCommunityThreadPage from './pages/CourseCommunityThreadPage.vue'
import CourseCommunityModerationPage from './pages/CourseCommunityModerationPage.vue'
import CourseCommunityModeratorThreadPage from './pages/CourseCommunityModeratorThreadPage.vue'
import CertificatePage from './pages/CertificatePage.vue'
import PublicCertificatePage from './pages/PublicCertificatePage.vue'
import AuthoringHomePage from './pages/AuthoringHomePage.vue'
import AuthoringDraftCreatePage from './pages/AuthoringDraftCreatePage.vue'
import AuthoringDraftShell from './pages/AuthoringDraftShell.vue'
import AuthoringDraftOverviewPage from './pages/AuthoringDraftOverviewPage.vue'
import AuthoringDraftStructurePage from './pages/AuthoringDraftStructurePage.vue'
import AuthoringDraftMembersPage from './pages/AuthoringDraftMembersPage.vue'
import AuthoringDraftLessonPage from './pages/AuthoringDraftLessonPage.vue'
import AuthoringDraftReviewPage from './pages/AuthoringDraftReviewPage.vue'
import AuthoringDraftReviewSnapshotPage from './pages/AuthoringDraftReviewSnapshotPage.vue'
import AuthoringDraftAssessmentsPage from './pages/AuthoringDraftAssessmentsPage.vue'
import AuthoringDraftAssessmentPage from './pages/AuthoringDraftAssessmentPage.vue'
import './style.css'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messages: { en: { appName: 'Bridging the Gap Academy', appContext: 'Structured learning', home: 'Home', courses: 'Courses', authoring: 'Authoring' } },
})

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: HomePage },
    { path: '/login', component: LoginPage },
    { path: '/admin', component: AdminPage, meta: { requiresAuth: true } },
    { path: '/courses', component: CourseListPage },
    { path: '/courses/by-id/:courseId/versions/:version', name: 'published-course-version', component: PublishedCourseReaderPage },
    { path: '/courses/by-id/:courseId', name: 'published-course-latest', component: PublishedCourseReaderPage },
    { path: '/certificates/:certificateId', component: CertificatePage, meta: { requiresAuth: true } },
    { path: '/verify/certificates/:certificateId', component: PublicCertificatePage },
    { path: '/courses/by-id/:courseId/community', name: 'course-community', component: CourseCommunityPage, meta: { requiresAuth: true } },
    { path: '/courses/by-id/:courseId/community/threads/:threadId', name: 'course-community-thread', component: CourseCommunityThreadPage, meta: { requiresAuth: true } },
    { path: '/courses/by-id/:courseId/community/moderation', component: CourseCommunityModerationPage, meta: { requiresAuth: true } },
    { path: '/courses/by-id/:courseId/community/moderation/threads/:threadId', component: CourseCommunityModeratorThreadPage, meta: { requiresAuth: true } },
    { path: '/courses/:slug', component: CourseOverviewPage },
    { path: '/courses/:slug/versions/:version/lessons/:lessonKey', component: LessonPage },
    { path: '/authoring', name: 'authoring-home', component: AuthoringHomePage, meta: { requiresAuth: true } },
    { path: '/authoring/new', name: 'authoring-draft-create', component: AuthoringDraftCreatePage, meta: { requiresAuth: true } },
    {
      path: '/authoring/drafts/:draftId',
      component: AuthoringDraftShell,
      meta: { requiresAuth: true },
      children: [
        { path: '', redirect: (to) => ({ name: 'authoring-draft-overview', params: { draftId: to.params.draftId } }) },
        { path: 'overview', name: 'authoring-draft-overview', component: AuthoringDraftOverviewPage },
        { path: 'structure', name: 'authoring-draft-structure', component: AuthoringDraftStructurePage },
        { path: 'lessons/:lessonId', name: 'authoring-draft-lesson', component: AuthoringDraftLessonPage },
        { path: 'members', name: 'authoring-draft-members', component: AuthoringDraftMembersPage },
        { path: 'assessments', name: 'authoring-draft-assessments', component: AuthoringDraftAssessmentsPage },
        { path: 'assessments/:assessmentId', name: 'authoring-draft-assessment', component: AuthoringDraftAssessmentPage },
        { path: 'review', name: 'authoring-draft-review', component: AuthoringDraftReviewPage },
        { path: 'reviews/:reviewId', name: 'authoring-draft-review-snapshot', component: AuthoringDraftReviewSnapshotPage },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  if (!to.matched.some((record) => record.meta.requiresAuth)) return true
  if (auth.state.value.status === 'bootstrapping') await auth.bootstrapSession()
  if (auth.state.value.status === 'authenticated') return true
  if (auth.state.value.status === 'unauthenticated') return loginLocation(routeReturnPath(to))
  // An unavailable session check should not render a protected page as though
  // authenticated. Login retains its existing retryable session state.
  return loginLocation(routeReturnPath(to))
})

// Give client-side navigation the same clear reading start as a new document.
// Initial loading and in-page authorization checks leave focus undisturbed.
router.afterEach((to, from) => {
  if (from.matched.length === 0) return
  const isPublishedReaderLessonChange =
    (to.name === 'published-course-latest' || to.name === 'published-course-version') &&
    to.path === from.path &&
    to.query.lesson !== from.query.lesson
  if (isPublishedReaderLessonChange) {
    return
  }
  void nextTick(() => document.getElementById('main')?.focus())
})

createApp(App).use(router).use(i18n).mount('#app')

void auth.bootstrapSession()
