<template>
  <div class="authoring-workspace-header authoring-workspace-header--lesson">
    <RouterLink class="authoring-workspace-header__back" :to="backTo">Back to Structure</RouterLink>
    <header>
      <h1 :id="titleId" tabindex="-1">{{ title }}</h1>
      <div class="authoring-workspace-header__chips" aria-label="Lesson context"><span>Lesson</span><span>Draft</span></div>
    </header>
    <AuthoringTabs label="Lesson sections" :items="tabs" />
  </div>
</template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import AuthoringTabs from './AuthoringTabs.vue'

withDefaults(defineProps<{ title: string; backTo: RouteLocationRaw; tabs: { label: string; to: RouteLocationRaw }[]; titleId?: string }>(), { titleId: 'authoring-lesson-title' })
</script>
