<template>
  <div class="authoring-workspace-header authoring-workspace-header--draft">
    <RouterLink class="authoring-workspace-header__back" to="/authoring">Back to Authoring</RouterLink>
    <header>
      <p class="authoring-workspace-header__eyebrow">Authoring Draft</p>
      <h1 :id="titleId">{{ title }}</h1>
      <dl class="authoring-workspace-header__metadata">
        <div v-for="item in metadata" :key="item.label"><dt>{{ item.label }}</dt><dd>{{ item.value }}</dd></div>
      </dl>
    </header>
    <AuthoringTabs label="Draft sections" :items="tabs" />
  </div>
</template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import AuthoringTabs from './AuthoringTabs.vue'

withDefaults(defineProps<{ title: string; metadata: { label: string; value: string }[]; tabs: { label: string; to: RouteLocationRaw }[]; titleId?: string }>(), { titleId: 'authoring-draft-title' })
</script>
