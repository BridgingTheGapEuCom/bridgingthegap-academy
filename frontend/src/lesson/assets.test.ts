import { describe, expect, it } from 'vitest'
import { draftAssetURL, lessonAssetURL, publishedAssetURL } from './assets'

describe('publishedAssetURL', () => {
  it('uses only encoded exact CourseVersion coordinates and the canonical asset key', () => {
    expect(publishedAssetURL(
      { courseID: '10000000-0000-4000-8000-000000000001', version: '1.2.3' },
      { assetKey: '50000000-0000-4000-8000-000000000001' },
    )).toBe('/api/courses/by-id/10000000-0000-4000-8000-000000000001/versions/1.2.3/assets/50000000-0000-4000-8000-000000000001')
  })

  it('uses a server-enforced attachment request for downloads', () => {
    expect(publishedAssetURL({ courseID: 'course/id', version: '1.2.3' }, { assetKey: 'asset/key' }, true)).toBe('/api/courses/by-id/course%2Fid/versions/1.2.3/assets/asset%2Fkey?download=1')
  })
})

describe('draftAssetURL', () => {
  it('uses only encoded Draft and Asset identities on the authenticated authoring route', () => {
    expect(draftAssetURL(
      { draftID: '11111111-1111-4111-8111-111111111111' },
      { assetKey: '55555555-5555-4555-8555-555555555555' },
    )).toBe('/api/authoring/drafts/11111111-1111-4111-8111-111111111111/assets/55555555-5555-4555-8555-555555555555/content')
    expect(draftAssetURL({ draftID: 'draft/id' }, { assetKey: 'asset/key' }, true)).toBe('/api/authoring/drafts/draft%2Fid/assets/asset%2Fkey/content?download=1')
  })

  it('keeps published and Draft rendering contexts separate', () => {
    expect(lessonAssetURL({ kind: 'published', courseID: 'course', version: '1.0.0' }, { assetKey: 'asset' })).toContain('/api/courses/by-id/')
    expect(lessonAssetURL({ kind: 'draft-preview', draftID: 'draft' }, { assetKey: 'asset' })).toContain('/api/authoring/drafts/')
  })
})
