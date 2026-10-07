<template>
  <BtgPanel as="section" class="authoring-review__history" aria-labelledby="authoring-review-history-title">
    <h3 id="authoring-review-history-title">{{ t('authoring.review.history.title') }}</h3>
    <p v-if="!reviews.length" class="authoring-review__empty">{{ t('authoring.review.history.empty') }}</p>
    <ol v-else class="authoring-review__history-list" :aria-label="t('authoring.review.history.list')">
      <li v-for="review in reviews" :key="review.id">
        <article>
          <h4>{{ t('authoring.review.history.cycle', { revision: review.draftRevision }) }}</h4>
          <p class="authoring-review__status">{{ statusLabel(review.status) }}</p>
          <AuthoringReviewMetadata :review="review" :labels="metadataLabels" />
          <RouterLink :to="authoringDraftReviewPath(draftId, review.id)">{{ t('authoring.review.history.view') }}</RouterLink>
        </article>
      </li>
    </ol>
  </BtgPanel>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { authoringDraftReviewPath, type AuthoringReview } from '../authoring/authoring'
import { useI18n } from 'vue-i18n'
import { useApplicationFormatters } from '../i18n/formatters'
import BtgPanel from './BtgPanel.vue'
import AuthoringReviewMetadata, { type AuthoringReviewMetadataLabels } from './AuthoringReviewMetadata.vue'

defineProps<{ draftId: string; reviews: AuthoringReview[] }>()
const { t } = useI18n()
const { date } = useApplicationFormatters()
const metadataLabels = computed<AuthoringReviewMetadataLabels>(() => ({
  draftRevision: t('authoring.review.metadata.draftRevision'),
  submitted: t('authoring.review.metadata.submitted'),
  decided: t('authoring.review.metadata.decided'),
  formatDate: (value) => date(value, 'dateTime'),
}))

function statusLabel(status: AuthoringReview['status']): string {
  switch (status) {
    case 'IN_REVIEW': return t('authoring.review.status.inReview')
    case 'APPROVED': return t('authoring.review.status.approved')
    case 'CHANGES_REQUESTED': return t('authoring.review.status.changesRequested')
  }
}

</script>
