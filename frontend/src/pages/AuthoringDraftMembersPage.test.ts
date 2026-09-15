import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { APIProblemError } from '../api/client'
import { authoringDraftContextKey } from '../authoring/draftContext'

const getAuthoringDraftMembersMock = vi.hoisted(() => vi.fn())
const getAuthoringDraftMock = vi.hoisted(() => vi.fn())
const addAuthoringMemberMock = vi.hoisted(() => vi.fn())
const changeAuthoringMemberRoleMock = vi.hoisted(() => vi.fn())
const revokeAuthoringMemberMock = vi.hoisted(() => vi.fn())

vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  getAuthoringDraftMembers: getAuthoringDraftMembersMock,
  getAuthoringDraft: getAuthoringDraftMock,
  addAuthoringMember: addAuthoringMemberMock,
  changeAuthoringMemberRole: changeAuthoringMemberRoleMock,
  revokeAuthoringMember: revokeAuthoringMemberMock,
}))

import AuthoringDraftMembersPage from './AuthoringDraftMembersPage.vue'

const draftID = '11111111-1111-4111-8111-111111111111'
const authorID = '22222222-2222-4222-8222-222222222222'
const maintainerID = '33333333-3333-4333-8333-333333333333'
const addedID = '44444444-4444-4444-8444-444444444444'

const draft = (revision = 3) => ({
  id: draftID, course_id: '55555555-5555-4555-8555-555555555555', intended_version: '1.0.0', source_language: 'en', title: 'Integration foundations', description: 'Draft.', objectives: ['Explain ownership'], changelog: 'Initial Draft.', license: { kind: 'STANDARD' as const, identifier: '', display_name: 'All Rights Reserved', url: '', custom_text: '' }, status: 'ACTIVE' as const, revision, created_at: '2026-09-15T10:00:00Z', updated_at: '2026-09-15T11:00:00Z',
})
const members = () => ({ members: [
  { userId: authorID, role: 'AUTHOR' as const },
  { userId: maintainerID, role: 'MAINTAINER' as const },
] })

async function renderPage() {
  const currentDraft = ref(draft())
  const replaceDraft = vi.fn((nextDraft) => { currentDraft.value = nextDraft })
  const markDraftUnavailable = vi.fn()
  return {
    ...render(AuthoringDraftMembersPage, { global: { provide: { [authoringDraftContextKey as symbol]: { draft: currentDraft, replaceDraft, markDraftUnavailable } } } }),
    currentDraft, replaceDraft, markDraftUnavailable,
  }
}

