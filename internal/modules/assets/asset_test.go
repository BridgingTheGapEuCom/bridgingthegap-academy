package assets_test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
)

const (
	assetID   = assets.AssetID("11111111-1111-4111-8111-111111111111")
	draftID   = "22222222-2222-4222-8222-222222222222"
	creatorID = "33333333-3333-4333-8333-333333333333"
	objectID  = assets.StorageObjectID("44444444-4444-4444-8444-444444444444")
	digest    = assets.SHA256Digest("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
)

func validAsset() assets.Asset {
	return assets.Asset{
		ID: assetID, OwnerDraftID: draftID, OriginalFilename: "architecture diagram.png", MediaType: "image/png",
		ByteSize: 2048, SHA256Digest: digest, StorageObjectID: objectID, Lifecycle: assets.LifecycleAvailable,
		CreatedByUserID: creatorID, CreatedAt: time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC),
	}
}

func TestAssetMetadataValidation(t *testing.T) {
	if parsed, err := assets.ParseAssetID(string(assetID)); err != nil || parsed != assetID {
		t.Fatalf("canonical Asset ID parse = %q, %v", parsed, err)
	}
	if _, err := assets.ParseAssetID("legacy-key"); err == nil {
		t.Fatal("non-canonical Asset ID accepted")
	}
	if err := validAsset().Validate(); err != nil {
		t.Fatalf("valid Asset rejected: %v", err)
	}

	tests := []struct {
		name   string
		change func(*assets.Asset)
	}{
		{"empty Asset ID", func(a *assets.Asset) { a.ID = "" }},
		{"path filename", func(a *assets.Asset) { a.OriginalFilename = "../secret.txt" }},
		{"control filename", func(a *assets.Asset) { a.OriginalFilename = "bad\nname.txt" }},
		{"invalid content type", func(a *assets.Asset) { a.MediaType = "image" }},
		{"content type parameters", func(a *assets.Asset) { a.MediaType = "text/plain; charset=utf-8" }},
		{"invalid byte size", func(a *assets.Asset) { a.ByteSize = 0 }},
		{"invalid digest", func(a *assets.Asset) { a.SHA256Digest = "not-a-sha256" }},
		{"invalid storage object", func(a *assets.Asset) { a.StorageObjectID = "local/path" }},
		{"invalid lifecycle", func(a *assets.Asset) { a.Lifecycle = "DELETED" }},
		{"invalid owner scope", func(a *assets.Asset) { a.OwnerDraftID = "draft" }},
		{"invalid creator", func(a *assets.Asset) { a.CreatedByUserID = "user" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			asset := validAsset()
			test.change(&asset)
			if err := asset.Validate(); err == nil {
				t.Fatal("invalid Asset accepted")
			}
		})
	}
}

func TestPendingToAvailablePreservesIdentityAndProvenance(t *testing.T) {
	pending := validAsset()
	pending.Lifecycle = assets.LifecyclePending
	pending.ByteSize = 0
	pending.SHA256Digest = ""
	pending.StorageObjectID = ""
	if err := pending.Validate(); err != nil {
		t.Fatal(err)
	}
	available, err := pending.MarkAvailable(assets.StoredBinary{StorageObjectID: objectID, ByteSize: 2048, SHA256Digest: digest})
	if err != nil {
		t.Fatal(err)
	}
	if available.ID != pending.ID || available.OwnerDraftID != draftID || available.CreatedByUserID != creatorID || available.Lifecycle != assets.LifecycleAvailable {
		t.Fatalf("identity or internal provenance changed: %#v", available)
	}
	if pending.Lifecycle != assets.LifecyclePending || pending.StorageObjectID != "" {
		t.Fatal("transition mutated the pending Asset")
	}
	if string(available.ID) == string(available.StorageObjectID) {
		t.Fatal("Asset identity and storage identity were conflated")
	}
}

func TestInternalAssetProvenanceIsNotJSONVisible(t *testing.T) {
	values := []any{
		validAsset(),
		assets.AssetInput{OwnerDraftID: draftID, OriginalFilename: "diagram.png", MediaType: "image/png", CreatedByUserID: creatorID},
		assets.IngestionInput{OwnerDraftID: draftID, OriginalFilename: "diagram.png", CreatedByUserID: creatorID, Content: strings.NewReader("binary")},
	}
	for _, source := range values {
		encoded, err := json.Marshal(source)
		if err != nil {
			t.Fatal(err)
		}
		value := string(encoded)
		for _, internal := range []string{draftID, creatorID, string(objectID)} {
			if strings.Contains(value, internal) {
				t.Fatalf("internal provenance leaked through JSON: %s", value)
			}
		}
	}
}

type storageContract struct{}

func (storageContract) Put(context.Context, io.Reader) (assets.StoredBinary, error) {
	return assets.StoredBinary{StorageObjectID: objectID, ByteSize: 2048, SHA256Digest: digest}, nil
}
func (storageContract) Open(context.Context, assets.StorageObjectID) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("binary")), nil
}
func (storageContract) DiscardUncommitted(context.Context, assets.StorageObjectID) error { return nil }

func TestBinaryStorageBoundaryHasProviderNeutralIdentity(t *testing.T) {
	var storage assets.BinaryStorage = storageContract{}
	stored, err := storage.Put(context.Background(), strings.NewReader("binary"))
	if err != nil || stored.Validate() != nil {
		t.Fatalf("storage result invalid: %#v, %v", stored, err)
	}
	if _, err := storage.Open(context.Background(), stored.StorageObjectID); err != nil {
		t.Fatal(err)
	}
}
