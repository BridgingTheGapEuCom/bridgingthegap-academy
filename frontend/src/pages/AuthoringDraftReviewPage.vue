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
      <AuthoringReviewStatusSummary :active-review="state.activeReview" :latest-review="state.latestReview" />
      <AuthoringReviewHistory :reviews="state.reviews" />
    </template>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import {
  getAuthoringActiveDraftReview,
  getAuthoringDraftReviewHistory,
  getAuthoringLatestDraftReview,
  type AuthoringReview,
} from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import AuthoringReviewHistory from '../components/AuthoringReviewHistory.vue'
import AuthoringReviewStatusSummary from '../components/AuthoringReviewStatusSummary.vue'
import BtgButton from '../components/BtgButton.vue'

type State =
  | { kind: 'loading' }
  | { kind: 'ready'; reviews: AuthoringReview[]; activeReview?: AuthoringReview; latestReview?: AuthoringReview }
  | { kind: 'unavailable' }

const { draft, markDraftUnavailable } = useAuthoringDraftContext()
const state = ref<State>({ kind: 'loading' })
const captureScope = useAuthoringAsyncScope(() => draft.value.id)
let active = true
let requestVersion = 0

watch(() => draft.value.id, () => { void loadReviews() }, { immediate: true })
onBeforeUnmount(() => { active = false })

async function loadReviews() {
  const isCurrent = captureScope()
  const generation = ++requestVersion
  const draftID = draft.value.id
  state.value = { kind: 'loading' }
  try {
    // History is the scope-confirming metadata read: it is 200 with an empty
    // list for an accessible Draft, unlike /active and /latest which use 404
    // to represent an absent cycle.
    const history = await getAuthoringDraftReviewHistory(draftID)
    if (!isCurrent() || !active || generation !== requestVersion) return

    const [activeReview, latestReview] = await Promise.all([
      optionalReview(() => getAuthoringActiveDraftReview(draftID)),
      optionalReview(() => getAuthoringLatestDraftReview(draftID)),
    ])
    if (!isCurrent() || !active || generation !== requestVersion) return
    // A non-empty history cannot legitimately lack a latest cycle. Do not
    // misrepresent an inconsistent or failed response as an empty workflow.
    if (history.reviews.length > 0 && !latestReview) {
      state.value = { kind: 'unavailable' }
      return
    }
    state.value = { kind: 'ready', reviews: history.reviews, activeReview, latestReview }
  } catch (error) {
    if (!isCurrent() || !active || generation !== requestVersion) return
    if (error instanceof APIProblemError && error.status === 404) {
      markDraftUnavailable()
      return
    }
    state.value = { kind: 'unavailable' }
  }
}

async function optionalReview(read: () => Promise<AuthoringReview>): Promise<AuthoringReview | undefined> {
  try {
    return await read()
  } catch (error) {
    if (error instanceof APIProblemError && error.status === 404) return undefined
    throw error
  }
}
</script>
