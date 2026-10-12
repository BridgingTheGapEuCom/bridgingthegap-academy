import { useI18n } from 'vue-i18n'
import type { LessonBlockRendererCopy } from '../components/LessonBlockRenderer.vue'
import type { LearnerKnowledgeCheckCopy } from '../components/LearnerKnowledgeCheck.vue'
import type { LearnerAttemptError } from '../courses/attempts'
import { useApplicationFormatters } from '../i18n/formatters'

export function useLearnerContentCopy() {
  const { t } = useI18n()
  const { number } = useApplicationFormatters()
  const blockCopy: LessonBlockRendererCopy = {
    information: t('learner.content.information'), note: t('learner.content.note'), warning: t('learner.content.warning'), tip: t('learner.content.tip'),
    imageUnavailable: t('learner.content.imageUnavailable'), video: t('learner.content.video'), audio: t('learner.content.audio'),
    assetUnavailable: (type) => t('learner.content.assetUnavailable', { type }), transcript: t('learner.content.transcript'),
    downloadTranscript: t('learner.content.downloadTranscript'), transcriptUnavailable: t('learner.content.transcriptUnavailable'),
    downloadCaptions: t('learner.content.downloadCaptions'), code: t('learner.content.code'), source: t('learner.content.source'),
    scrollableTable: t('learner.content.scrollableTable'), scrollableTableCaption: (caption) => t('learner.content.scrollableTableCaption', { caption }),
    download: t('learner.content.download'), downloadUnavailable: t('learner.content.downloadUnavailable'),
    knowledgeTitle: t('learner.assessment.title'), knowledgeUnavailable: t('learner.content.knowledgeUnavailable'), unsupported: t('learner.content.unsupported'),
  }
  const knowledgeCopy: LearnerKnowledgeCheckCopy = {
    title: t('learner.assessment.title'), practice: t('learner.assessment.practice'), submitted: t('learner.assessment.submitted'),
    unsaved: t('learner.assessment.unsaved'), saved: t('learner.assessment.saved'), instruction: t('learner.assessment.instruction'),
    chooseMatch: t('learner.assessment.chooseMatch'), save: t('learner.assessment.save'), saving: t('learner.assessment.saving'),
    submit: t('learner.assessment.submit'), submitting: t('learner.assessment.submitting'), conflict: t('learner.assessment.conflict'), resultTitle: t('learner.assessment.resultTitle'),
    error: (code: LearnerAttemptError) => t(`learner.assessment.errors.${code}`),
    result: (correct, total, percentage) => t('learner.assessment.result', { correct: number(correct, 'count'), total: number(total, 'count'), percentage: number(percentage / 100, 'percent') }),
  }
  return { blockCopy, knowledgeCopy }
}
