<template>
  <section class="authoring-section" aria-labelledby="authoring-overview-title">
    <h2 id="authoring-overview-title">Overview</h2>
    <p class="authoring-section__intro">Draft metadata is available here. Editing controls will be added in a later Authoring slice.</p>
    <dl class="authoring-overview__metadata">
      <div><dt>Source language</dt><dd>{{ draft.source_language }}</dd></div>
      <div><dt>Revision</dt><dd>{{ draft.revision }}</dd></div>
      <div><dt>Last updated</dt><dd>{{ formatDate(draft.updated_at) }}</dd></div>
    </dl>
    <section v-if="draft.objectives.length" aria-labelledby="authoring-objectives-title">
      <h3 id="authoring-objectives-title">Learning objectives</h3>
      <ul><li v-for="objective in draft.objectives" :key="objective">{{ objective }}</li></ul>
    </section>
  </section>
</template>

<script setup lang="ts">
import { useAuthoringDraftContext } from '../authoring/draftContext'

const draft = useAuthoringDraftContext()

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString('en', { year: 'numeric', month: 'short', day: 'numeric' })
}
</script>
