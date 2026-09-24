package assets

import (
	"context"
	"errors"
	"io"
)

type Origin string

const (
	OriginAuthoringDraft Origin = "AUTHORING_DRAFT"
	OriginPackageImport  Origin = "PACKAGE_IMPORT"
)

type ImportedAssetInput struct {
	ImportID         string
	PackageAssetKey  string
	OriginalFilename string
	MediaType        string
	ByteSize         int64
	SHA256Digest     SHA256Digest
	Content          io.Reader
}

func (in ImportedAssetInput) Validate() error {
	if in.ImportID == "" || in.PackageAssetKey == "" || in.Content == nil || in.ByteSize <= 0 || ValidateOriginalFilename(in.OriginalFilename) != nil {
		return ErrInvalidAsset
	}
	if _, err := NormalizeMediaType(in.MediaType); err != nil {
		return ErrInvalidAsset
	}
	if _, err := ParseSHA256Digest(string(in.SHA256Digest)); err != nil {
		return ErrInvalidAsset
	}
	return nil
}

var ErrImportedAssetUnavailable = errors.New("imported asset ingestion unavailable")

type ImportedAssetMetadata struct {
	ImportID         string
	PackageAssetKey  string
	OriginalFilename string
	MediaType        string
	ByteSize         int64
	SHA256Digest     SHA256Digest
	StorageObjectID  StorageObjectID
}

func (in ImportedAssetMetadata) Validate() error {
	if in.ImportID == "" || in.PackageAssetKey == "" || in.ByteSize <= 0 || ValidateOriginalFilename(in.OriginalFilename) != nil {
		return ErrInvalidAsset
	}
	if normalized, err := NormalizeMediaType(in.MediaType); err != nil || normalized != in.MediaType {
		return ErrInvalidAsset
	}
	if _, err := ParseSHA256Digest(string(in.SHA256Digest)); err != nil {
		return ErrInvalidAsset
	}
	if _, err := ParseStorageObjectID(string(in.StorageObjectID)); err != nil {
		return ErrInvalidAsset
	}
	return nil
}

type ImportRepository interface {
	CreateImportedAvailableAsset(context.Context, ImportedAssetMetadata) (Asset, error)
}
type ImportIngestionService struct {
	repository ImportRepository
	storage    BinaryStorage
	maxBytes   int64
}

func NewImportIngestionService(repository ImportRepository, storage BinaryStorage, maxBytes int64) (*ImportIngestionService, error) {
	if repository == nil || storage == nil || maxBytes <= 0 {
		return nil, ErrInvalidAsset
	}
	return &ImportIngestionService{repository, storage, maxBytes}, nil
}
func (s *ImportIngestionService) ImportValidatedAsset(ctx context.Context, in ImportedAssetInput) (Asset, error) {
	if s == nil || in.Validate() != nil {
		return Asset{}, ErrInvalidAsset
	}
	stored, err := s.storage.Put(ctx, newBoundedReader(contextReader{ctx: ctx, reader: in.Content}, s.maxBytes))
	if err != nil {
		return Asset{}, err
	}
	if stored.Validate() != nil || stored.ByteSize != in.ByteSize || stored.SHA256Digest != in.SHA256Digest {
		return Asset{}, combineRollback(ErrInvalidAsset, s.storage.DiscardUncommitted(context.Background(), stored.StorageObjectID))
	}
	asset, err := s.repository.CreateImportedAvailableAsset(ctx, ImportedAssetMetadata{ImportID: in.ImportID, PackageAssetKey: in.PackageAssetKey, OriginalFilename: in.OriginalFilename, MediaType: in.MediaType, ByteSize: in.ByteSize, SHA256Digest: in.SHA256Digest, StorageObjectID: stored.StorageObjectID})
	if err != nil {
		return Asset{}, combineRollback(err, s.storage.DiscardUncommitted(context.Background(), stored.StorageObjectID))
	}
	return asset, nil
}
