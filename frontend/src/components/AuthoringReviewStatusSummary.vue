<template>
  <section class="authoring-review__summary" aria-labelledby="authoring-review-state-title">
    <h3 id="authoring-review-state-title">Current Review state</h3>

    <template v-if="activeReview">
      <p class="authoring-review__status">{{ statusLabel(activeReview.status) }}</p>
      <p>This Review is currently being considered.</p>
      <AuthoringReviewMetadata :review="activeReview" />
    </template>

    <template v-else-if="latestReview">
      <p class="authoring-review__status">{{ statusLabel(latestReview.status) }}</p>
      <p v-if="latestReview.status === 'APPROVED'">This Review is approved, but the Draft has not been published.</p>
      <p v-else>A later Draft revision can begin a new Review cycle.</p>
      <AuthoringReviewMetadata :review="latestReview" />
    </template>

    <template v-else>
      <p class="authoring-review__status">Editing</p>
      <p>No Review cycle has been submitted yet. Editing is the current derived workspace state, not a persisted Review status.</p>
    </template>
  </section>
</template>

<script setup lang="ts">
import type { AuthoringReview } from '../authoring/authoring'
import AuthoringReviewMetadata from './AuthoringReviewMetadata.vue'

defineProps<{
  activeReview?: AuthoringReview
  latestReview?: AuthoringReview
}>()

function statusLabel(status: AuthoringReview['status']): string {
  switch (status) {
    case 'IN_REVIEW': return 'In review'
    case 'APPROVED': return 'Approved'
    case 'CHANGES_REQUESTED': return 'Changes requested'
  }
}

</script>
