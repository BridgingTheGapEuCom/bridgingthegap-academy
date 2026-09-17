import { describe, expect, it } from 'vitest'
import { publishedAssetURL } from './assets'

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
