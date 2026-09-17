package authoring

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type publicationAssetRepositoryFake struct {
	items map[assets.AssetID]assets.Asset
	err   error
}

func (r *publicationAssetRepositoryFake) CreateAsset(context.Context, assets.AssetInput) (assets.Asset, error) {
	return assets.Asset{}, errors.New("unexpected create")
}
func (r *publicationAssetRepositoryFake) GetAsset(_ context.Context, id assets.AssetID) (assets.Asset, error) {
	if r.err != nil {
		return assets.Asset{}, r.err
	}
	asset, ok := r.items[id]
	if !ok {
		return assets.Asset{}, assets.ErrNotFound
	}
	return asset, nil
}
func (r *publicationAssetRepositoryFake) MarkAssetAvailable(context.Context, assets.AssetID, assets.StoredBinary) (assets.Asset, error) {
	return assets.Asset{}, errors.New("unexpected update")
}
func (r *publicationAssetRepositoryFake) DiscardPendingAsset(context.Context, assets.AssetID) error {
	return errors.New("unexpected discard")
}

func availablePublicationAsset(id, draft, object, mediaType, filename string) assets.Asset {
	return assets.Asset{
		ID: assets.AssetID(id), OwnerDraftID: draft, OriginalFilename: filename,
		MediaType: mediaType, ByteSize: 42,
		SHA256Digest:    assets.SHA256Digest("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		StorageObjectID: assets.StorageObjectID(object), Lifecycle: assets.LifecycleAvailable,
		CreatedByUserID: "77777777-7777-4777-8777-777777777777", CreatedAt: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC),
	}
}

func snapshotWithPublicationBlocks(t *testing.T, blocks []courses.Block) ReviewSnapshot {
	t.Helper()
	_, snapshot := publicationValidationFixture(t)
	snapshot.Modules[0].Lessons[0].Content = courses.LessonContent{SchemaVersion: 1, Blocks: blocks}
	return snapshot
}

func TestAssetPublicationResolverFreezesCompatibleAssetsAndDeduplicatesBindings(t *testing.T) {
	imageID := "11111111-1111-4111-8111-111111111111"
	videoID := "22222222-2222-4222-8222-222222222222"
	audioID := "33333333-3333-4333-8333-333333333333"
	downloadID := "44444444-4444-4444-8444-444444444444"
	captionID := "55555555-5555-4555-8555-555555555555"
	snapshot := snapshotWithPublicationBlocks(t, []courses.Block{
		{Key: "image", Type: courses.BlockImage, Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: imageID}, AltText: "Diagram"}},
		{Key: "video", Type: courses.BlockVideo, Payload: courses.VideoBlockPayload{Asset: courses.AssetReference{AssetKey: videoID}, Title: "Video", Transcript: "Transcript", CaptionsAsset: courses.AssetReference{AssetKey: captionID}}},
		{Key: "audio", Type: courses.BlockAudio, Payload: courses.AudioBlockPayload{Asset: courses.AssetReference{AssetKey: audioID}, Title: "Audio", Transcript: "Transcript"}},
		{Key: "download", Type: courses.BlockDownload, Payload: courses.DownloadBlockPayload{Asset: courses.AssetReference{AssetKey: downloadID}, Label: "Notes"}},
		{Key: "image-again", Type: courses.BlockImage, Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: imageID}, AltText: "Diagram again"}},
	})
	ids := []string{imageID, videoID, audioID, downloadID, captionID}
	media := []string{"image/png", "video/mp4", "audio/mpeg", "application/octet-stream", "text/vtt"}
	repository := &publicationAssetRepositoryFake{items: map[assets.AssetID]assets.Asset{}}
	for index, id := range ids {
		object := []string{"a0000000-0000-4000-8000-000000000001", "b0000000-0000-4000-8000-000000000001", "c0000000-0000-4000-8000-000000000001", "d0000000-0000-4000-8000-000000000001", "e0000000-0000-4000-8000-000000000001"}[index]
		repository.items[assets.AssetID(id)] = availablePublicationAsset(id, string(snapshot.Draft.ID), object, media[index], "asset.bin")
	}
	resolution, err := NewAssetPublicationResolver(repository).Resolve(context.Background(), snapshot.Draft.ID, snapshot)
	if err != nil || len(resolution.Issues) != 0 || len(resolution.Bindings) != 5 {
		t.Fatalf("resolution = %#v, %v", resolution, err)
	}
	if resolution.Bindings[0].AssetKey != imageID || resolution.Bindings[0].StorageObjectID != "a0000000-0000-4000-8000-000000000001" || resolution.Bindings[0].SHA256Digest == "" {
		t.Fatalf("immutable metadata was not copied exactly: %#v", resolution.Bindings[0])
	}
	if reflect.ValueOf(resolution.Bindings[0]).FieldByName("OwnerDraftID").IsValid() || reflect.ValueOf(resolution.Bindings[0]).FieldByName("CreatedByUserID").IsValid() {
		t.Fatal("private ownership or creator provenance entered the Courses binding")
	}
}

