package assets_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
)

func TestIngestionMeasuresContentAndTransitionsToAvailable(t *testing.T) {
	content := []byte("plain text content")
	repository := newIngestionRepository()
	storage := newMemoryStorage()
	service := newIngestionService(t, repository, storage, int64(len(content)))

	asset, err := service.Ingest(context.Background(), ingestionInput("misleading.png", bytes.NewReader(content)))
	if err != nil {
		t.Fatal(err)
	}
	wantDigest := sha256.Sum256(content)
	if asset.Lifecycle != assets.LifecycleAvailable || asset.ByteSize != int64(len(content)) ||
		string(asset.SHA256Digest) != hex.EncodeToString(wantDigest[:]) || asset.MediaType != "text/plain" {
		t.Fatalf("authoritative Asset = %#v", asset)
	}
	if asset.OriginalFilename != "misleading.png" || asset.StorageObjectID != testStoredObjectID {
		t.Fatalf("identity or filename changed: %#v", asset)
	}
	if got := storage.objects[testStoredObjectID]; !bytes.Equal(got, content) {
		t.Fatalf("stored bytes = %q", got)
	}
}

func TestIngestionEnforcesActualStreamLimit(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		wantErr error
	}{
		{name: "exact limit", content: "12345"},
		{name: "over limit", content: "123456", wantErr: assets.ErrAssetTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := newIngestionRepository()
			storage := newMemoryStorage()
			service := newIngestionService(t, repository, storage, 5)
			asset, err := service.Ingest(context.Background(), ingestionInput("file.bin", strings.NewReader(test.content)))
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && asset.ByteSize != 5 {
				t.Fatalf("exact-limit size = %d", asset.ByteSize)
			}
			if test.wantErr != nil && (len(storage.objects) != 0 || repository.asset != nil) {
				t.Fatalf("over-limit attempt retained state: objects=%d asset=%#v", len(storage.objects), repository.asset)
			}
		})
	}
}

func TestIngestionFailuresNeverProduceAvailableMetadata(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*ingestionRepository, *memoryStorage) io.Reader
		wantErr   error
	}{
		{
			name: "reader failure",
			configure: func(_ *ingestionRepository, _ *memoryStorage) io.Reader {
				return io.MultiReader(strings.NewReader(strings.Repeat("x", 512)), errorReader{})
			},
			wantErr: assets.ErrBinaryStorage,
		},
		{
			name: "storage failure",
			configure: func(_ *ingestionRepository, storage *memoryStorage) io.Reader {
				storage.putErr = assets.ErrBinaryStorage
				return strings.NewReader("content")
			},
			wantErr: assets.ErrBinaryStorage,
		},
		{
			name: "cancelled",
			configure: func(_ *ingestionRepository, _ *memoryStorage) io.Reader {
				return cancelReader{}
			},
			wantErr: context.Canceled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newIngestionRepository()
			storage := newMemoryStorage()
			service := newIngestionService(t, repository, storage, 1024)
			_, err := service.Ingest(context.Background(), ingestionInput("file.bin", test.configure(repository, storage)))
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if repository.asset != nil && repository.asset.Lifecycle == assets.LifecycleAvailable {
				t.Fatalf("failed ingestion became AVAILABLE: %#v", repository.asset)
			}
			if len(storage.objects) != 0 {
				t.Fatalf("failed ingestion retained %d objects", len(storage.objects))
			}
		})
	}
}

func TestPersistenceFailureDiscardsBlobAndPendingMetadata(t *testing.T) {
	repository := newIngestionRepository()
	repository.markErr = errors.New("database failure")
	storage := newMemoryStorage()
	service := newIngestionService(t, repository, storage, 1024)

	_, err := service.Ingest(context.Background(), ingestionInput("file.txt", strings.NewReader("content")))
	if err == nil || len(storage.objects) != 0 || repository.asset != nil || storage.discards != 1 {
		t.Fatalf("rollback result: err=%v objects=%d asset=%#v discards=%d", err, len(storage.objects), repository.asset, storage.discards)
	}
}

func TestCleanupFailurePreservesPrimaryFailureAndSignalsIncompleteRollback(t *testing.T) {
	primary := errors.New("database failure")
	repository := newIngestionRepository()
	repository.markErr = primary
	repository.discardErr = errors.New("metadata cleanup failure")
	storage := newMemoryStorage()
	storage.discardErr = errors.New("blob cleanup failure")
	service := newIngestionService(t, repository, storage, 1024)

	_, err := service.Ingest(context.Background(), ingestionInput("file.txt", strings.NewReader("content")))
	if !errors.Is(err, primary) || !errors.Is(err, assets.ErrRollbackIncomplete) {
		t.Fatalf("cleanup error = %v", err)
	}
}

func TestPendingCleanupFailureDoesNotDeletePossiblyReferencedBlob(t *testing.T) {
	repository := newIngestionRepository()
	repository.markErr = errors.New("database failure")
	repository.discardErr = errors.New("pending row changed concurrently")
	storage := newMemoryStorage()
	service := newIngestionService(t, repository, storage, 1024)

	_, err := service.Ingest(context.Background(), ingestionInput("file.txt", strings.NewReader("content")))
	if !errors.Is(err, assets.ErrRollbackIncomplete) || storage.discards != 0 || len(storage.objects) != 1 {
		t.Fatalf("blob was deleted without first claiming PENDING metadata: err=%v discards=%d objects=%d", err, storage.discards, len(storage.objects))
	}
}

