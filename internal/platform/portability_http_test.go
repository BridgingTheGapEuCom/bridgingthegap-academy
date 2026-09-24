package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/portability"
)

type portabilityExportFake struct{ err error }

func (f portabilityExportFake) Write(_ context.Context, _ courses.CourseID, _ courses.Version, out io.Writer) error {
	if f.err != nil {
		return f.err
	}
	_, err := out.Write([]byte("PK\x03\x04test"))
	return err
}

type portabilityCourseFake struct {
	value courses.ImmutableCourseVersion
	err   error
}

type portabilityImportFake struct{}

func (portabilityImportFake) Import(context.Context, *portability.ValidatedCoursePackage) (portability.ImportResult, error) {
	return portability.ImportResult{}, errors.New("not reached")
}

type portabilityPreviewReaderFake struct {
	value *portability.ValidatedCoursePackage
}

func (f portabilityPreviewReaderFake) Read(context.Context, io.Reader) (*portability.ValidatedCoursePackage, error) {
	return f.value, nil
}

type portabilityExecuteFake struct{ result portability.ImportResult }

func (f portabilityExecuteFake) Import(_ context.Context, value *portability.ValidatedCoursePackage) (portability.ImportResult, error) {
	if value == nil {
		return portability.ImportResult{}, errors.New("missing preview package")
	}
	return f.result, nil
}

func (f portabilityCourseFake) GetImmutableCourseVersionByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error) {
	return f.value, f.err
}

func TestCoursePackageExportHidesUnauthorizedAndStreamsAuthorizedVersion(t *testing.T) {
	current := loginTestCurrent(t)
	version, _ := courses.ParseVersion("1.2.3")
	courseID := courses.CourseID("11111111-1111-4111-8111-111111111111")
	value := courses.ImmutableCourseVersion{CourseVersion: courses.CourseVersionInput{Title: "A course / title", Attribution: []courses.ContributorSnapshot{{UserID: string(current.UserID()), Role: courses.ContributorAuthor}}}}
	auth := &authHTTP{sessions: &authResolverFake{current: current}, portabilityExporter: portabilityExportFake{}, portabilityCourses: portabilityCourseFake{value: value}}
	router := authTestRouter(auth)
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	response := authRequest(router, http.MethodGet, "/api/courses/by-id/"+string(courseID)+"/versions/"+version.String()+"/export", "", cookie)
	if response.Code != http.StatusOK || !strings.HasPrefix(response.Body.String(), "PK") || response.Header().Get("Content-Type") != "application/zip" || !strings.Contains(response.Header().Get("Content-Disposition"), "1.2.3.btg-course.zip") {
		t.Fatalf("authorized export = %d %q %#v", response.Code, response.Body.String(), response.Header())
	}
	value.CourseVersion.Attribution = nil
	auth.portabilityCourses = portabilityCourseFake{value: value}
	denied := authRequest(router, http.MethodGet, "/api/courses/by-id/"+string(courseID)+"/versions/"+version.String()+"/export", "", cookie)
	if denied.Code != http.StatusNotFound {
		t.Fatalf("unauthorized export = %d", denied.Code)
	}
	value.CourseVersion.Attribution = []courses.ContributorSnapshot{{UserID: string(current.UserID()), Role: courses.ContributorMaintainer}}
	auth.portabilityCourses = portabilityCourseFake{value: value}
	maintainer := authRequest(router, http.MethodGet, "/api/courses/by-id/"+string(courseID)+"/versions/"+version.String()+"/export", "", cookie)
	if maintainer.Code != http.StatusOK {
		t.Fatalf("maintainer export = %d", maintainer.Code)
	}
	auth.portabilityCourses = portabilityCourseFake{value: value}
	auth.portabilityExporter = portabilityExportFake{err: portability.ErrExportIntegrity}
	failure := authRequest(router, http.MethodGet, "/api/courses/by-id/"+string(courseID)+"/versions/"+version.String()+"/export", "", cookie)
	if failure.Code != http.StatusInternalServerError || strings.Contains(failure.Body.String(), "integrity") {
		t.Fatalf("export failure = %d %s", failure.Code, failure.Body.String())
	}
}

