package platform

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
)

const (
	draftAssetDeliveryDraftID = authoring.DraftID("11111111-1111-4111-8111-111111111111")
	draftAssetDeliveryOtherID = authoring.DraftID("22222222-2222-4222-8222-222222222222")
	draftAssetDeliveryAssetID = assets.AssetID("55555555-5555-4555-8555-555555555555")
)

type draftAssetDeliveryRepository struct {
	asset assets.Asset
	err   error
}

func (r *draftAssetDeliveryRepository) GetAsset(context.Context, assets.AssetID) (assets.Asset, error) {
	return r.asset, r.err
}

type seekReadCloser struct{ *bytes.Reader }

func (seekReadCloser) Close() error { return nil }

type draftAssetDeliveryStorage struct {
	data   []byte
	openID assets.StorageObjectID
	err    error
}

func (s *draftAssetDeliveryStorage) Put(context.Context, io.Reader) (assets.StoredBinary, error) {
	return assets.StoredBinary{}, errors.New("not implemented")
}
func (s *draftAssetDeliveryStorage) Open(context.Context, assets.StorageObjectID) (io.ReadCloser, error) {
	return nil, errors.New("non-seekable open must not be used")
}
func (s *draftAssetDeliveryStorage) OpenSeekable(_ context.Context, id assets.StorageObjectID) (assets.ReadSeekCloser, error) {
	s.openID = id
	if s.err != nil {
		return nil, s.err
	}
	return seekReadCloser{bytes.NewReader(s.data)}, nil
}
func (s *draftAssetDeliveryStorage) DiscardUncommitted(context.Context, assets.StorageObjectID) error {
	return errors.New("not implemented")
}

func availableDraftAsset(owner authoring.DraftID, mediaType string, size int64) assets.Asset {
	return assets.Asset{
		ID: draftAssetDeliveryAssetID, Origin: assets.OriginAuthoringDraft, OwnerDraftID: string(owner),
		OriginalFilename: "lesson-media.mp4", MediaType: mediaType, ByteSize: size,
		SHA256Digest:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		StorageObjectID: "66666666-6666-4666-8666-666666666666", Lifecycle: assets.LifecycleAvailable,
		CreatedByUserID: "77777777-7777-4777-8777-777777777777", CreatedAt: time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
	}
}

func draftAssetDeliveryRouter(t *testing.T, roles map[authoring.DraftID]authoring.MemberRole, asset assets.Asset, repositoryErr error, storage *draftAssetDeliveryStorage) http.Handler {
	t.Helper()
	repository := &draftAssetDeliveryRepository{asset: asset, err: repositoryErr}
	service := authoring.NewDraftAssetReadService(repository, authoring.NewAuthorizationService(&authoringMembershipsFake{roles: roles}))
	return authTestRouter(&authHTTP{
		sessions: &authResolverFake{current: loginTestCurrent(t)}, authoringAssetDelivery: service, assetStorage: storage,
	})
}

func draftAssetGET(t *testing.T, draftID authoring.DraftID, assetID string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/authoring/drafts/"+string(draftID)+"/assets/"+assetID+"/content", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()})
	return request
}

func TestAuthoringDraftAssetStreamsPrivateBytesAndSupportsHead(t *testing.T) {
	data := []byte("0123456789abcdef")
	storage := &draftAssetDeliveryStorage{data: data}
	router := draftAssetDeliveryRouter(t, map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryDraftID: authoring.MemberAuthor}, availableDraftAsset(draftAssetDeliveryDraftID, "video/mp4", int64(len(data))), nil, storage)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, draftAssetGET(t, draftAssetDeliveryDraftID, string(draftAssetDeliveryAssetID)))
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), data) {
		t.Fatalf("full response=%d body=%q", response.Code, response.Body.Bytes())
	}
	if response.Header().Get("Content-Type") != "video/mp4" || response.Header().Get("Content-Length") != "16" || response.Header().Get("Accept-Ranges") != "bytes" || response.Header().Get("Cache-Control") != draftAssetCacheControl || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unsafe Draft asset headers: %#v", response.Header())
	}
	if response.Header().Get("ETag") != `"sha256-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"` || !strings.HasPrefix(response.Header().Get("Content-Disposition"), "inline") {
		t.Fatalf("validator/disposition headers: %#v", response.Header())
	}
	if storage.openID != "66666666-6666-4666-8666-666666666666" || strings.Contains(response.Body.String(), string(storage.openID)) {
		t.Fatalf("storage identity leaked or wrong object opened")
	}

	head := draftAssetGET(t, draftAssetDeliveryDraftID, string(draftAssetDeliveryAssetID))
	head.Method = http.MethodHead
	headResponse := httptest.NewRecorder()
	router.ServeHTTP(headResponse, head)
	if headResponse.Code != http.StatusOK || headResponse.Body.Len() != 0 || headResponse.Header().Get("Content-Length") != "16" {
		t.Fatalf("HEAD=%d headers=%#v body=%q", headResponse.Code, headResponse.Header(), headResponse.Body.String())
	}
}

