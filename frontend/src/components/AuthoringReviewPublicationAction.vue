<template>
  <section
    class="authoring-review__decisions authoring-review__publication"
    aria-labelledby="authoring-review-publication-title"
    :aria-busy="state.kind === 'submitting'"
  >
    <h3 id="authoring-review-publication-title">Publish this Review</h3>
    <p>Publishing creates an immutable CourseVersion from this exact frozen Review snapshot.</p>

    <p v-if="state.kind === 'submitting'" class="authoring-review__status" role="status">Publishing course version…</p>

    <div
      v-else-if="state.kind === 'confirmation-failure'"
      class="authoring-review__publication-feedback authoring-review__publication-feedback--error"
    >
      <h4 id="authoring-review-publication-feedback-title" tabindex="-1">Publication status could not be confirmed</h4>
      <p>The publication request completed, but the refreshed Review did not confirm Published. Review the current state before trying again.</p>
    </div>

    <div
      v-else-if="state.kind === 'validation-failure'"
      class="authoring-review__publication-feedback authoring-review__publication-feedback--error"
    >
      <h4 id="authoring-review-publication-feedback-title" tabindex="-1">Publication validation issues</h4>
      <p>Publication could not proceed because {{ issueCountLabel(state.issues.length) }} found.</p>
      <ul class="authoring-review__validation-list" aria-label="Publication validation issues">
        <li v-for="(issue, index) in state.issues" :key="`${issue.code}:${issue.path}:${index}`">
          <strong>{{ issueTitle(issue.code) }}</strong>
          <p>{{ issue.message }}</p>
          <p v-if="issueGuidance(issue.code)" class="authoring-review__validation-guidance">{{ issueGuidance(issue.code) }}</p>
          <p v-if="issue.path" class="authoring-review__validation-path">Location: <code>{{ issue.path }}</code></p>
        </li>
      </ul>
    </div>

    <div
      v-else-if="state.kind === 'conflict'"
      class="authoring-review__publication-feedback authoring-review__publication-feedback--conflict"
    >
      <h4 id="authoring-review-publication-feedback-title" tabindex="-1">{{ conflictTitle(state.code) }}</h4>
      <p>{{ conflictMessage(state.code) }}</p>
    </div>

    <div
      v-else-if="state.kind === 'operational-failure'"
      class="authoring-review__publication-feedback authoring-review__publication-feedback--error"
    >
      <h4 id="authoring-review-publication-feedback-title" tabindex="-1">Publication failed</h4>
      <p role="alert">We couldn’t publish this Review right now. Please try again.</p>
    </div>

    <BtgButton
      v-if="canPublish"
      :disabled="state.kind === 'submitting'"
      :aria-busy="state.kind === 'submitting'"
      @click="$emit('publish')"
    >{{ state.kind === 'submitting' ? 'Publishing…' : 'Publish course version' }}</BtgButton>
  </section>
</template>

<script setup lang="ts">
import type { PublicationValidationIssue } from '../authoring/authoring'
import type { AuthoringPublicationConflictCode, AuthoringPublicationState } from '../authoring/publication'
import BtgButton from './BtgButton.vue'

defineProps<{
  canPublish: boolean
  state: AuthoringPublicationState
}>()

defineEmits<{
  publish: []
}>()

function issueCountLabel(count: number): string {
  return `${count} blocking ${count === 1 ? 'issue was' : 'issues were'}`
}

function issueTitle(code: PublicationValidationIssue['code']): string {
  if (code === 'unresolved_asset_reference') return 'Unresolved asset reference'
  if (code === 'unresolved_assessment_reference') return 'Unresolved assessment reference'
  return 'Publication requirement'
}

function issueGuidance(code: PublicationValidationIssue['code']): string | undefined {
  if (code === 'unresolved_asset_reference') return 'The referenced asset must be available before this course can be published.'
  if (code === 'unresolved_assessment_reference') return 'The referenced assessment must be available before this course can be published.'
  return undefined
}

function conflictTitle(code: AuthoringPublicationConflictCode | undefined): string {
  if (code === 'course_version_already_exists') return 'Course version already exists'
  if (code === 'publication_conflict') return 'Publication conflict'
  return 'Review state changed'
}

function conflictMessage(code: AuthoringPublicationConflictCode | undefined): string {
  switch (code) {
    case 'review_revision_conflict':
      return 'This Review changed before publication completed. The latest Review state has been loaded; publication was not retried.'
    case 'review_not_approved':
      return 'This Review is no longer approved for publication. The current Review state has been loaded.'
    case 'course_version_already_exists':
      return 'This semantic version already exists for another publication. Choose a new version in a later Draft and Review cycle before publishing.'
    case 'publication_conflict':
      return 'Publication could not be completed because the stored publication state is inconsistent. Review the current state before trying again.'
    default:
      return 'Publication could not be completed because the Review state changed. Review the current state before trying again.'
  }
}
</script>
