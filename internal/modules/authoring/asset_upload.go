package authoring

import (
	"context"
	"errors"
	"io"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

type AssetIngestor interface {
	Ingest(context.Context, assets.IngestionInput) (assets.Asset, error)
}

// AuthorizedAssetUpload captures the exact server-authorized Draft and actor.
// HTTP may parse the binary only after obtaining this closure and cannot
// replace either ownership value with multipart fields.
type AuthorizedAssetUpload func(context.Context, string, io.Reader) (assets.Asset, error)

type AssetUploadService struct {
	ingestor   AssetIngestor
	authorizer Authorizer
}

func NewAssetUploadService(ingestor AssetIngestor, authorizer Authorizer) *AssetUploadService {
	return &AssetUploadService{ingestor: ingestor, authorizer: authorizer}
}

func (s *AssetUploadService) AuthorizeUpload(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID) (AuthorizedAssetUpload, error) {
	if s == nil || s.ingestor == nil || s.authorizer == nil || draftID == "" {
		return nil, ErrAuthorizationUnavailable
	}
	if err := s.authorizer.Authorize(ctx, actor, CapabilityAssetUpload, DraftResource(draftID)); err != nil {
		if errors.Is(err, ErrAuthorizationDenied) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	creatorID := string(actor.UserID())
	return func(ctx context.Context, filename string, content io.Reader) (assets.Asset, error) {
		return s.ingestor.Ingest(ctx, assets.IngestionInput{
			OwnerDraftID: string(draftID), CreatedByUserID: creatorID,
			OriginalFilename: filename, Content: content,
		})
	}, nil
}
