<template>
  <BtgPanel as="section" class="authoring-review__summary" aria-labelledby="authoring-review-state-title">
    <h3 id="authoring-review-state-title">{{ t('authoring.review.summary.title') }}</h3>

    <template v-if="activeReview">
      <p class="authoring-review__status">{{ statusLabel(activeReview.status) }}</p>
      <p>{{ t('authoring.review.summary.active') }}</p>
      <AuthoringReviewMetadata :review="activeReview" :labels="metadataLabels" />
    </template>

    <template v-else-if="latestReview">
      <p class="authoring-review__status">{{ statusLabel(latestReview.status) }}</p>
      <p v-if="latestReview.status === 'APPROVED'">{{ t('authoring.review.summary.approved') }}</p>
      <p v-else>{{ t('authoring.review.summary.terminal') }}</p>
      <AuthoringReviewMetadata :review="latestReview" :labels="metadataLabels" />
    </template>

    <template v-else>
      <p class="authoring-review__status">{{ t('authoring.review.status.editing') }}</p>
      <p>{{ t('authoring.review.summary.editing') }}</p>
    </template>
  </BtgPanel>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AuthoringReview } from '../authoring/authoring'
import { useI18n } from 'vue-i18n'
import { useApplicationFormatters } from '../i18n/formatters'
import BtgPanel from './BtgPanel.vue'
import AuthoringReviewMetadata, { type AuthoringReviewMetadataLabels } from './AuthoringReviewMetadata.vue'

defineProps<{
  activeReview?: AuthoringReview
  latestReview?: AuthoringReview
}>()
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
