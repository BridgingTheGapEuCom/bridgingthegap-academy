<template>
  <section class="authoring-review__decisions authoring-review__publication" aria-labelledby="authoring-review-publication-status-title">
    <h3 id="authoring-review-publication-status-title">Publication</h3>

    <template v-if="publication.published">
      <div class="authoring-review__publication-feedback authoring-review__publication-feedback--success">
        <h4>Published course version</h4>
        <p role="status">Published: this exact Review produced course version {{ publication.published.courseVersion }}.</p>
        <dl class="authoring-review__metadata">
          <div><dt>Version</dt><dd>{{ publication.published.courseVersion }}</dd></div>
          <div><dt>Published</dt><dd><time :datetime="publication.published.publishedAt">{{ formatTimestamp(publication.published.publishedAt) }}</time></dd></div>
        </dl>
        <RouterLink :to="publishedCourseVersionPath(publication.published.courseId, publication.published.courseVersion)">
          View published course version {{ publication.published.courseVersion }}
        </RouterLink>
      </div>
    </template>

    <template v-else-if="review.status !== 'APPROVED'">
      <p>Publication is unavailable because this Review is not approved. Current status: {{ statusLabel(review.status) }}.</p>
    </template>

    <template v-else-if="!publication.publishable">
      <div class="authoring-review__publication-feedback authoring-review__publication-feedback--error">
        <h4>Publication is blocked</h4>
        <p>This approved Review has blocking publication requirements.</p>
        <ul class="authoring-review__validation-list" aria-label="Publication validation issues">
          <li v-for="(issue, index) in publication.issues" :key="`${issue.code}:${issue.path}:${index}`">
            <p>{{ issue.message }}</p>
            <p v-if="issue.path" class="authoring-review__validation-path">Location: <code>{{ issue.path }}</code></p>
          </li>
        </ul>
      </div>
    </template>

    <template v-else-if="publication.canPublish">
      <div class="authoring-review__publication-feedback authoring-review__publication-feedback--ready">
        <h4>Ready for publication</h4>
        <p>This approved Review can be published as course version {{ intendedVersion }}.</p>
      </div>
    </template>

    <template v-else>
      <p>This Review is ready for publication.</p>
    </template>
  </section>
</template>

<script setup lang="ts">
import type { AuthoringReview, AuthoringReviewPublicationStatus } from '../authoring/authoring'
import { publishedCourseVersionPath } from '../courses/courses'

defineProps<{
  review: AuthoringReview
  publication: AuthoringReviewPublicationStatus
  intendedVersion: string
}>()

function formatTimestamp(value: string): string {
  return new Intl.DateTimeFormat('en-GB', {
    dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC',
  }).format(new Date(value))
}

function statusLabel(status: AuthoringReview['status']): string {
  switch (status) {
    case 'IN_REVIEW': return 'in review'
    case 'CHANGES_REQUESTED': return 'changes requested'
    case 'APPROVED': return 'approved'
  }
}
</script>