describe('AuthoringDraftMembersPage', () => {
  beforeEach(() => {
    for (const mock of [getAuthoringDraftMembersMock, getAuthoringDraftMock, addAuthoringMemberMock, changeAuthoringMemberRoleMock, revokeAuthoringMemberMock]) mock.mockReset()
    getAuthoringDraftMembersMock.mockResolvedValue(members())
  })
  afterEach(cleanup)

  it('loads the authoritative active membership list in its server order with opaque IDs and accessible controls', async () => {
    await renderPage()
    const list = await screen.findByRole('list', { name: 'Current Draft members' })
    expect(within(list).getAllByRole('listitem').map((item) => item.textContent)).toEqual([
      expect.stringContaining(authorID), expect.stringContaining(maintainerID),
    ])
    expect(screen.getByRole('combobox', { name: `Role for ${authorID}` })).toBeTruthy()
    expect(screen.getByRole('button', { name: `Revoke access for ${authorID}` })).toBeTruthy()
    expect(getAuthoringDraftMembersMock).toHaveBeenCalledWith(draftID)
  })

  it('shows loading, empty, unavailable, and hidden states without inventing membership data', async () => {
    let resolveMembers: (value: ReturnType<typeof members>) => void = () => undefined
    getAuthoringDraftMembersMock.mockImplementationOnce(() => new Promise<ReturnType<typeof members>>((resolve) => { resolveMembers = resolve }))
    await renderPage()
    expect(screen.getByRole('status').textContent).toContain('Loading current members')
    resolveMembers({ members: [] })
    expect(await screen.findByText('No active members were returned for this Draft.')).toBeTruthy()

    cleanup()
    getAuthoringDraftMembersMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(members())
    await renderPage()
    await screen.findByText('We couldn’t load current members right now.')
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('list', { name: 'Current Draft members' })).toBeTruthy()

    cleanup()
    getAuthoringDraftMembersMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    const { markDraftUnavailable } = await renderPage()
    await waitFor(() => expect(markDraftUnavailable).toHaveBeenCalledTimes(1))
  })

  it('adds a member with the authoritative Draft revision, clears the form, and reloads memberships', async () => {
    addAuthoringMemberMock.mockResolvedValue({ draftRevision: 4 })
    getAuthoringDraftMembersMock.mockResolvedValueOnce(members()).mockResolvedValueOnce({ members: [...members().members, { userId: addedID, role: 'MAINTAINER' as const }] })
    const { currentDraft } = await renderPage()
    await screen.findByRole('list', { name: 'Current Draft members' })
    await fireEvent.update(screen.getByRole('textbox', { name: /User ID/ }), addedID)
    await fireEvent.update(screen.getByRole('combobox', { name: /Role required/ }), 'MAINTAINER')
    await fireEvent.click(screen.getByRole('button', { name: 'Add member' }))
    await waitFor(() => expect(addAuthoringMemberMock).toHaveBeenCalledWith(draftID, { userId: addedID, role: 'MAINTAINER', expectedDraftRevision: 3 }))
    await waitFor(() => expect(currentDraft.value.revision).toBe(4))
    expect((screen.getByRole('textbox', { name: /User ID/ }) as HTMLInputElement).value).toBe('')
    expect(getAuthoringDraftMembersMock).toHaveBeenCalledTimes(2)
    expect(await screen.findByText('Member added.')).toBeTruthy()
  })

  it('changes a targeted role and revokes access only after inline confirmation', async () => {
    changeAuthoringMemberRoleMock.mockResolvedValue({ draftRevision: 4 })
    revokeAuthoringMemberMock.mockResolvedValue({ draftRevision: 5 })
    const { currentDraft } = await renderPage()
    const list = await screen.findByRole('list', { name: 'Current Draft members' })
    const authorRow = within(list).getByText(authorID).closest('li') as HTMLElement
    await fireEvent.update(within(authorRow).getByRole('combobox', { name: `Role for ${authorID}` }), 'MAINTAINER')
    await fireEvent.click(within(authorRow).getByRole('button', { name: 'Save role' }))
    await waitFor(() => expect(changeAuthoringMemberRoleMock).toHaveBeenCalledWith(draftID, authorID, { role: 'MAINTAINER', expectedDraftRevision: 3 }))
    await waitFor(() => expect(currentDraft.value.revision).toBe(4))
    await screen.findByText('Member role updated.')

    const refreshedList = screen.getByRole('list', { name: 'Current Draft members' })
    const maintainerRow = within(refreshedList).getByText(maintainerID).closest('li') as HTMLElement
    await fireEvent.click(within(maintainerRow).getByRole('button', { name: `Revoke access for ${maintainerID}` }))
    expect(revokeAuthoringMemberMock).not.toHaveBeenCalled()
    await fireEvent.click(within(maintainerRow).getByRole('button', { name: `Confirm revoke access for ${maintainerID}` }))
    await waitFor(() => expect(revokeAuthoringMemberMock).toHaveBeenCalledWith(draftID, maintainerID, 4))
    await waitFor(() => expect(currentDraft.value.revision).toBe(5))
  })

  it('preserves local form input on a conflict and explicitly reloads the Draft and memberships', async () => {
    addAuthoringMemberMock.mockRejectedValueOnce(new APIProblemError(409, undefined, undefined))
    getAuthoringDraftMock.mockResolvedValueOnce(draft(4))
    getAuthoringDraftMembersMock.mockResolvedValueOnce(members()).mockResolvedValueOnce({ members: [{ userId: maintainerID, role: 'MAINTAINER' as const }] })
    const { currentDraft } = await renderPage()
    await screen.findByRole('list', { name: 'Current Draft members' })
    await fireEvent.update(screen.getByRole('textbox', { name: /User ID/ }), addedID)
    await fireEvent.click(screen.getByRole('button', { name: 'Add member' }))
    expect(await screen.findByText(/must keep at least one maintainer/)).toBeTruthy()
    expect((screen.getByRole('textbox', { name: /User ID/ }) as HTMLInputElement).value).toBe(addedID)
    expect(addAuthoringMemberMock).toHaveBeenCalledTimes(1)
    await fireEvent.click(screen.getByRole('button', { name: 'Reload latest Draft' }))
    await waitFor(() => expect(currentDraft.value.revision).toBe(4))
    expect(screen.getByRole('list', { name: 'Current Draft members' }).textContent).toContain(maintainerID)
    expect(addAuthoringMemberMock).toHaveBeenCalledTimes(1)
  })

  it('keeps current rows on a last-maintainer or validation failure and reports it accessibly', async () => {
    revokeAuthoringMemberMock.mockRejectedValueOnce(new APIProblemError(409, undefined, undefined))
    await renderPage()
    const list = await screen.findByRole('list', { name: 'Current Draft members' })
    const maintainerRow = within(list).getByText(maintainerID).closest('li') as HTMLElement
    await fireEvent.click(within(maintainerRow).getByRole('button', { name: `Revoke access for ${maintainerID}` }))
    await fireEvent.click(within(maintainerRow).getByRole('button', { name: `Confirm revoke access for ${maintainerID}` }))
    expect(await screen.findByText(/must keep at least one maintainer/)).toBeTruthy()
    expect(screen.getByRole('list', { name: 'Current Draft members' }).textContent).toContain(maintainerID)

    cleanup()
    await renderPage()
    await screen.findByRole('list', { name: 'Current Draft members' })
    await fireEvent.update(screen.getByRole('textbox', { name: /User ID/ }), 'not-a-user-id')
    await fireEvent.click(screen.getByRole('button', { name: 'Add member' }))
    expect(await screen.findByText('Enter a valid user ID.')).toBeTruthy()
    expect(addAuthoringMemberMock).not.toHaveBeenCalled()
  })

  it('keeps form input for validation failures and hides the Draft after a membership mutation becomes unavailable', async () => {
    addAuthoringMemberMock.mockRejectedValueOnce(new APIProblemError(400, undefined, undefined))
    await renderPage()
    await screen.findByRole('list', { name: 'Current Draft members' })
    await fireEvent.update(screen.getByRole('textbox', { name: /User ID/ }), addedID)
    await fireEvent.click(screen.getByRole('button', { name: 'Add member' }))
    expect((await screen.findByRole('alert')).textContent).toContain('We couldn’t save this membership change. Check the values and try again.')
    expect((screen.getByRole('textbox', { name: /User ID/ }) as HTMLInputElement).value).toBe(addedID)

    cleanup()
    addAuthoringMemberMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    const { markDraftUnavailable } = await renderPage()
    await screen.findByRole('list', { name: 'Current Draft members' })
    await fireEvent.update(screen.getByRole('textbox', { name: /User ID/ }), addedID)
    await fireEvent.click(screen.getByRole('button', { name: 'Add member' }))
    await waitFor(() => expect(markDraftUnavailable).toHaveBeenCalledTimes(1))
  })
})
