import { describe, expect, it, vi } from 'vitest'
import { getAuthoringDraft, isAuthoringDraftID, InvalidAuthoringDraftIDError, updateAuthoringDraft } from './authoring'

describe('Authoring API service', () => {
  it('uses the authenticated shared request boundary for a bounded Draft ID', async () => {
    const request = vi.fn().mockResolvedValue({ id: '11111111-1111-4111-8111-111111111111' })
    await getAuthoringDraft('11111111-1111-4111-8111-111111111111', { request })
    expect(request).toHaveBeenCalledWith('/api/authoring/drafts/11111111-1111-4111-8111-111111111111')
  })

  it('rejects malformed Draft IDs before constructing an API path', async () => {
    expect(isAuthoringDraftID('not-a-draft')).toBe(false)
    await expect(getAuthoringDraft('not-a-draft', { request: vi.fn() })).rejects.toBeInstanceOf(InvalidAuthoringDraftIDError)
  })

  it('uses PATCH with the shared authenticated request boundary for metadata updates', async () => {
    const request = vi.fn().mockResolvedValue({})
    await updateAuthoringDraft('11111111-1111-4111-8111-111111111111', { expectedRevision: 3, title: 'Updated' }, { request })
    expect(request).toHaveBeenCalledWith('/api/authoring/drafts/11111111-1111-4111-8111-111111111111', expect.objectContaining({ method: 'PATCH' }))
  })
})
