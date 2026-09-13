<template>
  <a class="skip-link" href="#main">Skip to content</a>
  <header class="site-header"><strong>{{ t('appName') }}</strong><nav aria-label="Main navigation"><RouterLink to="/">{{ t('home') }}</RouterLink></nav></header>
  <main id="main" tabindex="-1"><RouterView /></main>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>
