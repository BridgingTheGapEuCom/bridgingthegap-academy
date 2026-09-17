//go:build integration

package platform

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	assetslocal "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/localstorage"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPublishedAssetBinaryDelivery(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	storage, err := assetslocal.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	firstBytes := []byte("first immutable binary")
	secondBytes := []byte("second immutable binary")
	firstStored, err := storage.Put(ctx, bytes.NewReader(firstBytes))
	if err != nil {
		t.Fatal(err)
	}
	secondStored, err := storage.Put(ctx, bytes.NewReader(secondBytes))
	if err != nil {
		t.Fatal(err)
	}

	repository := coursespostgres.New(pool)
	store := courses.NewCourseVersionStore(repository)
	course, err := repository.CreateCourse(ctx, "published-asset-delivery")
	if err != nil {
		t.Fatal(err)
	}
	first := immutableCourseVersionFixture(t, course.ID, "1.0.0", "ea000000-0000-4000-8000-000000000001")
	first.AssetBindings[0] = bindingForStoredBinary(first.AssetBindings[0], firstStored)
	if _, err := store.Store(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := immutableCourseVersionFixture(t, course.ID, "1.1.0", "ea000000-0000-4000-8000-000000000002")
	second.AssetBindings[0] = bindingForStoredBinary(second.AssetBindings[0], secondStored)
	if _, err := store.Store(ctx, second); err != nil {
		t.Fatal(err)
	}

	router := authTestRouter(&authHTTP{
		publishedAssets: courses.NewPublishedAssetReadService(repository),
		assetStorage:    storage,
	})
	path := func(version string) string {
		return "/api/courses/by-id/" + string(course.ID) + "/versions/" + version + "/assets/" + first.AssetBindings[0].AssetKey
	}
	for _, test := range []struct {
		version string
		want    []byte
	}{
		{version: "1.0.0", want: firstBytes},
		{version: "1.1.0", want: secondBytes},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path(test.version), nil))
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), test.want) {
			t.Fatalf("exact version %s delivered %d %q, want %q", test.version, response.Code, response.Body.Bytes(), test.want)
		}
		if response.Header().Get("Content-Type") != "image/png" || response.Header().Get("Content-Length") != strconv.Itoa(len(test.want)) || bytes.Contains(response.Body.Bytes(), []byte(first.AssetBindings[0].StorageObjectID)) {
			t.Fatalf("frozen public representation was incorrect: %#v", response.Header())
		}
	}

	unpublished := "76000000-0000-4000-8000-000000000099"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/courses/by-id/"+string(course.ID)+"/versions/1.0.0/assets/"+unpublished, nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unpublished asset was public: %d %q", response.Code, response.Body.String())
	}
}

func bindingForStoredBinary(binding courses.PublishedAssetBinding, stored assets.StoredBinary) courses.PublishedAssetBinding {
	binding.StorageObjectID = string(stored.StorageObjectID)
	binding.ByteSize = stored.ByteSize
	binding.SHA256Digest = string(stored.SHA256Digest)
	return binding
}
