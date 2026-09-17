package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
)

const assetUploadDraftID = authoring.DraftID("11111111-1111-4111-8111-111111111111")

type assetUploadHTTPIngestor struct {
	input   assets.IngestionInput
	content []byte
	err     error
	max     int64
	calls   int
}

func (f *assetUploadHTTPIngestor) Ingest(_ context.Context, input assets.IngestionInput) (assets.Asset, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return assets.Asset{}, f.err
	}
	content, err := io.ReadAll(io.LimitReader(input.Content, f.max+1))
	if err != nil {
		return assets.Asset{}, err
	}
	if int64(len(content)) > f.max {
		return assets.Asset{}, assets.ErrAssetTooLarge
	}
	f.content = content
	detected, _, err := mime.ParseMediaType(http.DetectContentType(content))
	if err != nil {
		return assets.Asset{}, assets.ErrInvalidAssetContent
	}
	digest := sha256.Sum256(content)
	return assets.Asset{
		ID: "55555555-5555-4555-8555-555555555555", OwnerDraftID: input.OwnerDraftID,
		OriginalFilename: input.OriginalFilename, MediaType: detected, ByteSize: int64(len(content)),
		SHA256Digest: assets.SHA256Digest(hex.EncodeToString(digest[:])), StorageObjectID: "66666666-6666-4666-8666-666666666666",
		Lifecycle: assets.LifecycleAvailable, CreatedByUserID: "77777777-7777-4777-8777-777777777777",
		CreatedAt: time.Date(2026, 9, 17, 14, 0, 0, 0, time.UTC),
	}, nil
}

func assetUploadHTTPRouter(t *testing.T, roles map[authoring.DraftID]authoring.MemberRole, ingestor *assetUploadHTTPIngestor, max int64) http.Handler {
	t.Helper()
	service := authoring.NewAssetUploadService(ingestor, authoring.NewAuthorizationService(&authoringMembershipsFake{roles: roles}))
	return authTestRouter(&authHTTP{
		sessions: &authResolverFake{current: loginTestCurrent(t)}, authoringAssetUploads: service, assetMaxBytes: max,
	})
}

func TestAuthoringAssetUploadSuccessUsesTrustedScopeAndSafeProjection(t *testing.T) {
	ingestor := &assetUploadHTTPIngestor{max: 1024}
	router := assetUploadHTTPRouter(t, map[authoring.DraftID]authoring.MemberRole{assetUploadDraftID: authoring.MemberAuthor}, ingestor, 1024)
	request := assetMultipartRequest(t, "/api/authoring/drafts/"+string(assetUploadDraftID)+"/assets", []multipartTestPart{{name: "file", filename: "notes.png", contentType: "image/png", content: []byte("actual plain text")}})
	request.ContentLength = 1 // A false client length cannot bypass stream limits.
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("upload response=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	for _, expected := range []string{
		`"assetKey":"55555555-5555-4555-8555-555555555555"`, `"filename":"notes.png"`,
		`"mediaType":"text/plain"`, `"byteSize":17`, `"status":"AVAILABLE"`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("safe projection omitted %s: %s", expected, response.Body.String())
		}
	}
	for _, private := range []string{string(loginTestUser), string(assetUploadDraftID), "66666666-6666-4666-8666-666666666666", "sha256", "storage", "/tmp/"} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatalf("response exposed private value %q: %s", private, response.Body.String())
		}
	}
	if ingestor.calls != 1 || ingestor.input.OwnerDraftID != string(assetUploadDraftID) || ingestor.input.CreatedByUserID != string(loginTestUser) || ingestor.input.OriginalFilename != "notes.png" || string(ingestor.content) != "actual plain text" {
		t.Fatalf("trusted ingestion input=%#v content=%q calls=%d", ingestor.input, ingestor.content, ingestor.calls)
	}
}

func TestAuthoringAssetUploadAuthenticationAuthorizationAndCSRF(t *testing.T) {
	path := "/api/authoring/drafts/" + string(assetUploadDraftID) + "/assets"
	for _, test := range []struct {
		name       string
		roles      map[authoring.DraftID]authoring.MemberRole
		configure  func(*http.Request)
		wantStatus int
	}{
		{name: "unauthenticated", roles: map[authoring.DraftID]authoring.MemberRole{assetUploadDraftID: authoring.MemberAuthor}, configure: func(r *http.Request) { removeCookie(r) }, wantStatus: http.StatusUnauthorized},
		{name: "missing csrf", roles: map[authoring.DraftID]authoring.MemberRole{assetUploadDraftID: authoring.MemberAuthor}, configure: func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, wantStatus: http.StatusForbidden},
		{name: "untrusted origin", roles: map[authoring.DraftID]authoring.MemberRole{assetUploadDraftID: authoring.MemberAuthor}, configure: func(r *http.Request) { r.Header.Set("Origin", "https://attacker.example") }, wantStatus: http.StatusForbidden},
		{name: "other Draft membership", roles: map[authoring.DraftID]authoring.MemberRole{"22222222-2222-4222-8222-222222222222": authoring.MemberMaintainer}, wantStatus: http.StatusNotFound},
		{name: "revoked membership", roles: map[authoring.DraftID]authoring.MemberRole{}, wantStatus: http.StatusNotFound},
		{name: "administrator without Draft membership", roles: map[authoring.DraftID]authoring.MemberRole{}, wantStatus: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			ingestor := &assetUploadHTTPIngestor{max: 1024}
			router := assetUploadHTTPRouter(t, test.roles, ingestor, 1024)
			request := assetMultipartRequest(t, path, []multipartTestPart{{name: "file", filename: "notes.txt", content: []byte("content")}})
			if test.configure != nil {
				test.configure(request)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus || ingestor.calls != 0 {
				t.Fatalf("response=%d calls=%d body=%s", response.Code, ingestor.calls, response.Body.String())
			}
			if response.Code == http.StatusNotFound && (strings.Contains(response.Body.String(), "membership") || strings.Contains(response.Body.String(), "capability")) {
				t.Fatalf("hidden denial leaked details: %s", response.Body.String())
			}
		})
	}
}