func TestAssetPublicationResolverBlocksUnavailableAndIncompatibleReferencesSafely(t *testing.T) {
	validID := "11111111-1111-4111-8111-111111111111"
	foreignID := "22222222-2222-4222-8222-222222222222"
	pendingID := "33333333-3333-4333-8333-333333333333"
	snapshot := snapshotWithPublicationBlocks(t, []courses.Block{
		{Key: "malformed", Type: courses.BlockDownload, Payload: courses.DownloadBlockPayload{Asset: courses.AssetReference{AssetKey: "legacy-key"}, Label: "Legacy"}},
		{Key: "missing", Type: courses.BlockDownload, Payload: courses.DownloadBlockPayload{Asset: courses.AssetReference{AssetKey: "44444444-4444-4444-8444-444444444444"}, Label: "Missing"}},
		{Key: "foreign", Type: courses.BlockDownload, Payload: courses.DownloadBlockPayload{Asset: courses.AssetReference{AssetKey: foreignID}, Label: "Foreign"}},
		{Key: "pending", Type: courses.BlockDownload, Payload: courses.DownloadBlockPayload{Asset: courses.AssetReference{AssetKey: pendingID}, Label: "Pending"}},
		{Key: "mismatch", Type: courses.BlockImage, Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: validID}, AltText: "Wrong type"}},
		{Key: "check", Type: courses.BlockKnowledgeCheck, Payload: courses.KnowledgeCheckBlockPayload{AssessmentKey: "still-unresolved"}},
	})
	repository := &publicationAssetRepositoryFake{items: map[assets.AssetID]assets.Asset{
		assets.AssetID(validID):   availablePublicationAsset(validID, string(snapshot.Draft.ID), "a0000000-0000-4000-8000-000000000001", "text/plain", "wrong.txt"),
		assets.AssetID(foreignID): availablePublicationAsset(foreignID, "99999999-9999-4999-8999-999999999999", "b0000000-0000-4000-8000-000000000001", "application/pdf", "foreign.pdf"),
		assets.AssetID(pendingID): {ID: assets.AssetID(pendingID), OwnerDraftID: string(snapshot.Draft.ID), OriginalFilename: "pending.bin", MediaType: "application/octet-stream", Lifecycle: assets.LifecyclePending, CreatedByUserID: "77777777-7777-4777-8777-777777777777", CreatedAt: time.Now()},
	}}
	resolution, err := NewAssetPublicationResolver(repository).Resolve(context.Background(), snapshot.Draft.ID, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	want := []PublicationValidationCode{PublicationIssueAssetUnavailable, PublicationIssueAssetUnavailable, PublicationIssueAssetUnavailable, PublicationIssueAssetUnavailable, PublicationIssueAssetIncompatible}
	got := make([]PublicationValidationCode, 0, len(resolution.Issues))
	for _, issue := range resolution.Issues {
		got = append(got, issue.Code)
		if issue.Message == "" || issue.Path == "" {
			t.Fatalf("unsafe or incomplete issue: %#v", issue)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("issue codes = %#v, want %#v", got, want)
	}
	cycle, _ := publicationValidationFixture(t)
	validation := NewPublicationValidator().ValidateResolved(cycle, &snapshot, resolution.Bindings)
	if !hasPublicationIssue(validation, PublicationIssueAssessmentUnresolved) {
		t.Fatal("KNOWLEDGE_CHECK was accidentally resolved with assets")
	}
}
