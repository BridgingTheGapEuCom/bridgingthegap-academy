package assets

import (
	"context"
	"errors"
	"io"
)

var (
	ErrAssetTooLarge        = errors.New("asset content exceeds the configured size limit")
	ErrInvalidAssetContent  = errors.New("invalid asset content")
	ErrStorageObjectMissing = errors.New("storage object not found")
	ErrBinaryStorage        = errors.New("binary storage operation failed")
	ErrRollbackIncomplete   = errors.New("asset ingestion rollback incomplete")
)

// StoredBinary is provider-neutral proof of the exact bytes accepted by a
// BinaryStorage implementation.
type StoredBinary struct {
	StorageObjectID StorageObjectID
	ByteSize        int64
	SHA256Digest    SHA256Digest
}

func (s StoredBinary) Validate() error {
	if parsed, err := ParseStorageObjectID(string(s.StorageObjectID)); err != nil || parsed != s.StorageObjectID || s.ByteSize <= 0 {
		return ErrInvalidAsset
	}
	if parsed, err := ParseSHA256Digest(string(s.SHA256Digest)); err != nil || parsed != s.SHA256Digest {
		return ErrInvalidAsset
	}
	return nil
}

// BinaryStorage deliberately exposes no paths, buckets, URLs, or SDK values.
// Put creates the provider-controlled object identity and, on error, must leave
// no committed object. Open is the future
// delivery/read boundary. DiscardUncommitted exists only to compensate for an
// ingestion attempt whose metadata never became AVAILABLE; it is not an Asset
// deletion operation and must never be used for retained AVAILABLE content.
type BinaryStorage interface {
	Put(context.Context, io.Reader) (StoredBinary, error)
	Open(context.Context, StorageObjectID) (io.ReadCloser, error)
	DiscardUncommitted(context.Context, StorageObjectID) error
}
