<template>
  <dl class="authoring-review__metadata">
    <div><dt>{{ labels.draftRevision }}</dt><dd>{{ review.draftRevision }}</dd></div>
    <div><dt>{{ labels.submitted }}</dt><dd><time :datetime="review.submittedAt">{{ labels.formatDate(review.submittedAt) }}</time></dd></div>
    <template v-if="review.decidedAt">
      <div><dt>{{ labels.decided }}</dt><dd><time :datetime="review.decidedAt">{{ labels.formatDate(review.decidedAt) }}</time></dd></div>
    </template>
  </dl>
</template>

<script setup lang="ts">
import type { AuthoringReview } from '../authoring/authoring'

export type AuthoringReviewMetadataLabels = {
  draftRevision: string
  submitted: string
  decided: string
  formatDate: (value: string) => string
}

defineProps<{ review: AuthoringReview; labels: AuthoringReviewMetadataLabels }>()
</script>
