import { cleanup, fireEvent, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import AuthoringAssessmentAttachment, { type AuthoringAssessmentAttachmentCopy } from './AuthoringAssessmentAttachment.vue'

vi.mock('../authoring/authoring', async (original) => ({
  ...(await original<typeof import('../authoring/authoring')>()),
  listAuthoringDraftAssessments: vi.fn().mockResolvedValue({ items: [{ assessmentKey: '33333333-3333-4333-8333-333333333333', title: 'Knowledge check', questionCount: 2, revision: 1, updatedAt: '2026-10-06T10:00:00Z' }], limit: 20, offset: 0, total: 1 }),
}))

const copy: AuthoringAssessmentAttachmentCopy = {
  title: 'Assessment', selected: 'An assessment is selected.', selectedUnavailable: 'Selected assessment unavailable.', loading: 'Loading assessments…', loadUnavailable: 'Could not load assessments.', retry: 'Retry', empty: 'No assessments.', available: 'Available assessments', use: (title) => `Use ${title}`, questions: (count) => `${count} questions`, updated: (date) => `Updated ${date}`, previous: 'Previous', next: 'Next', pagination: (from, to, total) => `Assessments ${from}-${to} of ${total}`,
}

describe('AuthoringAssessmentAttachment', () => {
  afterEach(cleanup)

  it('uses explicit caller copy without application i18n and emits the selected assessment', async () => {
    const view = render(AuthoringAssessmentAttachment, { props: { draftId: '11111111-1111-4111-8111-111111111111', lessonId: '22222222-2222-4222-8222-222222222222', blockKey: 'knowledge-check', currentAssessmentKey: '', copy } })
    await screen.findByRole('list', { name: 'Available assessments' })
    await fireEvent.click(screen.getByRole('button', { name: 'Use Knowledge check' }))
    expect(view.emitted('attached')).toEqual([['33333333-3333-4333-8333-333333333333']])
  })
})
