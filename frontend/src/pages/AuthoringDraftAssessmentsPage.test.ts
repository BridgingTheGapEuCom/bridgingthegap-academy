import { cleanup, screen } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { authoringDraftContextKey } from '../authoring/draftContext'
import { pseudoLocalize } from '../i18n/pseudo'
import { renderWithI18n } from '../test/i18n'

const listAuthoringDraftAssessmentsMock = vi.hoisted(() => vi.fn())
const createAuthoringAssessmentMock = vi.hoisted(() => vi.fn())

vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  listAuthoringDraftAssessments: listAuthoringDraftAssessmentsMock,
  createAuthoringAssessment: createAuthoringAssessmentMock,
}))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

import AuthoringDraftAssessmentsPage from './AuthoringDraftAssessmentsPage.vue'

const draft = {
  id: '11111111-1111-4111-8111-111111111111', course_id: '55555555-5555-4555-8555-555555555555', intended_version: '1.0.0', source_language: 'en', title: 'Integration foundations', description: 'Draft.', objectives: ['Explain ownership'], changelog: 'Initial Draft.', license: { kind: 'STANDARD' as const, identifier: '', display_name: 'All Rights Reserved', url: '', custom_text: '' }, status: 'ACTIVE' as const, revision: 3, created_at: '2026-09-15T10:00:00Z', updated_at: '2026-09-15T11:00:00Z',
}

function renderPage(locale: 'en' | 'en-XA' = 'en') {
  return renderWithI18n(AuthoringDraftAssessmentsPage, {
    global: {
      provide: { [authoringDraftContextKey as symbol]: { draft: ref(draft), markDraftUnavailable: vi.fn() } },
      components: { RouterLink: { props: ['to'], template: '<a href="#"><slot /></a>' } },
    },
  }, locale)
}

describe('AuthoringDraftAssessmentsPage', () => {
  beforeEach(() => {
    listAuthoringDraftAssessmentsMock.mockReset()
    createAuthoringAssessmentMock.mockReset()
    listAuthoringDraftAssessmentsMock.mockResolvedValue({ items: [{ assessmentKey: '66666666-6666-4666-8666-666666666666', title: 'Terminology check', questionCount: 2, revision: 4, updatedAt: '2026-09-17T10:00:00Z' }], limit: 20, offset: 0, total: 1 })
  })
  afterEach(cleanup)

  it('uses localized assessment metadata and actions under the pseudo-locale', async () => {
    renderPage('en-XA')
    expect(await screen.findByRole('heading', { level: 2, name: pseudoLocalize('Assessments') })).toBeTruthy()
    expect(screen.getByRole('button', { name: pseudoLocalize('Create assessment') })).toBeTruthy()
    expect(screen.getByText(pseudoLocalize('Revision'))).toBeTruthy()
    expect(screen.getByRole('link', { name: pseudoLocalize('Edit {title}').replace('{title}', 'Terminology check') })).toBeTruthy()
  })
})
