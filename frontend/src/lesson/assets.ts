export type PublishedAssetReference = { assetKey: string }
export type PublishedAssetResolution = { kind: 'unresolved'; assetKey: string }

// M2.3 deliberately stores logical asset keys without inventing a delivery URL.
// Future asset delivery can extend this boundary without teaching renderers how
// to manufacture paths from opaque keys.
export function resolvePublishedAsset(asset: PublishedAssetReference): PublishedAssetResolution {
  return { kind: 'unresolved', assetKey: asset.assetKey }
}
