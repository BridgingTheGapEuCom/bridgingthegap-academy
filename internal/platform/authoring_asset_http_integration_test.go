//go:build integration

package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	assetslocal "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/localstorage"
	assetspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAuthoringAssetUploadAPI(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	identityRepository := identitypostgres.New(pool)
	creator, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	administrator, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := identityRepository.AssignGlobalRole(ctx, administrator.ID, identity.RoleAdministrator, nil); err != nil {
		t.Fatal(err)
	}

	version, err := courses.ParseVersion("0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	authoringRepository := authoringpostgres.New(pool)
	draft, err := authoringRepository.CreateDraftForCreator(ctx, authoring.DraftCreationInput{
		Title: "Asset upload integration", IntendedVersion: version, SourceLanguage: "en",
		Description:        "A Draft used to verify the authenticated asset upload boundary.",
		LearningObjectives: []string{"Verify asset ingestion"}, Changelog: "Initial Draft.",
	}, string(creator.ID))
	if err != nil {
		t.Fatal(err)
	}

	storageRoot := t.TempDir()
	storage, err := assetslocal.New(storageRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	assetRepository := assetspostgres.New(pool)
	ingestion, err := assets.NewIngestionService(assetRepository, storage, 1024)
	if err != nil {
		t.Fatal(err)
	}
	sessions := identity.NewSessionService(identityRepository, nil, nil)
	creatorSession, err := sessions.CreateSession(ctx, creator.ID)
	if err != nil {
		t.Fatal(err)
	}
	administratorSession, err := sessions.CreateSession(ctx, administrator.ID)
	if err != nil {
		t.Fatal(err)
	}
	authorizer := authoring.NewAuthorizationService(authoringRepository)
	router := authTestRouter(&authHTTP{
		sessions: sessions, authoringAssetUploads: authoring.NewAssetUploadService(ingestion, authorizer), assetMaxBytes: 1024,
	})

	content := []byte("authoritative HTTP upload integration content")
	request := assetMultipartRequest(t, "/api/authoring/drafts/"+string(draft.ID)+"/assets", []multipartTestPart{{
		name: "file", filename: "lesson-image.png", contentType: "image/png", content: content,
	}})
	request.Header.Del("Cookie")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: creatorSession.Token.Value()})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("upload response=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	var projection authoringAssetDTO
	if err := json.Unmarshal(response.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	if projection.AssetKey == "" || projection.Filename != "lesson-image.png" || projection.MediaType != "text/plain" || projection.ByteSize != int64(len(content)) || projection.Status != assets.LifecycleAvailable {
		t.Fatalf("safe projection=%#v", projection)
	}
	for _, private := range []string{string(creator.ID), string(draft.ID), "storageObject", "createdBy", "sha256", storageRoot} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatalf("upload response exposed private value %q: %s", private, response.Body.String())
		}
	}

	stored, err := assetRepository.GetAsset(ctx, assets.AssetID(projection.AssetKey))
	if err != nil {
		t.Fatal(err)
	}
	if stored.Lifecycle != assets.LifecycleAvailable || stored.OwnerDraftID != string(draft.ID) || stored.CreatedByUserID != string(creator.ID) || stored.StorageObjectID == "" {
		t.Fatalf("stored Asset provenance=%#v", stored)
	}
	opened, err := storage.Open(ctx, stored.StorageObjectID)
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := io.ReadAll(opened)
	closeErr := opened.Close()
	if readErr != nil || closeErr != nil || !bytes.Equal(got, content) {
		t.Fatalf("stored bytes=%q read=%v close=%v", got, readErr, closeErr)
	}

	before := countAssetsForDraft(t, ctx, pool, draft.ID)
	denied := assetMultipartRequest(t, "/api/authoring/drafts/"+string(draft.ID)+"/assets", []multipartTestPart{{
		name: "file", filename: "hidden.txt", content: []byte("must not be ingested"),
	}})
	denied.Header.Del("Cookie")
	denied.AddCookie(&http.Cookie{Name: sessionCookieName, Value: administratorSession.Token.Value()})
	deniedResponse := httptest.NewRecorder()
	router.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != http.StatusNotFound || strings.Contains(deniedResponse.Body.String(), "membership") || strings.Contains(deniedResponse.Body.String(), "capability") {
		t.Fatalf("administrator hidden denial=%d body=%s", deniedResponse.Code, deniedResponse.Body.String())
	}
	if after := countAssetsForDraft(t, ctx, pool, draft.ID); after != before {
		t.Fatalf("hidden authorization denial changed Asset count: before=%d after=%d", before, after)
	}
}

func countAssetsForDraft(t *testing.T, ctx context.Context, pool *pgxpool.Pool, draftID authoring.DraftID) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM assets.asset WHERE owner_draft_id = $1", draftID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
