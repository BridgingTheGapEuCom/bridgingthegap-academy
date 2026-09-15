import { createApp, nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import { auth } from './auth/auth'
import HomePage from './pages/HomePage.vue'
import LoginPage from './pages/LoginPage.vue'
import AdminPage from './pages/AdminPage.vue'
import CourseListPage from './pages/CourseListPage.vue'
import CourseOverviewPage from './pages/CourseOverviewPage.vue'
import LessonPage from './pages/LessonPage.vue'
import AuthoringDraftShell from './pages/AuthoringDraftShell.vue'
import AuthoringDraftOverviewPage from './pages/AuthoringDraftOverviewPage.vue'
import AuthoringDraftStructurePage from './pages/AuthoringDraftStructurePage.vue'
import AuthoringDraftMembersPage from './pages/AuthoringDraftMembersPage.vue'
import AuthoringDraftLessonPage from './pages/AuthoringDraftLessonPage.vue'
import './style.css'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messages: { en: { appName: 'Bridging the Gap Academy', appContext: 'Structured learning', home: 'Home', courses: 'Courses' } },
})

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: HomePage },
    { path: '/login', component: LoginPage },
    { path: '/admin', component: AdminPage },
    { path: '/courses', component: CourseListPage },
    { path: '/courses/:slug', component: CourseOverviewPage },
    { path: '/courses/:slug/versions/:version/lessons/:lessonKey', component: LessonPage },
    {
      path: '/authoring/drafts/:draftId',
      component: AuthoringDraftShell,
      children: [
        { path: '', redirect: (to) => ({ name: 'authoring-draft-overview', params: { draftId: to.params.draftId } }) },
        { path: 'overview', name: 'authoring-draft-overview', component: AuthoringDraftOverviewPage },
        { path: 'structure', name: 'authoring-draft-structure', component: AuthoringDraftStructurePage },
        { path: 'lessons/:lessonId', name: 'authoring-draft-lesson', component: AuthoringDraftLessonPage },
        { path: 'members', name: 'authoring-draft-members', component: AuthoringDraftMembersPage },
      ],
    },
  ],
})

// Give client-side navigation the same clear reading start as a new document.
// Initial loading and in-page authorization checks leave focus undisturbed.
router.afterEach((_to, from) => {
  if (from.matched.length === 0) return
  void nextTick(() => document.getElementById('main')?.focus())
})

createApp(App).use(router).use(i18n).mount('#app')

void auth.bootstrapSession()
