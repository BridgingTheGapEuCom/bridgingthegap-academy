<template>
  <section class="authoring-section authoring-review-snapshot" aria-labelledby="authoring-review-snapshot-title">
    <p v-if="state.kind === 'loading'" class="authoring-review__state" role="status">Loading frozen Review snapshot…</p>

    <div v-else-if="state.kind === 'unavailable'" class="authoring-review__state">
      <h2 id="authoring-review-snapshot-title">Review unavailable</h2>
      <p>We couldn’t load this Review snapshot right now.</p>
      <BtgButton variant="secondary" @click="load">Try again</BtgButton>
    </div>

    <div v-else-if="state.kind === 'unsupported'" class="authoring-review__state" role="alert">
      <h2 id="authoring-review-snapshot-title">Review snapshot format unavailable</h2>
      <p>This historical Review snapshot uses a format this version of the Academy cannot display.</p>
      <RouterLink :to="authoringDraftPath(draft.id, 'review')">Back to Review overview</RouterLink>
    </div>

    <template v-else>
      <nav aria-label="Review snapshot navigation"><RouterLink :to="authoringDraftPath(draft.id, 'review')">Back to Review overview</RouterLink></nav>
      <header class="authoring-review-snapshot__header">
        <p class="authoring-review-snapshot__eyebrow">Frozen Review snapshot</p>
        <h2 id="authoring-review-snapshot-title">Reviewing Draft revision {{ state.detail.review.draftRevision }}</h2>
        <p>This is the immutable snapshot captured when this Review was submitted. Later Draft edits do not change it.</p>
        <p class="authoring-review__status">{{ statusLabel(state.detail.review.status) }}</p>
        <p v-if="state.detail.review.status === 'APPROVED'">Approved Review is not a published Course.</p>
        <AuthoringReviewMetadata :review="state.detail.review" />
      </header>

      <section class="authoring-review-snapshot__draft" aria-labelledby="review-snapshot-draft-title">
        <h3 id="review-snapshot-draft-title">Draft metadata</h3>
        <h4>{{ state.detail.snapshot.draft.title }}</h4>
        <p v-if="state.detail.snapshot.draft.description">{{ state.detail.snapshot.draft.description }}</p>
        <dl class="authoring-review__metadata">
          <div><dt>Intended version</dt><dd>{{ state.detail.snapshot.draft.intendedVersion }}</dd></div>
          <div><dt>Source language</dt><dd>{{ state.detail.snapshot.draft.sourceLanguage }}</dd></div>
          <div><dt>Content license</dt><dd>{{ state.detail.snapshot.draft.license.DisplayName }}</dd></div>
        </dl>
        <div v-if="state.detail.snapshot.draft.objectives.length" aria-labelledby="review-snapshot-draft-objectives-title">
          <h4 id="review-snapshot-draft-objectives-title">Objectives</h4>
          <ul><li v-for="objective in state.detail.snapshot.draft.objectives" :key="objective">{{ objective }}</li></ul>
        </div>
        <div v-if="state.detail.snapshot.draft.changelog" aria-labelledby="review-snapshot-changelog-title">
          <h4 id="review-snapshot-changelog-title">Changelog</h4>
          <p>{{ state.detail.snapshot.draft.changelog }}</p>
        </div>
      </section>

      <section class="authoring-review-snapshot__structure" aria-labelledby="review-snapshot-structure-title">
        <h3 id="review-snapshot-structure-title">Snapshot structure</h3>
        <p v-if="!state.detail.snapshot.modules.length">No Modules were captured in this snapshot.</p>
        <template v-else>
          <AuthoringReviewSnapshotModule
            v-for="module in state.detail.snapshot.modules"
            :key="module.id"
            :module="module"
            :lesson-titles="lessonTitles"
          />
        </template>
      </section>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { APIProblemError } from '../api/client'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { authoringDraftPath, getAuthoringDraftReview, type AuthoringReviewDetail } from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import AuthoringReviewMetadata from '../components/AuthoringReviewMetadata.vue'
import AuthoringReviewSnapshotModule from '../components/AuthoringReviewSnapshotModule.vue'
import BtgButton from '../components/BtgButton.vue'

type State =
  | { kind: 'loading' }
  | { kind: 'ready'; detail: AuthoringReviewDetail }
  | { kind: 'unsupported' }
  | { kind: 'unavailable' }

const route = useRoute()
const { draft, markDraftUnavailable } = useAuthoringDraftContext()
const state = ref<State>({ kind: 'loading' })
const reviewID = () => typeof route.params.reviewId === 'string' ? route.params.reviewId : ''
const captureScope = useAuthoringAsyncScope(() => `${draft.value.id}:${reviewID()}`)
const lessonTitles = computed<Readonly<Record<string, string>>>(() => {
  if (state.value.kind !== 'ready') return {}
  return Object.fromEntries(state.value.detail.snapshot.modules.flatMap((module) => module.lessons.map((lesson) => [lesson.stableKey, lesson.title])))
})
let active = true
let requestVersion = 0

watch(() => [draft.value.id, reviewID()], () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false })

async function load() {
  const isCurrent = captureScope()
  const generation = ++requestVersion
  const draftID = draft.value.id
  const exactReviewID = reviewID()
  state.value = { kind: 'loading' }
  try {
    const detail = await getAuthoringDraftReview(draftID, exactReviewID)
    if (!isCurrent() || !active || generation !== requestVersion) return
    // The client supports the same snapshot version persisted by Authoring;
    // never reinterpret an unknown historical format as the current Draft.
    state.value = detail.snapshot.schemaVersion === 1 ? { kind: 'ready', detail } : { kind: 'unsupported' }
  } catch (error) {
    if (!isCurrent() || !active || generation !== requestVersion) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    state.value = { kind: 'unavailable' }
  }
}

function statusLabel(status: AuthoringReviewDetail['review']['status']): string {
  switch (status) {
    case 'IN_REVIEW': return 'In review'
    case 'APPROVED': return 'Approved'
    case 'CHANGES_REQUESTED': return 'Changes requested'
  }
}
</script>
