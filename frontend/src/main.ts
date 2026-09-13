import { createApp } from 'vue'
import { createI18n } from 'vue-i18n'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import HomePage from './pages/HomePage.vue'
import './style.css'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messages: { en: { appName: 'Bridging the Gap LMS', home: 'Home' } },
})

const router = createRouter({
  history: createWebHistory(),
  routes: [{ path: '/', component: HomePage }],
})

createApp(App).use(router).use(i18n).mount('#app')
