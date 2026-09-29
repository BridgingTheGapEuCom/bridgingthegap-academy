package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

type DraftAssetRepository interface {
	GetAsset(context.Context, assets.AssetID) (assets.Asset, error)
}

// DraftAssetReadService resolves only AVAILABLE assets owned by the exact
// authorized Draft. The returned storage identity stays inside the server.
type DraftAssetReadService struct {
	repository DraftAssetRepository
	authorizer Authorizer
}

func NewDraftAssetReadService(repository DraftAssetRepository, authorizer Authorizer) *DraftAssetReadService {
	return &DraftAssetReadService{repository: repository, authorizer: authorizer}
}

func (s *DraftAssetReadService) Exact(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, assetID assets.AssetID) (assets.Asset, error) {
	if s == nil || s.repository == nil || s.authorizer == nil {
		return assets.Asset{}, ErrAuthorizationUnavailable
	}
	if err := s.authorizer.Authorize(ctx, actor, CapabilityRead, DraftResource(draftID)); err != nil {
		if errors.Is(err, ErrAuthorizationDenied) {
			return assets.Asset{}, ErrNotFound
		}
		return assets.Asset{}, err
	}
	asset, err := s.repository.GetAsset(ctx, assetID)
	if errors.Is(err, assets.ErrNotFound) || errors.Is(err, assets.ErrInvalidAsset) {
		return assets.Asset{}, ErrNotFound
	}
	if err != nil {
		return assets.Asset{}, err
	}
	origin := asset.Origin
	if origin == "" {
		origin = assets.OriginAuthoringDraft
	}
	if origin != assets.OriginAuthoringDraft || asset.OwnerDraftID != string(draftID) || asset.Lifecycle != assets.LifecycleAvailable || asset.Validate() != nil {
		return assets.Asset{}, ErrNotFound
	}
	return asset, nil
}
