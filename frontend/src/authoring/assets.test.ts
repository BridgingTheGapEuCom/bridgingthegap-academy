import { describe, expect, it } from 'vitest'
import { acceptsUploadedMedia, assetAcceptHint, formatAssetByteSize } from './assets'

describe('authoring asset media compatibility', () => {
  it('uses server-detected media types for block compatibility', () => {
    expect(acceptsUploadedMedia('IMAGE', 'image/png')).toBe(true)
    expect(acceptsUploadedMedia('IMAGE', 'text/plain')).toBe(false)
    expect(acceptsUploadedMedia('VIDEO', 'video/mp4')).toBe(true)
    expect(acceptsUploadedMedia('VIDEO', 'audio/mpeg')).toBe(false)
    expect(acceptsUploadedMedia('AUDIO', 'audio/mpeg')).toBe(true)
    expect(acceptsUploadedMedia('AUDIO', 'image/png')).toBe(false)
    expect(acceptsUploadedMedia('DOWNLOAD', 'application/octet-stream')).toBe(true)
  })

  it('uses accept only as a native file-picker hint', () => {
    expect(assetAcceptHint('IMAGE')).toBe('image/*')
    expect(assetAcceptHint('VIDEO')).toBe('video/*')
    expect(assetAcceptHint('AUDIO')).toBe('audio/*')
    expect(assetAcceptHint('DOWNLOAD')).toBeUndefined()
    expect(formatAssetByteSize(999)).toBe('999 bytes')
    expect(formatAssetByteSize(1536)).toBe('1.5 KB')
  })
})
