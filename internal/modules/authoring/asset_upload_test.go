package authoring

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
)

type assetIngestorFake struct {
	input assets.IngestionInput
	asset assets.Asset
	err   error
	calls int
}

func (f *assetIngestorFake) Ingest(_ context.Context, input assets.IngestionInput) (assets.Asset, error) {
	f.calls++
	f.input = input
	return f.asset, f.err
}

func TestAssetUploadAuthorizationPrecedesIngestionAndCapturesTrustedScope(t *testing.T) {
	actor := resolvedActor(t)
	for _, role := range []MemberRole{MemberAuthor, MemberMaintainer} {
		t.Run(string(role), func(t *testing.T) {
			ingestor := &assetIngestorFake{asset: assets.Asset{ID: "55555555-5555-4555-8555-555555555555"}}
			memberships := &membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: role}}
			service := NewAssetUploadService(ingestor, NewAuthorizationService(memberships))
			upload, err := service.AuthorizeUpload(context.Background(), actor, testDraftA)
			if err != nil || upload == nil || ingestor.calls != 0 || memberships.lastDraft != testDraftA || memberships.lastUser != string(actor.UserID()) {
				t.Fatalf("authorization result: upload=%v err=%v calls=%d scope=%s/%s", upload != nil, err, ingestor.calls, memberships.lastDraft, memberships.lastUser)
			}
			if _, err := upload(context.Background(), "lesson.txt", strings.NewReader("content")); err != nil {
				t.Fatal(err)
			}
			if ingestor.calls != 1 || ingestor.input.OwnerDraftID != string(testDraftA) || ingestor.input.CreatedByUserID != string(actor.UserID()) || ingestor.input.OriginalFilename != "lesson.txt" {
				t.Fatalf("ingestion input = %#v", ingestor.input)
			}
		})
	}
}

func TestAssetUploadAuthorizationIsDraftScopedAndHasNoAdministratorBypass(t *testing.T) {
	actor := resolvedActor(t)
	for _, test := range []struct {
		name  string
		roles map[DraftID]MemberRole
	}{
		{name: "membership on another Draft", roles: map[DraftID]MemberRole{testDraftB: MemberMaintainer}},
		{name: "revoked or absent membership", roles: map[DraftID]MemberRole{}},
		{name: "global administrator without membership", roles: map[DraftID]MemberRole{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ingestor := &assetIngestorFake{}
			service := NewAssetUploadService(ingestor, NewAuthorizationService(&membershipReaderFake{roles: test.roles}))
			if upload, err := service.AuthorizeUpload(context.Background(), actor, testDraftA); !errors.Is(err, ErrNotFound) || upload != nil || ingestor.calls != 0 {
				t.Fatalf("hidden authorization result: upload=%v err=%v calls=%d", upload != nil, err, ingestor.calls)
			}
		})
	}
}

func TestAssetUploadPreservesIngestionFailure(t *testing.T) {
	primary := errors.New("ingestion unavailable")
	ingestor := &assetIngestorFake{err: primary}
	service := NewAssetUploadService(ingestor, NewAuthorizationService(&membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: MemberAuthor}}))
	upload, err := service.AuthorizeUpload(context.Background(), resolvedActor(t), testDraftA)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := upload(context.Background(), "lesson.txt", io.LimitReader(strings.NewReader("content"), 7)); !errors.Is(err, primary) {
		t.Fatalf("ingestion error = %v", err)
	}
}
