package postgres

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type Repository struct{ q *sqlc.Queries }

var _ assets.Repository = (*Repository)(nil)

func New(db sqlc.DBTX) *Repository { return &Repository{q: sqlc.New(db)} }

func (r *Repository) CreateAsset(ctx context.Context, input assets.AssetInput) (assets.Asset, error) {
	if input.Validate() != nil {
		return assets.Asset{}, assets.ErrInvalidAsset
	}
	ownerDraftID, err := uuid(input.OwnerDraftID)
	if err != nil {
		return assets.Asset{}, assets.ErrInvalidAsset
	}
	createdByUserID, err := uuid(input.CreatedByUserID)
	if err != nil {
		return assets.Asset{}, assets.ErrInvalidAsset
	}
	params := sqlc.CreateAssetParams{
		OwnerDraftID: ownerDraftID, OriginalFilename: input.OriginalFilename, MediaType: input.MediaType,
		Lifecycle: string(assets.LifecyclePending), CreatedByUserID: createdByUserID,
	}
	row, err := r.q.CreateAsset(ctx, params)
	if err != nil {
		return assets.Asset{}, storageError(err)
	}
	return mapAsset(row)
}

func (r *Repository) GetAsset(ctx context.Context, id assets.AssetID) (assets.Asset, error) {
	key, err := uuid(string(id))
	if err != nil {
		return assets.Asset{}, assets.ErrInvalidAsset
	}
	row, err := r.q.GetAsset(ctx, key)
	if err != nil {
		return assets.Asset{}, storageError(err)
	}
	return mapAsset(row)
}

func (r *Repository) MarkAssetAvailable(ctx context.Context, id assets.AssetID, stored assets.StoredBinary) (assets.Asset, error) {
	if stored.Validate() != nil {
		return assets.Asset{}, assets.ErrInvalidAsset
	}
	assetID, err := uuid(string(id))
	if err != nil {
		return assets.Asset{}, assets.ErrInvalidAsset
	}
	storageObjectID, err := uuid(string(stored.StorageObjectID))
	if err != nil {
		return assets.Asset{}, assets.ErrInvalidAsset
	}
	row, err := r.q.MarkAssetAvailable(ctx, sqlc.MarkAssetAvailableParams{
		ID: assetID, ByteSize: pgtype.Int8{Int64: stored.ByteSize, Valid: true},
		Sha256Digest: pgtype.Text{String: string(stored.SHA256Digest), Valid: true}, StorageObjectID: storageObjectID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return assets.Asset{}, assets.ErrInvalidLifecycleTransition
	}
	if err != nil {
		return assets.Asset{}, storageError(err)
	}
	return mapAsset(row)
}

func (r *Repository) DiscardPendingAsset(ctx context.Context, id assets.AssetID) error {
	key, err := uuid(string(id))
	if err != nil {
		return assets.ErrInvalidAsset
	}
	rows, err := r.q.DiscardPendingAsset(ctx, key)
	if err != nil {
		return storageError(err)
	}
	if rows == 0 {
		return assets.ErrInvalidLifecycleTransition
	}
	return nil
}

func (r *Repository) ListAvailableAssetsForDraft(ctx context.Context, draftID string, limit, offset int) ([]assets.Asset, int, error) {
	ownerDraftID, err := uuid(draftID)
	if err != nil || limit < 1 || offset < 0 {
		return nil, 0, assets.ErrInvalidAsset
	}
	rows, err := r.q.ListAvailableAssetsForDraft(ctx, sqlc.ListAvailableAssetsForDraftParams{OwnerDraftID: ownerDraftID, Limit: int32(limit), Offset: int32(offset)})
	if err != nil {
		return nil, 0, storageError(err)
	}
	total, err := r.q.CountAvailableAssetsForDraft(ctx, ownerDraftID)
	if err != nil {
		return nil, 0, storageError(err)
	}
	items := make([]assets.Asset, 0, len(rows))
	for _, row := range rows {
		asset, err := mapAsset(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, asset)
	}
	return items, int(total), nil
}

func mapAsset(row sqlc.AssetsAsset) (assets.Asset, error) {
	lifecycle, err := assets.ParseLifecycle(row.Lifecycle)
	if err != nil {
		return assets.Asset{}, assets.ErrInvalidAsset
	}
	asset := assets.Asset{
		ID: assets.AssetID(row.ID.String()), OwnerDraftID: row.OwnerDraftID.String(), OriginalFilename: row.OriginalFilename,
		MediaType: row.MediaType, Lifecycle: lifecycle, CreatedByUserID: row.CreatedByUserID.String(), CreatedAt: row.CreatedAt.Time.UTC(),
	}
	if row.ByteSize.Valid {
		asset.ByteSize = row.ByteSize.Int64
	}
	if row.Sha256Digest.Valid {
		asset.SHA256Digest = assets.SHA256Digest(row.Sha256Digest.String)
	}
	if row.StorageObjectID.Valid {
		asset.StorageObjectID = assets.StorageObjectID(row.StorageObjectID.String())
	}
	if err := asset.Validate(); err != nil {
		return assets.Asset{}, err
	}
	return asset, nil
}

func uuid(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return id, assets.ErrInvalidAsset
	}
	return id, nil
}

func storageError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return assets.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return assets.ErrConflict
		case "23502", "23514", "22P02":
			return assets.ErrInvalidAsset
		}
	}
	return errors.New("asset storage failure")
}
