<template>
  <BtgPageContainer as="section" class="settings-shell" :aria-labelledby="titleId">
    <div class="settings-shell__layout">
      <aside class="settings-shell__navigation" :aria-label="navigationLabel">
        <h2>{{ title }}</h2>
        <nav :aria-label="navigationLabel">
          <RouterLink v-for="item in items" :key="item.to" :to="item.to" exact-active-class="settings-shell__link--active">{{ item.label }}</RouterLink>
        </nav>
      </aside>
      <div class="settings-shell__content"><slot /></div>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import BtgPageContainer from './BtgPageContainer.vue'

defineProps<{
  title: string
  titleId: string
  navigationLabel: string
  items: readonly { to: string; label: string }[]
}>()
</script>

<style scoped>
.settings-shell { padding-block: var(--academy-page-padding-block); }
.settings-shell__layout { display: grid; grid-template-columns: minmax(11rem, 15rem) minmax(0, 1fr); align-items: start; gap: clamp(var(--academy-space-6), 6vw, var(--academy-space-8)); }
.settings-shell__navigation { display: grid; gap: var(--academy-space-3); color: var(--academy-text-primary); }
.settings-shell__navigation h2 { margin: 0; font-size: var(--academy-font-size-subsection-title); }
.settings-shell__navigation nav { display: grid; gap: var(--academy-space-1); }
.settings-shell__navigation a { display: block; border-inline-start: 0.2rem solid transparent; color: var(--academy-text-secondary); padding: var(--academy-space-2) var(--academy-space-3); text-decoration: none; }
.settings-shell__navigation .settings-shell__link--active { border-color: var(--academy-action-primary-bg); background: var(--academy-surface-selected); color: var(--academy-text-primary); font-weight: var(--academy-font-weight-strong); }
.settings-shell__content { display: grid; min-width: 0; gap: var(--academy-space-6); }
@media (max-width: 48rem) { .settings-shell__layout { grid-template-columns: 1fr; gap: var(--academy-space-5); } .settings-shell__navigation nav { grid-template-columns: minmax(0, 1fr); } }
</style>
