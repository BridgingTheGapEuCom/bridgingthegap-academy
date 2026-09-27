<template>
  <div :class="{ 'app-shell--home': route.path === '/' }">
    <a class="skip-link" href="#main">Skip to content</a>
    <header class="site-header" :class="{ 'site-header--home': route.path === '/' }">
      <div class="btg-page-container site-header__inner">
        <RouterLink class="site-brand" to="/">
          <strong>{{ t('appName') }}</strong>
          <span class="site-brand__context">{{ t('appContext') }}</span>
        </RouterLink>
        <nav class="site-navigation" aria-label="Main navigation">
          <RouterLink to="/">{{ t('home') }}</RouterLink>
          <RouterLink v-if="auth.state.value.status === 'authenticated'" to="/dashboard">{{ t('dashboard') }}</RouterLink>
          <RouterLink to="/courses">{{ t('courses') }}</RouterLink>
          <RouterLink v-if="auth.state.value.status === 'authenticated'" to="/authoring">{{ t('authoring') }}</RouterLink>
        </nav>
        <AppSessionControls />
      </div>
    </header>
    <main id="main" tabindex="-1"><RouterView /></main>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAuth } from './auth/auth'
import AppSessionControls from './components/AppSessionControls.vue'
const { t } = useI18n()
const auth = useAuth()
const route = useRoute()
</script>
