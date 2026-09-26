export type AssetAttachmentType = 'IMAGE' | 'VIDEO' | 'AUDIO' | 'DOWNLOAD'

export function assetAcceptHint(type: AssetAttachmentType): string | undefined {
  switch (type) {
    case 'IMAGE': return 'image/*'
    case 'VIDEO': return 'video/*'
    case 'AUDIO': return 'audio/*'
    case 'DOWNLOAD': return undefined
  }
}

// Compatibility is intentionally based on the server-detected media type.
// It does not inspect the local filename or browser-declared File.type.
export function acceptsUploadedMedia(type: AssetAttachmentType, mediaType: string): boolean {
  switch (type) {
    case 'IMAGE': return mediaType.startsWith('image/')
    case 'VIDEO': return mediaType.startsWith('video/')
    case 'AUDIO': return mediaType.startsWith('audio/')
    case 'DOWNLOAD': return true
  }
}

export function assetTypeLabel(type: AssetAttachmentType): string {
  switch (type) {
    case 'IMAGE': return 'image'
    case 'VIDEO': return 'video'
    case 'AUDIO': return 'audio'
    case 'DOWNLOAD': return 'file'
  }
}

export function formatAssetByteSize(byteSize: number): string {
  if (byteSize < 1024) return `${byteSize} bytes`
  if (byteSize < 1024 * 1024) return `${(byteSize / 1024).toFixed(1)} KB`
  return `${(byteSize / (1024 * 1024)).toFixed(1)} MB`
}
