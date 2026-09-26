package assets

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"
)

const mediaTypeDetectionBytes = 512
const ingestionCleanupTimeout = 5 * time.Second

// IngestionInput contains already-authorized ownership provenance plus the
// binary stream. The neutral Assets module deliberately does not decide Draft
// membership. It also accepts no caller-provided size, digest, media type, or
// storage identity.
type IngestionInput struct {
	OwnerDraftID     string `json:"-"`
	CreatedByUserID  string `json:"-"`
	OriginalFilename string
	Content          io.Reader `json:"-"`
}

type IngestionService struct {
	repository Repository
	storage    BinaryStorage
	maxBytes   int64
}

func NewIngestionService(repository Repository, storage BinaryStorage, maxBytes int64) (*IngestionService, error) {
	if repository == nil || storage == nil || maxBytes <= 0 {
		return nil, ErrInvalidAsset
	}
	return &IngestionService{repository: repository, storage: storage, maxBytes: maxBytes}, nil
}

// Ingest creates PENDING metadata, commits the stream through BinaryStorage,
// and conditionally transitions that exact Asset to AVAILABLE. PostgreSQL and
// binary storage are separate atomicity domains, so failures are reconciled
// with narrow compensating cleanup rather than a distributed transaction.
func (s *IngestionService) Ingest(ctx context.Context, input IngestionInput) (Asset, error) {
	if input.Content == nil || ctx == nil {
		return Asset{}, ErrInvalidAssetContent
	}

	stream := newBoundedReader(contextReader{ctx: ctx, reader: input.Content}, s.maxBytes)
	prefix, err := readDetectionPrefix(stream)
	if err != nil {
		return Asset{}, err
	}
	mediaType, err := detectedMediaType(prefix)
	if err != nil {
		return Asset{}, ErrInvalidAssetContent
	}
	metadata := AssetInput{
		OwnerDraftID: input.OwnerDraftID, OriginalFilename: input.OriginalFilename,
		MediaType: mediaType, CreatedByUserID: input.CreatedByUserID,
	}
	if metadata.Validate() != nil {
		return Asset{}, ErrInvalidAsset
	}

	pending, err := s.repository.CreateAsset(ctx, metadata)
	if err != nil {
		return Asset{}, err
	}
	stored, err := s.storage.Put(ctx, io.MultiReader(bytes.NewReader(prefix), stream))
	if err != nil {
		return Asset{}, combineRollback(err, s.discardPending(ctx, pending.ID))
	}
	available, err := s.repository.MarkAssetAvailable(ctx, pending.ID, stored)
	if err == nil {
		return available, nil
	}

	// An UPDATE may commit before a connection failure reaches the client. Read
	// back first: deleting the blob while the row is actually AVAILABLE would
	// violate the central metadata/blob invariant.
	cleanupCtx, cancel := cleanupContext(ctx)
	defer cancel()
	current, readErr := s.repository.GetAsset(cleanupCtx, pending.ID)
	if readErr == nil && current.Lifecycle == LifecycleAvailable && sameStoredBinary(current, stored) {
		return current, nil
	}
	if readErr == nil && current.Lifecycle == LifecyclePending {
		metadataErr := s.repository.DiscardPendingAsset(cleanupCtx, pending.ID)
		if metadataErr != nil {
			// The row may have become AVAILABLE since readback. Do not delete a
			// blob that another successful transition may now reference.
			return Asset{}, combineRollback(err, metadataErr)
		}
		return Asset{}, combineRollback(err, s.storage.DiscardUncommitted(cleanupCtx, stored.StorageObjectID))
	}
	if errors.Is(readErr, ErrNotFound) {
		return Asset{}, combineRollback(err, s.storage.DiscardUncommitted(cleanupCtx, stored.StorageObjectID))
	}

	// The persisted state is uncertain or conflicts with the just-written blob.
	// Retain the object for operator reconciliation rather than risk breaking an
	// AVAILABLE Asset.
	return Asset{}, errors.Join(err, ErrRollbackIncomplete)
}

func (s *IngestionService) discardPending(ctx context.Context, id AssetID) error {
	cleanupCtx, cancel := cleanupContext(ctx)
	defer cancel()
	return s.repository.DiscardPendingAsset(cleanupCtx, id)
}

func cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), ingestionCleanupTimeout)
}

func combineRollback(primary, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	return errors.Join(primary, ErrRollbackIncomplete)
}

func sameStoredBinary(asset Asset, stored StoredBinary) bool {
	return asset.StorageObjectID == stored.StorageObjectID && asset.ByteSize == stored.ByteSize && asset.SHA256Digest == stored.SHA256Digest
}

func readDetectionPrefix(reader io.Reader) ([]byte, error) {
	buffer := make([]byte, mediaTypeDetectionBytes)
	n, err := io.ReadFull(reader, buffer)
	switch {
	case err == nil:
	case errors.Is(err, io.ErrUnexpectedEOF) && n > 0:
	case errors.Is(err, ErrAssetTooLarge):
		return nil, ErrAssetTooLarge
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return nil, err
	default:
		return nil, ErrInvalidAssetContent
	}
	return buffer[:n], nil
}

func detectedMediaType(prefix []byte) (string, error) {
	value, _, err := mime.ParseMediaType(http.DetectContentType(prefix))
	if err != nil {
		return "", err
	}
	return NormalizeMediaType(value)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

// boundedReader allows exactly max bytes and then probes one additional byte.
// The over-limit byte is never passed to storage.
type boundedReader struct {
	reader    io.Reader
	remaining int64
	checked   bool
}

func newBoundedReader(reader io.Reader, maxBytes int64) *boundedReader {
	return &boundedReader{reader: reader, remaining: maxBytes}
}

func (r *boundedReader) Read(buffer []byte) (int, error) {
	if r.remaining > 0 {
		if int64(len(buffer)) > r.remaining {
			buffer = buffer[:r.remaining]
		}
		n, err := r.reader.Read(buffer)
		r.remaining -= int64(n)
		return n, err
	}
	if r.checked {
		return 0, io.EOF
	}
	r.checked = true
	var probe [1]byte
	n, err := io.ReadFull(r.reader, probe[:])
	if n > 0 {
		return 0, ErrAssetTooLarge
	}
	return 0, err
}
