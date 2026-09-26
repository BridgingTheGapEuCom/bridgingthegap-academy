export type PublishedAssetReference = { assetKey: string }
export type PublishedAssetDeliveryContext = { courseID: string; version: string }

// The public path contains only immutable CourseVersion coordinates and the
// canonical asset key. It never contains a storage-object locator or an
// Authoring identity. The server resolves this through the frozen binding.
export function publishedAssetURL(context: PublishedAssetDeliveryContext, asset: PublishedAssetReference, download = false): string {
  const path = `/api/courses/by-id/${encodeURIComponent(context.courseID)}/versions/${encodeURIComponent(context.version)}/assets/${encodeURIComponent(asset.assetKey)}`
  return download ? `${path}?download=1` : path
}
