import { cleanup, fireEvent, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import AuthoringAssetAttachment from './AuthoringAssetAttachment.vue'
import { authoringAssetAttachmentTestCopy } from '../test/authoringAssetAttachment'

vi.mock('../authoring/authoring', async (original) => ({
  ...(await original<typeof import('../authoring/authoring')>()),
  listAuthoringDraftAssets: vi.fn().mockResolvedValue({ items: [], limit: 20, offset: 0, total: 0 }),
  uploadAuthoringAsset: vi.fn(),
}))

describe('AuthoringAssetAttachment', () => {
  afterEach(cleanup)

  it('renders from explicit caller copy without application i18n', async () => {
    render(AuthoringAssetAttachment, { props: { draftId: 'draft', lessonId: 'lesson', blockKey: 'audio', type: 'AUDIO', label: 'Audio', currentAssetKey: '', copy: authoringAssetAttachmentTestCopy } })
    expect(screen.getByRole('region', { name: 'Audio attachment' })).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Choose audio' }))
    expect(screen.getByRole('dialog', { name: 'Change audio' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeTruthy()
  })
})
