<template>
  <dl class="authoring-review__metadata">
    <div><dt>Draft revision under review</dt><dd>{{ review.draftRevision }}</dd></div>
    <div><dt>Submitted by</dt><dd><code>{{ review.submittedBy }}</code></dd></div>
    <div><dt>Submitted</dt><dd><time :datetime="review.submittedAt">{{ formatTimestamp(review.submittedAt) }}</time></dd></div>
    <template v-if="review.decidedAt">
      <div><dt>Decided by</dt><dd><code>{{ review.decidedBy }}</code></dd></div>
      <div><dt>Decided</dt><dd><time :datetime="review.decidedAt">{{ formatTimestamp(review.decidedAt) }}</time></dd></div>
    </template>
  </dl>
</template>

<script setup lang="ts">
import type { AuthoringReview } from '../authoring/authoring'

defineProps<{ review: AuthoringReview }>()

function formatTimestamp(value: string): string {
  return new Intl.DateTimeFormat('en-GB', {
    dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC',
  }).format(new Date(value))
}
</script>
