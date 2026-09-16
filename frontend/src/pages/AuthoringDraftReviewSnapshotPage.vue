<template>
  <section class="authoring-section authoring-review-snapshot" aria-labelledby="authoring-review-snapshot-title">
    <div v-if="state.kind === 'loading'" class="authoring-review__state">
      <h2 id="authoring-review-snapshot-title" tabindex="-1">Review snapshot</h2>
      <p role="status">Loading frozen Review snapshot…</p>
    </div>

    <div v-else-if="state.kind === 'unavailable'" class="authoring-review__state">
      <h2 id="authoring-review-snapshot-title" tabindex="-1">Review unavailable</h2>
      <p role="alert">We couldn’t load this Review snapshot right now.</p>
      <BtgButton variant="secondary" @click="load">Try again</BtgButton>
    </div>

    <div v-else-if="state.kind === 'unsupported'" class="authoring-review__state" role="alert">
      <h2 id="authoring-review-snapshot-title" tabindex="-1">Review snapshot format unavailable</h2>
      <p>This historical Review snapshot uses a format this version of the Academy cannot display.</p>
      <RouterLink :to="authoringDraftPath(routeDraftID(), 'review')">Back to Review overview</RouterLink>
    </div>

    <template v-else>
      <nav aria-label="Review snapshot navigation"><RouterLink :to="authoringDraftPath(routeDraftID(), 'review')">Back to Review overview</RouterLink></nav>
      <p v-if="decisionStatus" class="authoring-review__status" role="status">{{ decisionStatus }}</p>
      <header class="authoring-review-snapshot__header">
        <p class="authoring-review-snapshot__eyebrow">Frozen Review snapshot</p>
        <h2 id="authoring-review-snapshot-title" tabindex="-1">Reviewing Draft revision {{ state.detail.review.draftRevision }}</h2>
        <p>This is the immutable snapshot captured when this Review was submitted. Later Draft edits do not change it.</p>
        <p class="authoring-review__status">{{ statusLabel(state.detail.review.status) }}</p>
        <p v-if="state.detail.review.status === 'APPROVED'">Approved Review is not a published Course.</p>
        <AuthoringReviewMetadata :review="state.detail.review" />
      </header>

      <AuthoringReviewDecisionActions
        v-if="state.detail.review.status === 'IN_REVIEW'"
        :busy="pendingDecision !== undefined"
        :pending-label="pendingDecision === 'request-changes' ? 'Requesting changes' : 'Approving'"
        :reloading="reloading"
        :policy-conflict="policyConflict"
        :conflict="decisionConflict"
        :error="decisionError"
        @approve="decide('approve')"
        @request-changes="decide('request-changes')"
        @reload="reloadReview"
      />

      <AuthoringReviewPublicationAction
        v-if="canPublishReview"
        :busy="publishing"
        :error="publicationFailure !== undefined"
        @publish="publishReview"
      />

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
import { preserveFocusAfterRemoval } from '../authoring/focus'
import {
  approveAuthoringReview,
  authoringDraftPath,
  getAuthoringDraftMembers,
  getAuthoringDraftReview,
  publishAuthoringDraftReview,
  requestAuthoringReviewChanges,
  type AuthoringReviewDetail,
} from '../authoring/authoring'
import { canDisplayAuthoringPublication, classifyAuthoringPublicationFailure, type AuthoringPublicationFailure } from '../authoring/publication'
import { useAuth } from '../auth/auth'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import AuthoringReviewDecisionActions from '../components/AuthoringReviewDecisionActions.vue'
import AuthoringReviewMetadata from '../components/AuthoringReviewMetadata.vue'
import AuthoringReviewPublicationAction from '../components/AuthoringReviewPublicationAction.vue'
import AuthoringReviewSnapshotModule from '../components/AuthoringReviewSnapshotModule.vue'
import BtgButton from '../components/BtgButton.vue'

type State =
  | { kind: 'loading' }
  | { kind: 'ready'; detail: AuthoringReviewDetail }
  | { kind: 'unsupported' }
  | { kind: 'unavailable' }

const route = useRoute()
const auth = useAuth()
const { markDraftUnavailable } = useAuthoringDraftContext()
const state = ref<State>({ kind: 'loading' })
const routeDraftID = () => typeof route.params.draftId === 'string' ? route.params.draftId : ''
const reviewID = () => typeof route.params.reviewId === 'string' ? route.params.reviewId : ''
const captureScope = useAuthoringAsyncScope(() => `${routeDraftID()}:${reviewID()}`)
const lessonTitles = computed<Readonly<Record<string, string>>>(() => {
  if (state.value.kind !== 'ready') return {}
  return Object.fromEntries(state.value.detail.snapshot.modules.flatMap((module) => module.lessons.map((lesson) => [lesson.stableKey, lesson.title])))
})
let active = true
let requestVersion = 0
let membershipRequestVersion = 0
const pendingDecision = ref<'approve' | 'request-changes'>()
const reloading = ref(false)
const policyConflict = ref(false)
const decisionConflict = ref(false)
const decisionError = ref<string>()
const decisionStatus = ref<string>()
const publishCapability = ref(false)
const publishing = ref(false)
const publicationFailure = ref<AuthoringPublicationFailure>()
const canPublishReview = computed(() => {
  if (state.value.kind !== 'ready') return false
  const review = state.value.detail.review
  return publishCapability.value
    && review.status === 'APPROVED'
    && review.draftId === routeDraftID()
    && review.id === reviewID()
    && Number.isSafeInteger(review.reviewRevision)
    && review.reviewRevision > 0
})

watch([routeDraftID, reviewID, () => auth.state.value], () => {
  pendingDecision.value = undefined
  reloading.value = false
  decisionStatus.value = undefined
  publishing.value = false
  publishCapability.value = false
  publicationFailure.value = undefined
  clearDecisionFeedback()
  void load()
}, { immediate: true })
onBeforeUnmount(() => { active = false })

