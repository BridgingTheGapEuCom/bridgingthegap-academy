package assets

import (
	"context"
	"io"
)

// StoredBinary is provider-neutral proof of the exact bytes accepted by a
// BinaryStorage implementation.
type StoredBinary struct {
	StorageObjectID StorageObjectID
	ByteSize        int64
	SHA256Digest    SHA256Digest
}

func (s StoredBinary) Validate() error {
	if !validUUID(string(s.StorageObjectID)) || s.ByteSize <= 0 {
		return ErrInvalidAsset
	}
	if parsed, err := ParseSHA256Digest(string(s.SHA256Digest)); err != nil || parsed != s.SHA256Digest {
		return ErrInvalidAsset
	}
	return nil
}

// BinaryStorage deliberately exposes no paths, buckets, URLs, or SDK values.
// Put creates the provider-controlled object identity. Open is the future
// delivery/read boundary. Destructive deletion is omitted until retention can
// prove that no immutable CourseVersion depends on the object.
type BinaryStorage interface {
	Put(context.Context, io.Reader) (StoredBinary, error)
	Open(context.Context, StorageObjectID) (io.ReadCloser, error)
}
