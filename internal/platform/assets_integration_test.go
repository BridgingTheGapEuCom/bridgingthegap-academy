//go:build integration

package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
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
}