func TestAmbiguousMarkResultConvergesWithoutDeletingAvailableBlob(t *testing.T) {
	repository := newIngestionRepository()
	repository.markErr = errors.New("connection lost after commit")
	repository.commitBeforeMarkError = true
	storage := newMemoryStorage()
	service := newIngestionService(t, repository, storage, 1024)

	asset, err := service.Ingest(context.Background(), ingestionInput("file.txt", strings.NewReader("content")))
	if err != nil || asset.Lifecycle != assets.LifecycleAvailable || storage.discards != 0 || len(storage.objects) != 1 {
		t.Fatalf("ambiguous commit did not converge: asset=%#v err=%v discards=%d", asset, err, storage.discards)
	}
}

func TestUnknownMarkOutcomeRetainsBlobRatherThanBreakingPossibleAvailableAsset(t *testing.T) {
	repository := newIngestionRepository()
	repository.markErr = errors.New("database failure")
	repository.getErr = errors.New("database unavailable")
	storage := newMemoryStorage()
	service := newIngestionService(t, repository, storage, 1024)

	_, err := service.Ingest(context.Background(), ingestionInput("file.txt", strings.NewReader("content")))
	if !errors.Is(err, assets.ErrRollbackIncomplete) || storage.discards != 0 || len(storage.objects) != 1 {
		t.Fatalf("uncertain result was destructively cleaned: err=%v discards=%d objects=%d", err, storage.discards, len(storage.objects))
	}
}

func ingestionInput(filename string, content io.Reader) assets.IngestionInput {
	return assets.IngestionInput{OwnerDraftID: draftID, CreatedByUserID: creatorID, OriginalFilename: filename, Content: content}
}

func newIngestionService(t *testing.T, repository assets.Repository, storage assets.BinaryStorage, max int64) *assets.IngestionService {
	t.Helper()
	service, err := assets.NewIngestionService(repository, storage, max)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

const testStoredObjectID = assets.StorageObjectID("55555555-5555-4555-8555-555555555555")

type ingestionRepository struct {
	asset                 *assets.Asset
	markErr               error
	getErr                error
	discardErr            error
	commitBeforeMarkError bool
}

func newIngestionRepository() *ingestionRepository { return &ingestionRepository{} }

func (r *ingestionRepository) CreateAsset(_ context.Context, input assets.AssetInput) (assets.Asset, error) {
	asset := assets.Asset{
		ID: assetID, OwnerDraftID: input.OwnerDraftID, OriginalFilename: input.OriginalFilename, MediaType: input.MediaType,
		Lifecycle: assets.LifecyclePending, CreatedByUserID: input.CreatedByUserID, CreatedAt: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC),
	}
	r.asset = &asset
	return asset, nil
}

func (r *ingestionRepository) GetAsset(_ context.Context, _ assets.AssetID) (assets.Asset, error) {
	if r.getErr != nil {
		return assets.Asset{}, r.getErr
	}
	if r.asset == nil {
		return assets.Asset{}, assets.ErrNotFound
	}
	return *r.asset, nil
}

func (r *ingestionRepository) MarkAssetAvailable(_ context.Context, _ assets.AssetID, stored assets.StoredBinary) (assets.Asset, error) {
	if r.asset == nil {
		return assets.Asset{}, assets.ErrNotFound
	}
	available, transitionErr := r.asset.MarkAvailable(stored)
	if transitionErr != nil {
		return assets.Asset{}, transitionErr
	}
	if r.markErr == nil || r.commitBeforeMarkError {
		r.asset = &available
	}
	if r.markErr != nil {
		return assets.Asset{}, r.markErr
	}
	return available, nil
}

func (r *ingestionRepository) DiscardPendingAsset(_ context.Context, _ assets.AssetID) error {
	if r.discardErr != nil {
		return r.discardErr
	}
	if r.asset == nil || r.asset.Lifecycle != assets.LifecyclePending {
		return assets.ErrInvalidLifecycleTransition
	}
	r.asset = nil
	return nil
}

type memoryStorage struct {
	objects    map[assets.StorageObjectID][]byte
	putErr     error
	discardErr error
	discards   int
}

func newMemoryStorage() *memoryStorage {
	return &memoryStorage{objects: make(map[assets.StorageObjectID][]byte)}
}

func (s *memoryStorage) Put(_ context.Context, reader io.Reader) (assets.StoredBinary, error) {
	if s.putErr != nil {
		return assets.StoredBinary{}, s.putErr
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return assets.StoredBinary{}, err
		}
		return assets.StoredBinary{}, assets.ErrBinaryStorage
	}
	digest := sha256.Sum256(content)
	s.objects[testStoredObjectID] = append([]byte(nil), content...)
	return assets.StoredBinary{StorageObjectID: testStoredObjectID, ByteSize: int64(len(content)), SHA256Digest: assets.SHA256Digest(hex.EncodeToString(digest[:]))}, nil
}

func (s *memoryStorage) Open(_ context.Context, id assets.StorageObjectID) (io.ReadCloser, error) {
	content, ok := s.objects[id]
	if !ok {
		return nil, assets.ErrStorageObjectMissing
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

func (s *memoryStorage) DiscardUncommitted(_ context.Context, id assets.StorageObjectID) error {
	s.discards++
	if s.discardErr != nil {
		return s.discardErr
	}
	delete(s.objects, id)
	return nil
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("source failed") }

type cancelReader struct{}

func (cancelReader) Read([]byte) (int, error) { return 0, context.Canceled }
