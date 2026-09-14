<template>
  <a class="skip-link" href="#main">Skip to content</a>
  <header class="site-header">
    <div class="btg-page-container site-header__inner">
      <RouterLink class="site-brand" to="/">
        <strong>{{ t('appName') }}</strong>
        <span class="site-brand__context">{{ t('appContext') }}</span>
      </RouterLink>
      <nav class="site-navigation" aria-label="Main navigation"><RouterLink to="/">{{ t('home') }}</RouterLink></nav>
    </div>
  </header>
  <main id="main" tabindex="-1"><RouterView /></main>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>