func TestPortabilityPreviewRequiresCapabilityAndMapsInvalidPackage(t *testing.T) {
	current := loginTestCurrent(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	for _, tc := range []struct {
		name       string
		authorizer *authorizationFake
		status     int
	}{
		{"denied", &authorizationFake{err: identity.ErrAuthorizationDenied}, http.StatusForbidden},
		{"invalid package", &authorizationFake{}, http.StatusUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &authHTTP{sessions: &authResolverFake{current: current}, authorizer: tc.authorizer, portabilityImporter: portabilityImportFake{}, portabilityReader: portability.NewReader(portability.DefaultLimits()), portabilityPreviews: newPortabilityPreviewStore(time.Now)}
			router := authTestRouter(auth)
			req := httptest.NewRequest(http.MethodPost, "/api/portability/imports/preview", strings.NewReader("not a zip"))
			req.Header.Set("Origin", "https://academy.example.com")
			req.Header.Set("X-CSRF-Token", authTestCSRFToken().Value())
			req.Header.Set("Content-Type", "application/zip")
			req.AddCookie(cookie)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
			}
			if tc.authorizer.calls != 1 || tc.authorizer.capability != identity.CapabilityPortabilityImport {
				t.Fatalf("capability not checked: %#v", tc.authorizer)
			}
		})
	}
}

func TestPortabilityPreviewStoreBindsOwnerAndExpires(t *testing.T) {
	now := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	store := newPortabilityPreviewStore(func() time.Time { return now })
	if _, _, err := store.create(identity.UserID("owner"), identity.SessionID("session"), nil); err == nil {
		t.Fatal("nil package accepted")
	}
	store.sessions["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] = portabilityPreviewSession{owner: "owner", expiresAt: now.Add(-time.Second)}
	if _, ok := store.get("owner", "session", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); ok {
		t.Fatal("expired preview resolved")
	}
}

func TestPortabilityPreviewThenExecuteIsOneShotAndSessionBound(t *testing.T) {
	current := loginTestCurrent(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	auth := &authHTTP{
		sessions:            &authResolverFake{current: current},
		authorizer:          &authorizationFake{},
		portabilityReader:   portabilityPreviewReaderFake{value: &portability.ValidatedCoursePackage{}},
		portabilityImporter: portabilityExecuteFake{result: portability.ImportResult{ImportID: "import-1", CourseID: "course-1", CourseVersionID: "version-1", Disposition: portability.ImportCreated}},
		portabilityPreviews: newPortabilityPreviewStore(time.Now),
	}
	router := authTestRouter(auth)
	preview := httptest.NewRequest(http.MethodPost, "/api/portability/imports/preview", strings.NewReader("zip"))
	preview.Header.Set("Origin", "https://academy.example.com")
	preview.Header.Set("X-CSRF-Token", authTestCSRFToken().Value())
	preview.Header.Set("Content-Type", "application/zip")
	preview.AddCookie(cookie)
	previewResponse := httptest.NewRecorder()
	router.ServeHTTP(previewResponse, preview)
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("preview = %d %s", previewResponse.Code, previewResponse.Body.String())
	}
	var payload struct {
		Token string `json:"previewToken"`
	}
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &payload); err != nil || len(payload.Token) != 64 {
		t.Fatalf("preview token = %#v, %v", payload, err)
	}
	execute := httptest.NewRequest(http.MethodPost, "/api/portability/imports/"+payload.Token+"/execute", nil)
	execute.Header.Set("Origin", "https://academy.example.com")
	execute.Header.Set("X-CSRF-Token", authTestCSRFToken().Value())
	execute.AddCookie(cookie)
	executeResponse := httptest.NewRecorder()
	router.ServeHTTP(executeResponse, execute)
	if executeResponse.Code != http.StatusOK || !strings.Contains(executeResponse.Body.String(), `"status":"IMPORTED"`) {
		t.Fatalf("execute = %d %s", executeResponse.Code, executeResponse.Body.String())
	}
	retry := httptest.NewRequest(http.MethodPost, "/api/portability/imports/"+payload.Token+"/execute", nil)
	retry.Header.Set("Origin", "https://academy.example.com")
	retry.Header.Set("X-CSRF-Token", authTestCSRFToken().Value())
	retry.AddCookie(cookie)
	retryResponse := httptest.NewRecorder()
	router.ServeHTTP(retryResponse, retry)
	if retryResponse.Code != http.StatusNotFound {
		t.Fatalf("used preview token = %d", retryResponse.Code)
	}
}

func TestPortabilityProblemDoesNotExposeInternalFailure(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/portability/imports/preview", nil)
	w := httptest.NewRecorder()
	portabilityProblem(w, r, errors.New("/private/storage/path"))
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "/private") {
		t.Fatalf("unsafe error: %d %s", w.Code, w.Body.String())
	}
}
