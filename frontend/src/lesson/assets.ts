export type PublishedAssetReference = { assetKey: string }
export type PublishedAssetDeliveryContext = { courseID: string; version: string }
export type DraftAssetDeliveryContext = { draftID: string }
export type AssetRenderingContext =
  | { kind: 'published'; courseID: string; version: string }
  | { kind: 'draft-preview'; draftID: string }

// The public path contains only immutable CourseVersion coordinates and the
// canonical asset key. It never contains a storage-object locator or an
// Authoring identity. The server resolves this through the frozen binding.
export function publishedAssetURL(context: PublishedAssetDeliveryContext, asset: PublishedAssetReference, download = false): string {
  const path = `/api/courses/by-id/${encodeURIComponent(context.courseID)}/versions/${encodeURIComponent(context.version)}/assets/${encodeURIComponent(asset.assetKey)}`
  return download ? `${path}?download=1` : path
}

// Draft URLs contain only the authorized Draft and stable Asset identities.
// Storage object identifiers never cross this boundary.
export function draftAssetURL(context: DraftAssetDeliveryContext, asset: PublishedAssetReference, download = false): string {
  const path = `/api/authoring/drafts/${encodeURIComponent(context.draftID)}/assets/${encodeURIComponent(asset.assetKey)}/content`
  return download ? `${path}?download=1` : path
}

export function lessonAssetURL(context: AssetRenderingContext, asset: PublishedAssetReference, download = false): string {
  return context.kind === 'published'
    ? publishedAssetURL(context, asset, download)
    : draftAssetURL(context, asset, download)
}
