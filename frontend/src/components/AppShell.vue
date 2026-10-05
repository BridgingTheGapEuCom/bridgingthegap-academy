<template>
  <div class="app-shell">
    <a class="skip-link" href="#main">{{ t('navigation.skipToContent') }}</a>
    <header class="site-header">
      <div class="btg-page-container site-header__inner">
        <RouterLink class="site-brand" to="/">
          <strong>{{ t('common.brand.name') }}</strong>
          <span class="site-brand__context">{{ t('common.brand.context') }}</span>
        </RouterLink>
        <nav class="site-navigation" :aria-label="t('navigation.mainLabel')">
          <RouterLink to="/">{{ t('navigation.home') }}</RouterLink>
          <RouterLink v-if="auth.state.value.status === 'authenticated'" to="/dashboard">{{ t('navigation.dashboard') }}</RouterLink>
          <RouterLink to="/courses">{{ t('navigation.courses') }}</RouterLink>
          <RouterLink v-if="auth.state.value.status === 'authenticated'" to="/authoring">{{ t('navigation.authoring') }}</RouterLink>
        </nav>
        <AppSessionControls />
      </div>
    </header>
    <main id="main" tabindex="-1"><RouterView /></main>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useAuth } from '../auth/auth'
import AppSessionControls from './AppSessionControls.vue'

const { t } = useI18n()
const auth = useAuth()
</script>
