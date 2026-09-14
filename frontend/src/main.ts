import { createApp, nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import { auth } from './auth/auth'
import HomePage from './pages/HomePage.vue'
import LoginPage from './pages/LoginPage.vue'
import AdminPage from './pages/AdminPage.vue'
import './style.css'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messages: { en: { appName: 'Bridging the Gap Academy', appContext: 'Structured learning', home: 'Home' } },
})

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: HomePage },
    { path: '/login', component: LoginPage },
    { path: '/admin', component: AdminPage },
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
