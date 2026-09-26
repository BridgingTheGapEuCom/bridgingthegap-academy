<template>
  <section class="authoring-review__history" aria-labelledby="authoring-review-history-title">
    <h3 id="authoring-review-history-title">Review history</h3>
    <p v-if="!reviews.length" class="authoring-review__empty">No Review cycles yet.</p>
    <ol v-else class="authoring-review__history-list" aria-label="Review history">
      <li v-for="review in reviews" :key="review.id">
        <article>
          <h4>Review cycle for Draft revision {{ review.draftRevision }}</h4>
          <p class="authoring-review__status">{{ statusLabel(review.status) }}</p>
          <AuthoringReviewMetadata :review="review" />
          <RouterLink :to="authoringDraftReviewPath(draftId, review.id)">View review</RouterLink>
        </article>
      </li>
    </ol>
  </section>
</template>

<script setup lang="ts">
import { authoringDraftReviewPath, type AuthoringReview } from '../authoring/authoring'
import AuthoringReviewMetadata from './AuthoringReviewMetadata.vue'

defineProps<{ draftId: string; reviews: AuthoringReview[] }>()

function statusLabel(status: AuthoringReview['status']): string {
  switch (status) {
    case 'IN_REVIEW': return 'In review'
    case 'APPROVED': return 'Approved'
    case 'CHANGES_REQUESTED': return 'Changes requested'
  }
}

</script>
