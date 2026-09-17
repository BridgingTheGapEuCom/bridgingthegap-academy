//go:build integration

package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	assetslocal "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/localstorage"
	assetspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAssetsPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	repository := assetspostgres.New(pool)
	pendingInput := assets.AssetInput{
		OwnerDraftID: "10000000-0000-4000-8000-000000000001", OriginalFilename: "architecture diagram.png",
		MediaType: "image/png", CreatedByUserID: "20000000-0000-4000-8000-000000000002",
	}
	pending, err := repository.CreateAsset(ctx, pendingInput)
	if err != nil {
		t.Fatal(err)
	}
	if pending.ID == "" || pending.Lifecycle != assets.LifecyclePending || pending.StorageObjectID != "" || pending.CreatedAt.IsZero() {
		t.Fatalf("pending Asset did not round trip: %#v", pending)
	}

	stored := assets.StoredBinary{
		StorageObjectID: "30000000-0000-4000-8000-000000000003", ByteSize: 4096,
		SHA256Digest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	available, err := repository.MarkAssetAvailable(ctx, pending.ID, stored)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.GetAsset(ctx, pending.ID)
	if err != nil || loaded != available {
		t.Fatalf("available Asset did not round trip: %#v, %v", loaded, err)
	}
	if loaded.ID == assets.AssetID(loaded.StorageObjectID) || loaded.OwnerDraftID != pendingInput.OwnerDraftID || loaded.CreatedByUserID != pendingInput.CreatedByUserID {
		t.Fatal("Asset identity, storage identity, or private provenance was not preserved")
	}
	if _, err := repository.MarkAssetAvailable(ctx, pending.ID, stored); !errors.Is(err, assets.ErrInvalidLifecycleTransition) {
		t.Fatalf("AVAILABLE Asset accepted a second content attachment: %v", err)
	}
	if err := repository.DiscardPendingAsset(ctx, pending.ID); !errors.Is(err, assets.ErrInvalidLifecycleTransition) {
		t.Fatalf("AVAILABLE Asset was eligible for ingestion rollback: %v", err)
	}

	duplicatePending, err := repository.CreateAsset(ctx, assets.AssetInput{
		OwnerDraftID: "10000000-0000-4000-8000-000000000004", OriginalFilename: "other.png", MediaType: "image/png",
		CreatedByUserID: "20000000-0000-4000-8000-000000000005",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.MarkAssetAvailable(ctx, duplicatePending.ID, stored); !errors.Is(err, assets.ErrConflict) {
		t.Fatalf("duplicate storage identity was accepted: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO assets.asset (
			id, owner_draft_id, original_filename, media_type, lifecycle, created_by_user_id
		)
		SELECT id, owner_draft_id, original_filename, media_type, 'PENDING', created_by_user_id
		FROM assets.asset WHERE id = $1`, pending.ID); err == nil {
		t.Fatal("duplicate Asset identity was accepted")
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO assets.asset (
			owner_draft_id, original_filename, media_type, byte_size, sha256_digest,
			storage_object_id, lifecycle, created_by_user_id
		) VALUES ($1, 'bad.png', 'image/png', 0, 'bad', $2, 'AVAILABLE', $3)`,
		pendingInput.OwnerDraftID, "40000000-0000-4000-8000-000000000006", pendingInput.CreatedByUserID); err == nil {
		t.Fatal("database accepted invalid AVAILABLE integrity metadata")
	}

	t.Run("real local ingestion round trip", func(t *testing.T) {
		storage, err := assetslocal.New(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer storage.Close()
		service, err := assets.NewIngestionService(repository, storage, 1024)
		if err != nil {
			t.Fatal(err)
		}
		content := []byte("authoritative local storage integration content")
		created, err := service.Ingest(ctx, assets.IngestionInput{
			OwnerDraftID: "10000000-0000-4000-8000-000000000007", CreatedByUserID: "20000000-0000-4000-8000-000000000008",
			OriginalFilename: "actually-text.png", Content: bytes.NewReader(content),
		})
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := repository.GetAsset(ctx, created.ID)
		if err != nil || loaded != created {
			t.Fatalf("ingested metadata readback = %#v, %v", loaded, err)
		}
		digest := sha256.Sum256(content)
		if loaded.Lifecycle != assets.LifecycleAvailable || loaded.MediaType != "text/plain" || loaded.ByteSize != int64(len(content)) ||
			string(loaded.SHA256Digest) != hex.EncodeToString(digest[:]) {
			t.Fatalf("ingested metadata = %#v", loaded)
		}
		opened, err := storage.Open(ctx, loaded.StorageObjectID)
		if err != nil {
			t.Fatal(err)
		}
		got, readErr := io.ReadAll(opened)
		closeErr := opened.Close()
		if readErr != nil || closeErr != nil || !bytes.Equal(got, content) {
			t.Fatalf("ingested bytes = %q, read=%v close=%v", got, readErr, closeErr)
		}
	})

	t.Run("real persistence failure cleans local object and pending row", func(t *testing.T) {
		storage, err := assetslocal.New(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer storage.Close()
		failingRepository := &markFailingAssetRepository{Repository: repository}
		recordingStorage := &recordingBinaryStorage{BinaryStorage: storage}
		service, err := assets.NewIngestionService(failingRepository, recordingStorage, 1024)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Ingest(ctx, assets.IngestionInput{
			OwnerDraftID: "10000000-0000-4000-8000-000000000009", CreatedByUserID: "20000000-0000-4000-8000-000000000010",
			OriginalFilename: "rollback.txt", Content: bytes.NewReader([]byte("rollback content")),
		}); err == nil {
			t.Fatal("injected persistence failure succeeded")
		}
		if _, err := repository.GetAsset(ctx, failingRepository.createdID); !errors.Is(err, assets.ErrNotFound) {
			t.Fatalf("failed ingestion retained PENDING metadata: %v", err)
		}
		if _, err := storage.Open(ctx, recordingStorage.stored.StorageObjectID); !errors.Is(err, assets.ErrStorageObjectMissing) {
			t.Fatalf("failed ingestion retained committed binary: %v", err)
		}
	})
}

type markFailingAssetRepository struct {
	assets.Repository
	createdID assets.AssetID
}

func (r *markFailingAssetRepository) CreateAsset(ctx context.Context, input assets.AssetInput) (assets.Asset, error) {
	created, err := r.Repository.CreateAsset(ctx, input)
	if err == nil {
		r.createdID = created.ID
	}
	return created, err
}

func (*markFailingAssetRepository) MarkAssetAvailable(context.Context, assets.AssetID, assets.StoredBinary) (assets.Asset, error) {
	return assets.Asset{}, errors.New("injected persistence failure")
}

type recordingBinaryStorage struct {
	assets.BinaryStorage
	stored assets.StoredBinary
}

func (s *recordingBinaryStorage) Put(ctx context.Context, reader io.Reader) (assets.StoredBinary, error) {
	stored, err := s.BinaryStorage.Put(ctx, reader)
	if err == nil {
		s.stored = stored
	}
	return stored, err
}
