package authoring

import (
	"context"
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

const DefaultAssetListLimit = 20
const MaxAssetListLimit = 100

type AssetListRepository = assets.AvailableAssetLister
type AssetListPage struct {
	Items                []assets.Asset
	Limit, Offset, Total int
}
type AssetListService struct {
	repository AssetListRepository
	authorizer Authorizer
}

func NewAssetListService(repository AssetListRepository, authorizer Authorizer) *AssetListService {
	return &AssetListService{repository: repository, authorizer: authorizer}
}
func (s *AssetListService) List(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, limit, offset int) (AssetListPage, error) {
	if s == nil || s.repository == nil || s.authorizer == nil {
		return AssetListPage{}, ErrAuthorizationUnavailable
	}
	if limit == 0 {
		limit = DefaultAssetListLimit
	}
	if limit < 1 || limit > MaxAssetListLimit || offset < 0 {
		return AssetListPage{}, assets.ErrInvalidAsset
	}
	if err := s.authorizer.Authorize(ctx, actor, CapabilityAssetUpload, DraftResource(draftID)); err != nil {
		if errors.Is(err, ErrAuthorizationDenied) {
			return AssetListPage{}, ErrNotFound
		}
		return AssetListPage{}, err
	}
	items, total, err := s.repository.ListAvailableAssetsForDraft(ctx, string(draftID), limit, offset)
	if err != nil {
		return AssetListPage{}, err
	}
	return AssetListPage{Items: items, Limit: limit, Offset: offset, Total: total}, nil
}