async function load(): Promise<boolean> {
  const isCurrent = captureScope()
  const generation = ++requestVersion
  const draftID = routeDraftID()
  const exactReviewID = reviewID()
  state.value = { kind: 'loading' }
  try {
    const detail = await getAuthoringDraftReview(draftID, exactReviewID)
    if (!isCurrent() || !active || generation !== requestVersion) return false
    // The client supports the same snapshot version persisted by Authoring;
    // never reinterpret an unknown historical format as the current Draft.
    state.value = detail.snapshot.schemaVersion === 1 ? { kind: 'ready', detail } : { kind: 'unsupported' }
    if (state.value.kind === 'ready') void loadPublishCapability(detail)
    return state.value.kind === 'ready'
  } catch (error) {
    if (!isCurrent() || !active || generation !== requestVersion) return false
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return false }
    state.value = { kind: 'unavailable' }
    return false
  }
}

async function loadPublishCapability(detail: AuthoringReviewDetail): Promise<void> {
  const isCurrent = captureScope()
  const generation = ++membershipRequestVersion
  const actor = auth.state.value.status === 'authenticated' ? auth.state.value.userId : undefined
  publishCapability.value = false
  if (!actor || detail.review.status !== 'APPROVED' || detail.review.draftId !== routeDraftID() || detail.review.id !== reviewID()) return
  try {
    // This is a display capability based on the existing authoritative active
    // membership read. The publish endpoint remains the security boundary.
    const members = await getAuthoringDraftMembers(routeDraftID())
    if (!isCurrent() || !active || generation !== membershipRequestVersion) return
    publishCapability.value = canDisplayAuthoringPublication(members, actor)
  } catch (error) {
    if (!isCurrent() || !active || generation !== membershipRequestVersion) return
    if (error instanceof APIProblemError && error.status === 404) markDraftUnavailable()
    // An unavailable capability read must never make a publish action appear.
    publishCapability.value = false
  }
}

function clearDecisionFeedback() {
  policyConflict.value = false
  decisionConflict.value = false
  decisionError.value = undefined
}

async function decide(decision: 'approve' | 'request-changes') {
  if (pendingDecision.value || reloading.value || policyConflict.value || decisionConflict.value || state.value.kind !== 'ready' || state.value.detail.review.status !== 'IN_REVIEW') return
  const isCurrent = captureScope()
  const detail = state.value.detail
  const draftID = routeDraftID()
  const exactReviewID = reviewID()
  const restoreFocus = preserveFocusAfterRemoval(() => document.getElementById('authoring-review-snapshot-title'))
  pendingDecision.value = decision
  decisionStatus.value = undefined
  clearDecisionFeedback()
  try {
    const input = { expectedReviewRevision: detail.review.reviewRevision }
    if (decision === 'approve') await approveAuthoringReview(draftID, exactReviewID, input)
    else await requestAuthoringReviewChanges(draftID, exactReviewID, input)
    if (!isCurrent() || !active) return
    // Re-read only this immutable Review resource. This keeps the historical
    // snapshot separate from current Draft state and uses server status/revision.
    if (await load()) {
      decisionStatus.value = decision === 'approve'
        ? 'Review approved. The frozen snapshot is unchanged and has not been published.'
        : 'Changes requested. This Review cycle is finished and the frozen snapshot is unchanged.'
      if (isCurrent()) await restoreFocus()
    }
  } catch (error) {
    if (!isCurrent() || !active) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    if (error instanceof APIProblemError && error.status === 409) {
      if (error.problem?.code === 'independent_reviewer_required') policyConflict.value = true
      else decisionConflict.value = true
      return
    }
    decisionError.value = error instanceof APIProblemError && error.status === 400
      ? 'We couldn’t save this Review decision. Please check the Review state and try again.'
      : 'We couldn’t save this Review decision right now. Please try again.'
  } finally {
    if (isCurrent()) pendingDecision.value = undefined
  }
}

async function reloadReview() {
  if (pendingDecision.value || reloading.value) return
  const isCurrent = captureScope()
  const restoreFocus = preserveFocusAfterRemoval(() => document.getElementById('authoring-review-snapshot-title'))
  reloading.value = true
  decisionStatus.value = undefined
  clearDecisionFeedback()
  try {
    if (await load() && isCurrent()) await restoreFocus()
  } finally {
    if (isCurrent()) reloading.value = false
  }
}

async function publishReview() {
  if (publishing.value || !canPublishReview.value || state.value.kind !== 'ready') return
  const isCurrent = captureScope()
  const detail = state.value.detail
  const draftID = routeDraftID()
  const exactReviewID = reviewID()
  publishing.value = true
  publicationFailure.value = undefined
  try {
    // Only the exact authoritative Review revision is browser input. The API
    // derives publisher identity, publication time, provenance, and snapshot.
    await publishAuthoringDraftReview(draftID, exactReviewID, { expectedReviewRevision: detail.review.reviewRevision })
    if (!isCurrent() || !active) return
    // The Review itself remains APPROVED; publication is a separate immutable
    // fact. Re-read this exact resource rather than inventing PUBLISHED state.
    await load()
  } catch (error) {
    if (!isCurrent() || !active) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    const failure = classifyAuthoringPublicationFailure(error)
    publicationFailure.value = failure
    if (failure.kind === 'conflict' && (failure.code === 'review_revision_conflict' || failure.code === 'review_not_approved')) {
      // Refresh the authoritative exact Review, but never replay publication
      // with a revision the user did not explicitly submit.
      await load()
    }
  } finally {
    if (isCurrent()) publishing.value = false
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
