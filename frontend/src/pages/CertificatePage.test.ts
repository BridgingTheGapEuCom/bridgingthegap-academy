import { cleanup, screen } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { pseudoLocalize } from '../i18n/pseudo'
import { renderWithI18n } from '../test/i18n'

const certificateID = '22222222-2222-4222-8222-222222222222'
const auth = { request: vi.fn() }
const read = vi.hoisted(() => vi.fn())
vi.mock('../auth/auth', () => ({ useAuth: () => auth }))
vi.mock('../courses/certificates', async (importOriginal) => ({
  ...await importOriginal<typeof import('../courses/certificates')>(),
  getLearnerCertificate: read,
}))

import CertificatePage from './CertificatePage.vue'

const certificate = {
  certificateId: certificateID,
  status: 'ACTIVE' as const,
  issuedAt: '2026-01-01T00:00:00Z',
  revokedAt: null,
  achievement: { courseId: '11111111-1111-4111-8111-111111111111', title: 'A very long authored course title that remains unchanged', version: '1.0.0', language: 'en', criteria: 'Complete every lesson.' },
  issuer: { id: 'https://academy.example', name: 'Academy' },
  verificationPath: `/verify/certificates/${certificateID}`,
}

async function renderPage(locale: 'en' | 'en-XA' = 'en') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/certificates/:certificateId', component: CertificatePage },
      { path: '/verify/certificates/:certificateId', component: { template: '<div />' } },
    ],
  })
  await router.push(`/certificates/${certificateID}`)
  await router.isReady()
  return renderWithI18n(CertificatePage, { global: { plugins: [router] } }, locale)
}

describe('learner certificate page', () => {
  beforeEach(() => read.mockReset().mockResolvedValue(certificate))
  afterEach(cleanup)

  it('renders the learner-owned record with localized metadata and an intact identifier', async () => {
    await renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: certificate.achievement.title })).toBeTruthy()
    expect(screen.getByText('Valid')).toBeTruthy()
    expect(screen.getByText(/Jan 1, 2026/)).toBeTruthy()
    expect(screen.getByText(certificateID)).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Open public verification' }).getAttribute('href')).toBe(`/verify/certificates/${certificateID}`)
    expect(read).toHaveBeenCalledWith(certificateID, auth)
  })

  it('presents revoked state and pseudo-localizes Academy copy without changing authored data', async () => {
    read.mockResolvedValue({ ...certificate, status: 'REVOKED', revokedAt: '2026-02-02T00:00:00Z' })
    await renderPage('en-XA')
    expect(await screen.findByText(pseudoLocalize('Revoked'))).toBeTruthy()
    expect(screen.getByRole('heading', { level: 1, name: certificate.achievement.title })).toBeTruthy()
  })
})
