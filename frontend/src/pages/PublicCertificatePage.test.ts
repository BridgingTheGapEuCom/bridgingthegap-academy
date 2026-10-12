import { cleanup, screen } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { pseudoLocalize } from '../i18n/pseudo'
import { renderWithI18n } from '../test/i18n'

const certificateID = '22222222-2222-4222-8222-222222222222'
const read = vi.hoisted(() => vi.fn())
vi.mock('../courses/certificates', async (importOriginal) => ({
  ...await importOriginal<typeof import('../courses/certificates')>(),
  getPublicCertificate: read,
}))

import PublicCertificatePage from './PublicCertificatePage.vue'

const certificate = {
  certificateId: certificateID,
  status: 'ACTIVE' as const,
  issuedAt: '2026-01-01T00:00:00Z',
  revokedAt: null,
  achievement: { courseId: '11111111-1111-4111-8111-111111111111', title: 'Public authored course title', version: '1.0.0', language: 'en', criteria: 'Complete every lesson.' },
  issuer: { id: 'https://academy.example', name: 'Academy' },
}

async function renderPage(locale: 'en' | 'en-XA' = 'en') {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/verify/certificates/:certificateId', component: PublicCertificatePage }] })
  await router.push(`/verify/certificates/${certificateID}`)
  await router.isReady()
  return renderWithI18n(PublicCertificatePage, { global: { plugins: [router] } }, locale)
}

describe('public certificate verification page', () => {
  beforeEach(() => read.mockReset().mockResolvedValue(certificate))
  afterEach(cleanup)

  it('renders a recipient-free public verification result', async () => {
    await renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: 'Certificate verification' })).toBeTruthy()
    expect(screen.getByText('Valid certificate')).toBeTruthy()
    expect(screen.getByRole('heading', { level: 2, name: certificate.achievement.title })).toBeTruthy()
    expect(screen.queryByText(/email|learner id|user id/i)).toBeNull()
  })

  it('pseudo-localizes registry chrome without changing authored data', async () => {
    await renderPage('en-XA')
    expect(await screen.findByRole('heading', { level: 1, name: pseudoLocalize('Certificate verification') })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 2, name: certificate.achievement.title })).toBeTruthy()
  })
})
