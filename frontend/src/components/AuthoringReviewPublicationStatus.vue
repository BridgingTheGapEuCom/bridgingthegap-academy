<template>
  <BtgPanel as="section" class="authoring-review__decisions authoring-review__publication" aria-labelledby="authoring-review-publication-status-title">
    <h3 id="authoring-review-publication-status-title">{{ t('authoring.review.publication.title') }}</h3>

    <template v-if="publication.published">
      <BtgPanel class="authoring-review__publication-feedback" variant="subtle" padding="compact">
        <h4 id="authoring-review-published-title" tabindex="-1">{{ t('authoring.review.publication.publishedTitle') }}</h4>
        <p>{{ t('authoring.review.publication.published', { version: publication.published.courseVersion }) }}</p>
        <dl class="authoring-review__metadata">
          <div><dt>{{ t('authoring.review.metadata.version') }}</dt><dd>{{ publication.published.courseVersion }}</dd></div>
          <div><dt>{{ t('authoring.review.metadata.published') }}</dt><dd><time :datetime="publication.published.publishedAt">{{ date(publication.published.publishedAt, 'dateTime') }}</time></dd></div>
        </dl>
        <RouterLink :to="publishedCourseVersionPath(publication.published.courseId, publication.published.courseVersion)">
          {{ t('authoring.review.publication.view', { version: publication.published.courseVersion }) }}
        </RouterLink>
        <CoursePackageExportButton :course-id="publication.published.courseId" :version="publication.published.courseVersion" />
      </BtgPanel>
    </template>

    <template v-else-if="review.status !== 'APPROVED'">
      <p>{{ t('authoring.review.publication.unavailable', { status: statusLabel(review.status) }) }}</p>
    </template>

    <template v-else-if="!publication.publishable">
      <BtgPanel class="authoring-review__publication-feedback" variant="subtle" padding="compact">
        <h4 id="authoring-review-publication-blocked-title" tabindex="-1">{{ t('authoring.review.publication.blockedTitle') }}</h4>
        <p>{{ t('authoring.review.publication.blockedDescription') }}</p>
        <ul class="authoring-review__validation-list" :aria-label="t('authoring.review.publication.issues')">
          <li v-for="(issue, index) in publication.issues" :key="`${issue.code}:${issue.path}:${index}`">
            <p>{{ issueTitle(issue.code) }}</p>
            <p>{{ issue.message }}</p>
            <p v-if="issueGuidance(issue.code)" class="authoring-review__validation-guidance">{{ issueGuidance(issue.code) }}</p>
            <p v-if="issue.path" class="authoring-review__validation-path">{{ t('authoring.review.publication.location', { path: issue.path }) }}</p>
          </li>
        </ul>
      </BtgPanel>
    </template>

    <template v-else-if="publication.canPublish">
      <BtgPanel class="authoring-review__publication-feedback" variant="subtle" padding="compact">
        <h4>{{ t('authoring.review.publication.readyTitle') }}</h4>
        <p>{{ t('authoring.review.publication.ready', { version: intendedVersion }) }}</p>
      </BtgPanel>
    </template>

    <template v-else>
      <p>{{ t('authoring.review.publication.readyNoCapability') }}</p>
    </template>
  </BtgPanel>
</template>

<script setup lang="ts">
import type { AuthoringReview, AuthoringReviewPublicationStatus } from '../authoring/authoring'
import { publishedCourseVersionPath } from '../courses/courses'
import { useI18n } from 'vue-i18n'
import { useApplicationFormatters } from '../i18n/formatters'
import BtgPanel from './BtgPanel.vue'
import CoursePackageExportButton from './CoursePackageExportButton.vue'

defineProps<{
  review: AuthoringReview
  publication: AuthoringReviewPublicationStatus
  intendedVersion: string
}>()
const { t } = useI18n()
const { date } = useApplicationFormatters()

function statusLabel(status: AuthoringReview['status']): string {
  switch (status) {
    case 'IN_REVIEW': return t('authoring.review.publication.statusInReview')
    case 'CHANGES_REQUESTED': return t('authoring.review.publication.statusChangesRequested')
    case 'APPROVED': return t('authoring.review.publication.statusApproved')
  }
}

function issueTitle(code: string): string {
  if (code === 'unresolved_asset_reference') return t('authoring.review.publication.assetTitle')
  if (code === 'unresolved_assessment_reference') return t('authoring.review.publication.assessmentTitle')
  return t('authoring.review.publication.requirementTitle')
}

function issueGuidance(code: string): string | undefined {
  if (code === 'unresolved_asset_reference') return t('authoring.review.publication.assetGuidance')
  if (code === 'unresolved_assessment_reference') return t('authoring.review.publication.assessmentGuidance')
  return undefined
}
</script>
