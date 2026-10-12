import type { LearnerKnowledgeCheckCopy } from '../components/LearnerKnowledgeCheck.vue'
import type { LessonBlockRendererCopy } from '../components/LessonBlockRenderer.vue'
import type { PublishedCourseReaderLessonCopy } from '../components/PublishedCourseReaderLesson.vue'

export const knowledgeCheckCopy: LearnerKnowledgeCheckCopy = {
  title: 'Knowledge check', practice: 'Practice responses stay on this page. Sign in to save and submit your answers.',
  submitted: 'This attempt has been submitted.', unsaved: 'Unsaved changes.', saved: 'Progress saved.',
  instruction: 'Choose answers, then save progress when you are ready.', chooseMatch: 'Choose a match', save: 'Save progress',
  saving: 'Saving progress…', submit: 'Submit answers', submitting: 'Submitting…', conflict: 'Progress changed elsewhere.',
  resultTitle: 'Submitted result', error: (code) => `Error: ${code}`, result: (correct, total, percentage) => `${correct} of ${total} questions correct (${percentage}%).`,
}

export const lessonBlockCopy: LessonBlockRendererCopy = {
  information: 'Information', note: 'Note', warning: 'Warning', tip: 'Tip', imageUnavailable: 'Image unavailable.',
  video: 'Video', audio: 'Audio', assetUnavailable: (type) => `${type} asset unavailable.`, transcript: 'Transcript',
  downloadTranscript: 'Download transcript', transcriptUnavailable: 'Transcript unavailable.', downloadCaptions: 'Download captions',
  code: 'Code', source: 'Source', scrollableTable: 'Scrollable lesson table', scrollableTableCaption: (caption) => `Scrollable table: ${caption}`,
  download: 'Download', downloadUnavailable: 'Download unavailable.', knowledgeTitle: 'Knowledge check',
  knowledgeUnavailable: 'Interactive knowledge checks will be available when assessments are enabled.', unsupported: 'This lesson block cannot be displayed safely.',
}

export const readerLessonCopy: PublishedCourseReaderLessonCopy = {
  lesson: 'Lesson', objectives: 'What you’ll learn', content: 'Lesson content', unsupported: 'This lesson uses a content format this version of the Academy cannot display.', invalid: 'Invalid content.',
  empty: 'This lesson has no published content yet.', complete: 'Lesson complete', markComplete: 'Mark lesson complete', completing: 'Saving completion…',
  estimatedDuration: (duration) => `Estimated duration: ${duration}`, progressError: (code) => `Progress error: ${code}`,
}
