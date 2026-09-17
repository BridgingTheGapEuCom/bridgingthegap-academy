package assets

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("asset record not found")
	ErrConflict = errors.New("asset record conflicts with existing data")
)

// Repository owns only Asset metadata. Binary bytes always cross the separate
// BinaryStorage boundary.
type Repository interface {
	CreateAsset(context.Context, AssetInput) (Asset, error)
	GetAsset(context.Context, AssetID) (Asset, error)
	MarkAssetAvailable(context.Context, AssetID, StoredBinary) (Asset, error)
	DiscardPendingAsset(context.Context, AssetID) error
}
