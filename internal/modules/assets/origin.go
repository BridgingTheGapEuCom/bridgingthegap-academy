package assets

import (
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
