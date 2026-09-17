package platform

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type publishedAssetHTTPRepository struct {
	binding courses.PublishedAssetBinding
	err     error
	course  courses.CourseID
	version courses.Version
	asset   string
}

func (r *publishedAssetHTTPRepository) GetPublishedAssetBindingByCourseAndVersionAndAssetKey(_ context.Context, courseID courses.CourseID, version courses.Version, assetKey string) (courses.PublishedAssetBinding, error) {
	r.course, r.version, r.asset = courseID, version, assetKey
	return r.binding, r.err
}

type publishedAssetHTTPStorage struct {
	data   []byte
	openID assets.StorageObjectID
	err    error
}

func (s *publishedAssetHTTPStorage) Put(context.Context, io.Reader) (assets.StoredBinary, error) {
	return assets.StoredBinary{}, errors.New("not implemented")
}

func (s *publishedAssetHTTPStorage) Open(_ context.Context, id assets.StorageObjectID) (io.ReadCloser, error) {
	s.openID = id
	if s.err != nil {
		return nil, s.err
	}
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

func (s *publishedAssetHTTPStorage) DiscardUncommitted(context.Context, assets.StorageObjectID) error {
	return errors.New("not implemented")
}

func publishedAssetHTTPBinding(mediaType string) courses.PublishedAssetBinding {
	return courses.PublishedAssetBinding{
		AssetKey:         "50000000-0000-4000-8000-000000000001",
		StorageObjectID:  "60000000-0000-4000-8000-000000000001",
		OriginalFilename: "architecture.png",
		MediaType:        mediaType,
		ByteSize:         int64(len("frozen bytes")),
		SHA256Digest:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
}

func publishedAssetHTTPRouter(binding courses.PublishedAssetBinding, storage *publishedAssetHTTPStorage, err error) (http.Handler, *publishedAssetHTTPRepository) {
	repository := &publishedAssetHTTPRepository{binding: binding, err: err}
	auth := &authHTTP{
		publishedAssets: courses.NewPublishedAssetReadService(repository),
		assetStorage:    storage,
	}
	return authTestRouter(auth), repository
}

func TestPublishedAssetHTTPStreamsFrozenBindingWithImmutableCaching(t *testing.T) {
	binding := publishedAssetHTTPBinding("image/png")
	storage := &publishedAssetHTTPStorage{data: []byte("frozen bytes")}
	router, repository := publishedAssetHTTPRouter(binding, storage, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/courses/by-id/10000000-0000-4000-8000-000000000001/versions/1.2.3/assets/"+binding.AssetKey, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "frozen bytes" {
		t.Fatalf("unexpected binary response: status=%d body=%q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "image/png" || response.Header().Get("Content-Length") != "12" || response.Header().Get("Cache-Control") != publishedAssetCacheControl || response.Header().Get("ETag") != `"sha256-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"` {
		t.Fatalf("unexpected immutable representation headers: %#v", response.Header())
	}
	if response.Header().Get("Content-Disposition") == "" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Accept-Ranges") != "" || response.Body.String() == binding.StorageObjectID {
		t.Fatalf("unsafe public binary response: %#v", response.Header())
	}
	if storage.openID != assets.StorageObjectID(binding.StorageObjectID) || repository.course != "10000000-0000-4000-8000-000000000001" || repository.version.String() != "1.2.3" || repository.asset != binding.AssetKey {
		t.Fatalf("binary was not resolved through exact binding: storage=%q repository=%#v", storage.openID, repository)
	}
}

func TestPublishedAssetHTTPETagHeadAndSafeAttachmentDisposition(t *testing.T) {
	binding := publishedAssetHTTPBinding("image/svg+xml")
	storage := &publishedAssetHTTPStorage{data: []byte("frozen bytes")}
	router, _ := publishedAssetHTTPRouter(binding, storage, nil)
	path := "/api/courses/by-id/10000000-0000-4000-8000-000000000001/versions/1.2.3/assets/" + binding.AssetKey

	headRequest := httptest.NewRequest(http.MethodHead, path, nil)
	headResponse := httptest.NewRecorder()
	router.ServeHTTP(headResponse, headRequest)
	if headResponse.Code != http.StatusOK || headResponse.Body.Len() != 0 || headResponse.Header().Get("Content-Disposition")[:10] != "attachment" {
		t.Fatalf("unsafe HEAD representation: status=%d headers=%#v body=%q", headResponse.Code, headResponse.Header(), headResponse.Body.String())
	}

	cacheRequest := httptest.NewRequest(http.MethodGet, path, nil)
	cacheRequest.Header.Set("If-None-Match", `"sha256-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`)
	cacheResponse := httptest.NewRecorder()
	router.ServeHTTP(cacheResponse, cacheRequest)
	if cacheResponse.Code != http.StatusNotModified || cacheResponse.Body.Len() != 0 {
		t.Fatalf("ETag revalidation = %d, body=%q", cacheResponse.Code, cacheResponse.Body.String())
	}
}

func TestPublishedAssetHTTPHidesMissingBindingsAndUnpublishedAssets(t *testing.T) {
	binding := publishedAssetHTTPBinding("image/png")
	storage := &publishedAssetHTTPStorage{data: []byte("frozen bytes")}
	router, _ := publishedAssetHTTPRouter(binding, storage, courses.ErrNotFound)
	for _, path := range []string{
		"/api/courses/by-id/10000000-0000-4000-8000-000000000001/versions/1.2.3/assets/" + binding.AssetKey,
		"/api/courses/by-id/not-a-course/versions/1.2.3/assets/" + binding.AssetKey,
		"/api/courses/by-id/10000000-0000-4000-8000-000000000001/versions/not-a-version/assets/" + binding.AssetKey,
		"/api/courses/by-id/10000000-0000-4000-8000-000000000001/versions/1.2.3/assets/not-an-asset",
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound || bytes.Contains(response.Body.Bytes(), []byte(binding.StorageObjectID)) {
			t.Fatalf("missing public resource leaked or had wrong status for %q: %d %q", path, response.Code, response.Body.String())
		}
	}
}

func TestPublishedAssetHTTPSanitizesStorageFailure(t *testing.T) {
	binding := publishedAssetHTTPBinding("application/octet-stream")
	storage := &publishedAssetHTTPStorage{err: assets.ErrBinaryStorage}
	router, _ := publishedAssetHTTPRouter(binding, storage, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/courses/by-id/10000000-0000-4000-8000-000000000001/versions/1.2.3/assets/"+binding.AssetKey, nil))
	if response.Code != http.StatusServiceUnavailable || bytes.Contains(response.Body.Bytes(), []byte(binding.StorageObjectID)) || bytes.Contains(response.Body.Bytes(), []byte("filesystem")) {
		t.Fatalf("storage failure was not sanitized: %d %q", response.Code, response.Body.String())
	}
}