func TestAuthoringAssetUploadStrictMultipartAndLimits(t *testing.T) {
	path := "/api/authoring/drafts/" + string(assetUploadDraftID) + "/assets"
	newRouter := func(max int64) (*assetUploadHTTPIngestor, http.Handler) {
		ingestor := &assetUploadHTTPIngestor{max: max}
		return ingestor, assetUploadHTTPRouter(t, map[authoring.DraftID]authoring.MemberRole{assetUploadDraftID: authoring.MemberMaintainer}, ingestor, max)
	}

	for _, test := range []struct {
		name  string
		parts []multipartTestPart
	}{
		{name: "missing file", parts: []multipartTestPart{{name: "description", content: []byte("not a file")}}},
		{name: "multiple files", parts: []multipartTestPart{{name: "file", filename: "one.txt", content: []byte("one")}, {name: "file", filename: "two.txt", content: []byte("two")}}},
		{name: "creator override", parts: []multipartTestPart{{name: "file", filename: "one.txt", content: []byte("one")}, {name: "creatorId", content: []byte("attacker")}}},
		{name: "Draft override", parts: []multipartTestPart{{name: "file", filename: "one.txt", content: []byte("one")}, {name: "draftId", content: []byte("22222222-2222-4222-8222-222222222222")}}},
		{name: "invalid filename", parts: []multipartTestPart{{name: "file", filename: "../secret.txt", content: []byte("one")}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ingestor, router := newRouter(1024)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, assetMultipartRequest(t, path, test.parts))
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_asset_upload"`) {
				t.Fatalf("response=%d body=%s", response.Code, response.Body.String())
			}
			if test.name == "invalid filename" || test.name == "missing file" {
				if ingestor.calls != 0 {
					t.Fatalf("invalid metadata reached ingestion: %d", ingestor.calls)
				}
			}
		})
	}

	ingestor, router := newRouter(8)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, assetMultipartRequest(t, path, []multipartTestPart{{name: "file", filename: "exact.txt", content: []byte("12345678")}}))
	if response.Code != http.StatusCreated {
		t.Fatalf("exact-limit response=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	overLimit := assetMultipartRequest(t, path, []multipartTestPart{{name: "file", filename: "large.txt", content: []byte("123456789")}})
	overLimit.ContentLength = 1 // A false declared length cannot bypass the stream limit.
	router.ServeHTTP(response, overLimit)
	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), `"code":"asset_too_large"`) || ingestor.calls != 2 {
		t.Fatalf("over-limit response=%d calls=%d body=%s", response.Code, ingestor.calls, response.Body.String())
	}

	malformedIngestor, malformedRouter := newRouter(1024)
	malformed := httptest.NewRequest(http.MethodPost, path, strings.NewReader("not multipart"))
	malformed.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	malformed.Header.Set("Origin", "https://academy.example.com")
	malformed.Header.Set("X-CSRF-Token", authTestCSRFToken().Value())
	malformed.AddCookie(&http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()})
	response = httptest.NewRecorder()
	malformedRouter.ServeHTTP(response, malformed)
	if response.Code != http.StatusBadRequest || malformedIngestor.calls != 0 {
		t.Fatalf("malformed response=%d calls=%d", response.Code, malformedIngestor.calls)
	}
}

func TestAuthoringAssetUploadOperationalFailuresAreSanitized(t *testing.T) {
	path := "/api/authoring/drafts/" + string(assetUploadDraftID) + "/assets"
	for _, failure := range []error{assets.ErrBinaryStorage, errors.New("SQLSTATE 08006 /private/storage/path"), errors.Join(errors.New("metadata failed"), assets.ErrRollbackIncomplete)} {
		ingestor := &assetUploadHTTPIngestor{max: 1024, err: failure}
		router := assetUploadHTTPRouter(t, map[authoring.DraftID]authoring.MemberRole{assetUploadDraftID: authoring.MemberAuthor}, ingestor, 1024)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, assetMultipartRequest(t, path, []multipartTestPart{{name: "file", filename: "notes.txt", content: []byte("content")}}))
		if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "SQLSTATE") || strings.Contains(response.Body.String(), "/private/") || strings.Contains(response.Body.String(), "rollback") {
			t.Fatalf("failure %v response=%d body=%s", failure, response.Code, response.Body.String())
		}
	}
}

type multipartTestPart struct {
	name        string
	filename    string
	contentType string
	content     []byte
}

func assetMultipartRequest(t *testing.T, path string, parts []multipartTestPart) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, part := range parts {
		var target io.Writer
		var err error
		if part.filename == "" {
			target, err = writer.CreateFormField(part.name)
		} else {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", `form-data; name="`+part.name+`"; filename="`+strings.ReplaceAll(part.filename, `"`, `\"`)+`"`)
			if part.contentType != "" {
				header.Set("Content-Type", part.contentType)
			}
			target, err = writer.CreatePart(header)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := target.Write(part.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", "https://academy.example.com")
	request.Header.Set("X-CSRF-Token", authTestCSRFToken().Value())
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()})
	return request
}

func removeCookie(request *http.Request) { request.Header.Del("Cookie") }
