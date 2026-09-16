<template>
  <section class="authoring-section authoring-review" aria-labelledby="authoring-review-title">
    <header class="authoring-review__header">
      <h2 id="authoring-review-title" tabindex="-1">Review</h2>
      <p class="authoring-section__intro">Review cycles evaluate a frozen Draft revision. This page shows cycle metadata only.</p>
    </header>

    <p v-if="state.kind === 'loading'" class="authoring-review__state" role="status">Loading Review information…</p>
    <div v-else-if="state.kind === 'unavailable'" class="authoring-review__state">
      <p>We couldn’t load Review information right now.</p>
      <BtgButton variant="secondary" @click="loadReviews">Try again</BtgButton>
    </div>

    <template v-else>
      <p v-if="submissionStatus" class="authoring-review__status" role="status">{{ submissionStatus }}</p>
      <p v-if="submissionError" class="authoring-review__error" role="alert">{{ submissionError }}</p>
      <AuthoringReviewSubmission
        v-if="!state.activeReview"
        :busy="submitting"
        :reloading="reloading"
        :conflict="submissionConflict"
        @submit="submitForReview"
        @reload="reloadReviewState"
      />
      <AuthoringReviewStatusSummary :active-review="state.activeReview" :latest-review="state.latestReview" />
      <AuthoringReviewHistory :draft-id="draft.id" :reviews="state.reviews" />
    </template>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { preserveFocusAfterRemoval } from '../authoring/focus'
import {
  getAuthoringDraft,
  getAuthoringDraftReviewHistory,
  submitAuthoringDraftReview,
  type AuthoringReview,
} from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import AuthoringReviewHistory from '../components/AuthoringReviewHistory.vue'
import AuthoringReviewSubmission from '../components/AuthoringReviewSubmission.vue'
import AuthoringReviewStatusSummary from '../components/AuthoringReviewStatusSummary.vue'
import BtgButton from '../components/BtgButton.vue'

type State =
  | { kind: 'loading' }
  | { kind: 'ready'; reviews: AuthoringReview[]; activeReview?: AuthoringReview; latestReview?: AuthoringReview }
  | { kind: 'unavailable' }

const { draft, replaceDraft, markDraftUnavailable } = useAuthoringDraftContext()
const state = ref<State>({ kind: 'loading' })
const captureScope = useAuthoringAsyncScope(() => draft.value.id)
const submitting = ref(false)
const reloading = ref(false)
const submissionConflict = ref(false)
const submissionStatus = ref<string>()
const submissionError = ref<string>()
let active = true
let requestVersion = 0

watch(() => draft.value.id, () => {
  submitting.value = false
  reloading.value = false
  submissionConflict.value = false
  clearSubmissionFeedback()
  void loadReviews()
}, { immediate: true })
onBeforeUnmount(() => { active = false })

function clearSubmissionFeedback() {
  submissionStatus.value = undefined
  submissionError.value = undefined
}

async function loadReviews(): Promise<boolean> {
  const isCurrent = captureScope()
  const generation = ++requestVersion
  const draftID = draft.value.id
  state.value = { kind: 'loading' }
  try {
    // The newest-first history response is one exact-Draft authorization and
    // state boundary. Deriving active and latest independently from this one
    // response avoids combining values observed before and after a concurrent
    // revocation, submission, or decision.
    const history = await getAuthoringDraftReviewHistory(draftID)
    if (!isCurrent() || !active || generation !== requestVersion) return false
    const latestReview = history.reviews[0]
    const activeReview = history.reviews.find((review) => review.status === 'IN_REVIEW')
    state.value = { kind: 'ready', reviews: history.reviews, activeReview, latestReview }
    return true
  } catch (error) {
    if (!isCurrent() || !active || generation !== requestVersion) return false
    if (error instanceof APIProblemError && error.status === 404) {
      markDraftUnavailable()
      return false
    }
    state.value = { kind: 'unavailable' }
    return false
  }
}

async function submitForReview() {
  if (submitting.value || reloading.value || submissionConflict.value || state.value.kind !== 'ready' || state.value.activeReview) return
  const isCurrent = captureScope()
  const draftID = draft.value.id
  const restoreFocus = preserveFocusAfterRemoval(() => document.getElementById('authoring-review-title'))
  submitting.value = true
  clearSubmissionFeedback()
  try {
    // The authoritative Draft context is the only source of the expected
    // revision. The browser never sends a Review snapshot or actor identity.
    await submitAuthoringDraftReview(draftID, { expectedDraftRevision: draft.value.revision })
    if (!isCurrent() || !active) return
    if (await loadReviews()) {
      submissionStatus.value = 'Draft submitted for review.'
      if (isCurrent()) await restoreFocus()
    }
  } catch (error) {
    if (!isCurrent() || !active) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    if (error instanceof APIProblemError && error.status === 409) {
      submissionConflict.value = true
      return
    }
    submissionError.value = error instanceof APIProblemError && error.status === 400
      ? 'We couldn’t submit this Draft for review. Check the Draft and try again.'
      : 'We couldn’t submit this Draft for review right now. Please try again.'
  } finally {
    if (isCurrent()) submitting.value = false
  }
}

async function reloadReviewState() {
  if (submitting.value || reloading.value) return
  const isCurrent = captureScope()
  const draftID = draft.value.id
  const restoreFocus = preserveFocusAfterRemoval(() => document.getElementById('authoring-review-title'))
  reloading.value = true
  clearSubmissionFeedback()
  try {
    const latestDraft = await getAuthoringDraft(draftID)
    if (!isCurrent() || !active) return
    replaceDraft(latestDraft)
    if (await loadReviews()) {
      submissionConflict.value = false
      submissionStatus.value = 'The latest Draft and Review state have been loaded.'
      if (isCurrent()) await restoreFocus()
    }
  } catch (error) {
    if (!isCurrent() || !active) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    submissionError.value = 'We couldn’t reload this Draft right now. Please try again.'
  } finally {
    if (isCurrent()) reloading.value = false
  }
}

</script>
