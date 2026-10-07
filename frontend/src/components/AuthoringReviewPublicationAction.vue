<template>
  <BtgPanel
    as="section"
    class="authoring-review__decisions authoring-review__publication"
    aria-labelledby="authoring-review-publication-title"
    :aria-busy="state.kind === 'submitting'"
  >
    <h3 id="authoring-review-publication-title">{{ t('authoring.review.publication.publishTitle') }}</h3>
    <p>{{ t('authoring.review.publication.publishDescription') }}</p>

    <p v-if="state.kind === 'submitting'" class="authoring-review__status" role="status">{{ t('authoring.review.publication.publishing') }}</p>

    <BtgPanel
      v-else-if="state.kind === 'confirmation-failure'"
      class="authoring-review__publication-feedback"
      variant="subtle"
      padding="compact"
    >
      <h4 id="authoring-review-publication-feedback-title" tabindex="-1">{{ t('authoring.review.publication.confirmationTitle') }}</h4>
      <p>{{ t('authoring.review.publication.confirmationDescription') }}</p>
    </BtgPanel>

    <BtgPanel
      v-else-if="state.kind === 'validation-failure'"
      class="authoring-review__publication-feedback"
      variant="subtle"
      padding="compact"
    >
      <h4 id="authoring-review-publication-feedback-title" tabindex="-1">{{ t('authoring.review.publication.validationTitle') }}</h4>
      <p>{{ t('authoring.review.publication.validationCount', { count: state.issues.length }, state.issues.length) }}</p>
      <ul class="authoring-review__validation-list" :aria-label="t('authoring.review.publication.issues')">
        <li v-for="(issue, index) in state.issues" :key="`${issue.code}:${issue.path}:${index}`">
          <strong>{{ issueTitle(issue.code) }}</strong>
          <p>{{ issue.message }}</p>
          <p v-if="issueGuidance(issue.code)" class="authoring-review__validation-guidance">{{ issueGuidance(issue.code) }}</p>
          <p v-if="issue.path" class="authoring-review__validation-path">{{ t('authoring.review.publication.location', { path: issue.path }) }}</p>
        </li>
      </ul>
    </BtgPanel>

    <BtgPanel
      v-else-if="state.kind === 'conflict'"
      class="authoring-review__publication-feedback"
      variant="subtle"
      padding="compact"
    >
      <h4 id="authoring-review-publication-feedback-title" tabindex="-1">{{ conflictTitle(state.code) }}</h4>
      <p>{{ conflictMessage(state.code) }}</p>
    </BtgPanel>

    <BtgPanel
      v-else-if="state.kind === 'operational-failure'"
      class="authoring-review__publication-feedback"
      variant="subtle"
      padding="compact"
    >
      <h4 id="authoring-review-publication-feedback-title" tabindex="-1">{{ t('authoring.review.publication.failed') }}</h4>
      <p role="alert">{{ t('authoring.review.publication.failedDescription') }}</p>
    </BtgPanel>

    <BtgButton
      v-if="canPublish"
      :disabled="state.kind === 'submitting'"
      :aria-busy="state.kind === 'submitting'"
      @click="$emit('publish')"
    >{{ state.kind === 'submitting' ? t('authoring.review.publication.busy') : t('authoring.review.publication.action') }}</BtgButton>
  </BtgPanel>
</template>

<script setup lang="ts">
import type { PublicationValidationIssue } from '../authoring/authoring'
import type { AuthoringPublicationConflictCode, AuthoringPublicationState } from '../authoring/publication'
import { useI18n } from 'vue-i18n'
import BtgButton from './BtgButton.vue'
import BtgPanel from './BtgPanel.vue'

defineProps<{
  canPublish: boolean
  state: AuthoringPublicationState
}>()

defineEmits<{
  publish: []
}>()
const { t } = useI18n()

function issueTitle(code: PublicationValidationIssue['code']): string {
  if (code === 'unresolved_asset_reference') return t('authoring.review.publication.assetTitle')
  if (code === 'unresolved_assessment_reference') return t('authoring.review.publication.assessmentTitle')
  return t('authoring.review.publication.requirementTitle')
}

function issueGuidance(code: PublicationValidationIssue['code']): string | undefined {
  if (code === 'unresolved_asset_reference') return t('authoring.review.publication.assetGuidance')
  if (code === 'unresolved_assessment_reference') return t('authoring.review.publication.assessmentGuidance')
  return undefined
}

function conflictTitle(code: AuthoringPublicationConflictCode | undefined): string {
  if (code === 'course_version_already_exists') return t('authoring.review.publication.versionConflictTitle')
  if (code === 'publication_conflict') return t('authoring.review.publication.publicationConflictTitle')
  return t('authoring.review.publication.reviewRevisionConflictTitle')
}

function conflictMessage(code: AuthoringPublicationConflictCode | undefined): string {
  switch (code) {
    case 'review_revision_conflict':
      return t('authoring.review.publication.reviewRevisionConflict')
    case 'review_not_approved':
      return t('authoring.review.publication.reviewNotApproved')
    case 'course_version_already_exists':
      return t('authoring.review.publication.versionConflict')
    case 'publication_conflict':
      return t('authoring.review.publication.publicationConflict')
    default:
      return t('authoring.review.publication.conflictDefault')
  }
}
</script>