func TestAuthoringDraftAssetETagAndDownloadDisposition(t *testing.T) {
	data := []byte("download data")
	storage := &draftAssetDeliveryStorage{data: data}
	asset := availableDraftAsset(draftAssetDeliveryDraftID, "application/pdf", int64(len(data)))
	asset.OriginalFilename = "lesson notes.pdf"
	router := draftAssetDeliveryRouter(t, map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryDraftID: authoring.MemberMaintainer}, asset, nil, storage)

	request := draftAssetGET(t, draftAssetDeliveryDraftID, string(draftAssetDeliveryAssetID))
	request.Header.Set("If-None-Match", `"sha256-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotModified || response.Body.Len() != 0 {
		t.Fatalf("matching ETag=%d body=%q", response.Code, response.Body.String())
	}

	request = draftAssetGET(t, draftAssetDeliveryDraftID, string(draftAssetDeliveryAssetID))
	request.Header.Set("If-None-Match", `"sha256-other"`)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Disposition"), "attachment") || response.Body.String() != string(data) {
		t.Fatalf("mismatched ETag response=%d headers=%#v body=%q", response.Code, response.Header(), response.Body.String())
	}
}

func TestAuthoringDraftAssetExplicitDownloadUsesSafeAttachment(t *testing.T) {
	data := []byte("image bytes")
	storage := &draftAssetDeliveryStorage{data: data}
	asset := availableDraftAsset(draftAssetDeliveryDraftID, "image/png", int64(len(data)))
	asset.OriginalFilename = "course diagram.png"
	router := draftAssetDeliveryRouter(t, map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryDraftID: authoring.MemberAuthor}, asset, nil, storage)
	request := draftAssetGET(t, draftAssetDeliveryDraftID, string(draftAssetDeliveryAssetID))
	request.URL.RawQuery = "download=1"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Disposition"), "attachment") || !strings.Contains(response.Header().Get("Content-Disposition"), "course diagram.png") {
		t.Fatalf("download response=%d headers=%#v", response.Code, response.Header())
	}
}

func TestAuthoringDraftAssetByteRanges(t *testing.T) {
	data := []byte("0123456789abcdef")
	router := draftAssetDeliveryRouter(t, map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryDraftID: authoring.MemberAuthor}, availableDraftAsset(draftAssetDeliveryDraftID, "audio/mpeg", int64(len(data))), nil, &draftAssetDeliveryStorage{data: data})
	for _, test := range []struct {
		name, value, body, contentRange string
		status                          int
	}{
		{name: "initial", value: "bytes=0-3", body: "0123", contentRange: "bytes 0-3/16", status: http.StatusPartialContent},
		{name: "middle", value: "bytes=5-9", body: "56789", contentRange: "bytes 5-9/16", status: http.StatusPartialContent},
		{name: "suffix", value: "bytes=-4", body: "cdef", contentRange: "bytes 12-15/16", status: http.StatusPartialContent},
		{name: "unsatisfiable", value: "bytes=30-40", contentRange: "bytes */16", status: http.StatusRequestedRangeNotSatisfiable},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := draftAssetGET(t, draftAssetDeliveryDraftID, string(draftAssetDeliveryAssetID))
			request.Header.Set("Range", test.value)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status || (test.status != http.StatusRequestedRangeNotSatisfiable && response.Body.String() != test.body) || response.Header().Get("Content-Range") != test.contentRange {
				t.Fatalf("range %q = %d headers=%#v body=%q", test.value, response.Code, response.Header(), response.Body.String())
			}
		})
	}
}

func TestAuthoringDraftAssetHidesAuthorizationOwnershipAndAvailability(t *testing.T) {
	available := availableDraftAsset(draftAssetDeliveryDraftID, "image/png", 4)
	for _, test := range []struct {
		name       string
		roles      map[authoring.DraftID]authoring.MemberRole
		asset      assets.Asset
		repoErr    error
		draftID    authoring.DraftID
		assetID    string
		configure  func(*http.Request)
		wantStatus int
	}{
		{name: "wrong Draft path", roles: map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryOtherID: authoring.MemberAuthor}, asset: available, draftID: draftAssetDeliveryOtherID, assetID: string(draftAssetDeliveryAssetID), wantStatus: http.StatusNotFound},
		{name: "inaccessible Draft", roles: map[authoring.DraftID]authoring.MemberRole{}, asset: available, draftID: draftAssetDeliveryDraftID, assetID: string(draftAssetDeliveryAssetID), wantStatus: http.StatusNotFound},
		{name: "unknown asset", roles: map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryDraftID: authoring.MemberAuthor}, asset: available, repoErr: assets.ErrNotFound, draftID: draftAssetDeliveryDraftID, assetID: string(draftAssetDeliveryAssetID), wantStatus: http.StatusNotFound},
		{name: "unavailable asset", roles: map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryDraftID: authoring.MemberAuthor}, asset: func() assets.Asset {
			a := available
			a.Lifecycle = assets.LifecyclePending
			a.ByteSize = 0
			a.SHA256Digest = ""
			a.StorageObjectID = ""
			return a
		}(), draftID: draftAssetDeliveryDraftID, assetID: string(draftAssetDeliveryAssetID), wantStatus: http.StatusNotFound},
		{name: "malformed asset", roles: map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryDraftID: authoring.MemberAuthor}, asset: available, draftID: draftAssetDeliveryDraftID, assetID: "not-an-asset", wantStatus: http.StatusNotFound},
		{name: "no session", roles: map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryDraftID: authoring.MemberAuthor}, asset: available, draftID: draftAssetDeliveryDraftID, assetID: string(draftAssetDeliveryAssetID), configure: removeCookie, wantStatus: http.StatusUnauthorized},
		{name: "runtime bearer only", roles: map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryDraftID: authoring.MemberAuthor}, asset: available, draftID: draftAssetDeliveryDraftID, assetID: string(draftAssetDeliveryAssetID), configure: func(r *http.Request) { removeCookie(r); r.Header.Set("Authorization", "Bearer runtime-token") }, wantStatus: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := draftAssetDeliveryRouter(t, test.roles, test.asset, test.repoErr, &draftAssetDeliveryStorage{data: []byte("data")})
			request := draftAssetGET(t, test.draftID, test.assetID)
			if test.configure != nil {
				test.configure(request)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			storageLeaked := test.asset.StorageObjectID != "" && strings.Contains(response.Body.String(), string(test.asset.StorageObjectID))
			if response.Code != test.wantStatus || storageLeaked || strings.Contains(response.Body.String(), "membership") || strings.Contains(response.Body.String(), "capability") {
				t.Fatalf("response=%d body=%q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAuthoringDraftAssetStorageFailuresAreSanitized(t *testing.T) {
	asset := availableDraftAsset(draftAssetDeliveryDraftID, "image/png", 4)
	for _, failure := range []error{assets.ErrStorageObjectMissing, assets.ErrBinaryStorage, errors.New("/private/storage/object failed")} {
		router := draftAssetDeliveryRouter(t, map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryDraftID: authoring.MemberAuthor}, asset, nil, &draftAssetDeliveryStorage{err: failure})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, draftAssetGET(t, draftAssetDeliveryDraftID, string(draftAssetDeliveryAssetID)))
		want := http.StatusServiceUnavailable
		if errors.Is(failure, assets.ErrStorageObjectMissing) {
			want = http.StatusNotFound
		}
		if response.Code != want || strings.Contains(response.Body.String(), "/private/") || strings.Contains(response.Body.String(), string(asset.StorageObjectID)) {
			t.Fatalf("failure=%v response=%d body=%q", failure, response.Code, response.Body.String())
		}
	}
}

func TestAuthoringDraftAssetRejectsStorageIntegrityMismatch(t *testing.T) {
	asset := availableDraftAsset(draftAssetDeliveryDraftID, "image/png", 999)
	router := draftAssetDeliveryRouter(t, map[authoring.DraftID]authoring.MemberRole{draftAssetDeliveryDraftID: authoring.MemberAuthor}, asset, nil, &draftAssetDeliveryStorage{data: []byte("short")})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, draftAssetGET(t, draftAssetDeliveryDraftID, string(draftAssetDeliveryAssetID)))
	if response.Code != http.StatusServiceUnavailable || response.Body.String() == "short" {
		t.Fatalf("integrity mismatch response=%d body=%q", response.Code, response.Body.String())
	}
}
