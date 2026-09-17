import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { shallowRef } from 'vue'
import { APIProblemError } from '../api/client'

type AuthenticationState =
  | { status: 'bootstrapping' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; userId: string; expiresAt: string }
  | { status: 'unavailable' }

const authMock = vi.hoisted(() => ({ state: { value: { status: 'authenticated' } as AuthenticationState }, bootstrapSession: vi.fn() }))
const createAuthoringDraftMock = vi.hoisted(() => vi.fn())

vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))
vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  createAuthoringDraft: createAuthoringDraftMock,
}))

import AuthoringDraftCreatePage from './AuthoringDraftCreatePage.vue'

const draftID = '11111111-1111-4111-8111-111111111111'
const createdDraft = {
  id: draftID, course_id: '22222222-2222-4222-8222-222222222222', intended_version: '0.1.0', source_language: 'en', title: 'First Draft', description: 'A complete description.', objectives: ['Explain the topic'], changelog: 'Initial Draft.',
  license: { kind: 'ALL_RIGHTS_RESERVED' as const, identifier: '', display_name: 'All Rights Reserved', url: '', custom_text: '' }, status: 'ACTIVE' as const, revision: 1, created_at: '2026-09-17T10:00:00Z', updated_at: '2026-09-17T10:00:00Z',
}

function routerFor() {
  return createRouter({ history: createMemoryHistory(), routes: [
    { path: '/authoring/new', component: AuthoringDraftCreatePage },
    { path: '/authoring', component: { template: '<p>Authoring home</p>' } },
    { path: '/authoring/drafts/:draftId/overview', component: { template: '<p>Draft workspace</p>' } },
    { path: '/login', component: { template: '<p>Login</p>' } },
  ] })
}

async function renderPage() {
  const router = routerFor()
  await router.push('/authoring/new')
  await router.isReady()
  return { ...render({ template: '<RouterView />' }, { global: { plugins: [router] } }), router }
}

async function completeForm() {
  await fireEvent.update(screen.getByRole('textbox', { name: /^Title required$/ }), 'First Draft')
  await fireEvent.update(screen.getByRole('textbox', { name: /^Description required$/ }), 'A complete description.')
  await fireEvent.update(screen.getByRole('textbox', { name: /^Learning objectives required$/ }), 'Explain the topic')
}

describe('AuthoringDraftCreatePage', () => {
  beforeEach(() => {
    authMock.state = shallowRef<AuthenticationState>({ status: 'authenticated', userId: '33333333-3333-4333-8333-333333333333', expiresAt: '2026-09-17T12:00:00Z' })
    authMock.bootstrapSession.mockReset()
    createAuthoringDraftMock.mockReset()
    createAuthoringDraftMock.mockResolvedValue(createdDraft)
  })
  afterEach(cleanup)

  it('uses labelled native controls and creates from the authoritative result only', async () => {
    const { router } = await renderPage()
    await completeForm()
    await fireEvent.click(screen.getByRole('button', { name: 'Create Draft' }))
    await waitFor(() => expect(createAuthoringDraftMock).toHaveBeenCalledWith({
      title: 'First Draft', intendedVersion: '0.1.0', sourceLanguage: 'en', description: 'A complete description.', objectives: ['Explain the topic'], changelog: 'Initial Draft.',
    }))
    await waitFor(() => expect(router.currentRoute.value.path).toBe(`/authoring/drafts/${draftID}/overview`))
    expect(createAuthoringDraftMock.mock.calls[0][0]).not.toHaveProperty('userId')
  })

  it('blocks invalid input accessibly without sending a request and preserves values', async () => {
    await renderPage()
    await fireEvent.update(screen.getByRole('textbox', { name: /^Title required$/ }), 'Retained title')
    await fireEvent.click(screen.getByRole('button', { name: 'Create Draft' }))
    expect(await screen.findByText('Enter a description.')).toBeTruthy()
    expect((screen.getByRole('textbox', { name: /^Title required$/ }) as HTMLInputElement).value).toBe('Retained title')
    expect(createAuthoringDraftMock).not.toHaveBeenCalled()
  })

  it('disables duplicate submission and keeps field values for sanitised API failures', async () => {
    let resolveCreate: (value: typeof createdDraft) => void = () => undefined
    createAuthoringDraftMock.mockImplementationOnce(() => new Promise<typeof createdDraft>((resolve) => { resolveCreate = resolve }))
    await renderPage()
    await completeForm()
    await fireEvent.click(screen.getByRole('button', { name: 'Create Draft' }))
    expect(screen.getByRole('status').textContent).toContain('Creating Draft')
    expect((screen.getByRole('button', { name: 'Creating…' }) as HTMLButtonElement).disabled).toBe(true)
    await fireEvent.click(screen.getByRole('button', { name: 'Creating…' }))
    expect(createAuthoringDraftMock).toHaveBeenCalledTimes(1)
    resolveCreate(createdDraft)
    await waitFor(() => expect(createAuthoringDraftMock).toHaveBeenCalledTimes(1))

    cleanup()
    createAuthoringDraftMock.mockRejectedValueOnce(new APIProblemError(400, undefined, undefined))
    await renderPage()
    await completeForm()
    await fireEvent.click(screen.getByRole('button', { name: 'Create Draft' }))
    expect((await screen.findByRole('alert')).textContent).toContain('Check the fields')
    expect((screen.getByRole('textbox', { name: /^Title required$/ }) as HTMLInputElement).value).toBe('First Draft')
  })

  it('uses the normal back path and drops late responses after navigation', async () => {
    let resolveCreate: (value: typeof createdDraft) => void = () => undefined
    createAuthoringDraftMock.mockImplementationOnce(() => new Promise<typeof createdDraft>((resolve) => { resolveCreate = resolve }))
    const { router } = await renderPage()
    await completeForm()
    await fireEvent.click(screen.getByRole('button', { name: 'Create Draft' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(router.currentRoute.value.path).toBe('/authoring'))
    resolveCreate(createdDraft)
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(router.currentRoute.value.path).toBe('/authoring')
  })

  it('returns an expired creation session to sign in without retrying the mutation', async () => {
    createAuthoringDraftMock.mockImplementationOnce(async () => {
      authMock.state.value = { status: 'unauthenticated' }
      throw new APIProblemError(401, undefined, undefined)
    })
    const { router } = await renderPage()
    await completeForm()
    await fireEvent.click(screen.getByRole('button', { name: 'Create Draft' }))
    await waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
    expect(router.currentRoute.value.query.returnTo).toBe('/authoring/new')
    expect(createAuthoringDraftMock).toHaveBeenCalledTimes(1)
  })
})
