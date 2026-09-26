//go:build integration

package platform

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
	pluginspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func pluginManagementHTTPRouter(t *testing.T, fixture dashboardHTTPFixture, allowed map[identity.Capability]bool) (http.Handler, *dashboardHTTPAuthorizer, *http.Cookie) {
	t.Helper()
	authorizer := &dashboardHTTPAuthorizer{allowed: allowed}
	router := authTestRouter(&authHTTP{
		sessions:         &authResolverFake{current: loginTestCurrent(t)},
		authorizer:       authorizer,
		pluginManagement: &pluginManagementAPI{registry: fixture.registry},
	})
	return router, authorizer, &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
}

func testPluginManagementHTTP(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	fixture := newDashboardHTTPFixture(t, ctx, pool)
	manager := map[identity.Capability]bool{identity.CapabilityPluginsManage: true}
	router, authorizer, cookie := pluginManagementHTTPRouter(t, fixture, manager)

	for _, path := range []string{"/api/plugins", "/api/plugins/" + string(fixture.dashboard.Release.PluginID) + "/" + fixture.dashboard.Release.Version} {
		response := authRequest(router, http.MethodGet, path, "", cookie)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("management read %s = %d %s", path, response.Code, response.Body.String())
		}
	}
	if len(authorizer.calls) != 2 || authorizer.calls[0] != identity.CapabilityPluginsManage || authorizer.calls[1] != identity.CapabilityPluginsManage {
		t.Fatalf("management capability calls=%v", authorizer.calls)
	}

	listed := authRequest(router, http.MethodGet, "/api/plugins", "", cookie)
	for _, expected := range []string{
		`"pluginId":"` + string(fixture.dashboard.Release.PluginID) + `"`,
		`"artifactDigest":"` + fixture.dashboard.Release.ArtifactDigest + `"`,
		`"currentTrust":"BTG_OWNED"`,
		`"enabled":true`,
		`"entrypoints"`,
	} {
		if !strings.Contains(listed.Body.String(), expected) {
			t.Fatalf("management list omitted %s: %s", expected, listed.Body.String())
		}
	}
	for _, forbidden := range []string{"installationId", "privateKey", "signingPayload", "signatureValue", "resourcePath", "token", "session"} {
		if strings.Contains(strings.ToLower(listed.Body.String()), strings.ToLower(forbidden)) {
			t.Fatalf("management list leaked %s: %s", forbidden, listed.Body.String())
		}
	}

	dashboardOnlyRouter, dashboardOnly, dashboardCookie := pluginManagementHTTPRouter(t, fixture, map[identity.Capability]bool{identity.CapabilityDashboardWidgetsManage: true})
	for _, path := range []string{"/api/plugins", "/api/plugins/" + string(fixture.dashboard.Release.PluginID) + "/" + fixture.dashboard.Release.Version} {
		response := authRequest(dashboardOnlyRouter, http.MethodGet, path, "", dashboardCookie)
		if response.Code != http.StatusForbidden {
			t.Fatalf("dashboard.widgets.manage granted %s: %d", path, response.Code)
		}
	}
	if len(dashboardOnly.calls) != 2 || dashboardOnly.calls[0] != identity.CapabilityPluginsManage || dashboardOnly.calls[1] != identity.CapabilityPluginsManage {
		t.Fatalf("wrong capability boundary=%v", dashboardOnly.calls)
	}

	unauthenticated := authRequest(router, http.MethodGet, "/api/plugins", "", nil)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("runtime/session substitution boundary=%d", unauthenticated.Code)
	}
	runtimeToken := httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
	runtimeToken.Header.Set("Authorization", "Bearer runtime-token-is-not-a-session")
	runtimeResponse := httptest.NewRecorder()
	router.ServeHTTP(runtimeResponse, runtimeToken)
	if runtimeResponse.Code != http.StatusUnauthorized {
		t.Fatalf("runtime bearer authenticated management read=%d", runtimeResponse.Code)
	}
	missing := authRequest(router, http.MethodGet, "/api/plugins/com.example.academy.missing/1.0.0", "", cookie)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing release=%d", missing.Code)
	}
}

func pluginPackageUploadRequest(router http.Handler, archive []byte, cookie *http.Cookie, csrf, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/plugins", bytes.NewReader(archive))
	request.Header.Set("Content-Type", pluginPackageUploadMediaType)
	request.Header.Set("Origin", origin)
	request.Header.Set("X-CSRF-Token", csrf)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func mutatePluginArchive(t *testing.T, archive []byte, mutate func(string, []byte) []byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range reader.File {
		body, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(body)
		if err != nil {
			t.Fatal(err)
		}
		_ = body.Close()
		entry, err := writer.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(mutate(file.Name, data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func unsafePluginArchive(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	entry, err := writer.Create("../unsafe.js")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("unsafe")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func malformedPluginManifestArchive(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	entry, err := writer.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(`{"format":`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func testPluginPackageRegistrationHTTP(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	repository := pluginspostgres.New(pool)
	registry := plugins.NewRegistryService(repository, plugins.Policy{})
	authorizer := &dashboardHTTPAuthorizer{allowed: map[identity.Capability]bool{identity.CapabilityPluginsManage: true}}
	router := authTestRouter(&authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, authorizer: authorizer, pluginManagement: &pluginManagementAPI{registry: registry}})
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	csrf := authTestCSRFToken().Value()
	manifest := plugins.Manifest{Format: plugins.PackageFormat, FormatVersion: 1, ID: "com.example.academy.install-http", Version: "1.0.0", Name: "Install HTTP", Description: "safe", Publisher: plugins.Publisher{Name: "Example"}, PluginTypes: []plugins.PluginType{plugins.TypeCourseWidget, plugins.TypeDashboardWidget}, Entrypoints: []plugins.Entrypoint{{ID: "course", Type: plugins.TypeCourseWidget, Name: "Course", Resource: "resources/course.js"}, {ID: "dashboard", Type: plugins.TypeDashboardWidget, Name: "Dashboard", Resource: "resources/dashboard.js"}}, Permissions: []plugins.Permission{plugins.PermissionNone}}
	archive, err := plugins.BuildPackageForTesting(manifest, map[string][]byte{"resources/course.js": []byte("course"), "resources/dashboard.js": []byte("dashboard")}, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	created := pluginPackageUploadRequest(router, archive, cookie, csrf, "https://academy.example.com")
	if created.Code != http.StatusCreated || created.Header().Get("Cache-Control") != "no-store" || !strings.Contains(created.Body.String(), `"currentTrust":"UNKNOWN"`) || !strings.Contains(created.Body.String(), `"enabled":false`) || !strings.Contains(created.Body.String(), `"entrypoints"`) {
		t.Fatalf("unsigned registration=%d %s", created.Code, created.Body.String())
	}
	release, err := registry.Get(ctx, manifest.ID, manifest.Version)
	if err != nil || release.Enabled || release.CurrentTrust != plugins.TrustUnknown {
		t.Fatalf("registered release=%#v err=%v", release, err)
	}
	resource, err := repository.GetResource(ctx, release.Release, "resources/dashboard.js")
	if err != nil || string(resource.Content) != "dashboard" {
		t.Fatalf("registered resource=%#v err=%v", resource, err)
	}
	var placementCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM plugins.dashboard_widget_placement`).Scan(&placementCount); err != nil || placementCount != 0 {
		t.Fatalf("registration changed Dashboard placement state: count=%d err=%v", placementCount, err)
	}
	detail := authRequest(router, http.MethodGet, "/api/plugins/"+string(manifest.ID)+"/"+manifest.Version, "", cookie)
	if detail.Code != http.StatusOK || detail.Body.String() != created.Body.String() {
		t.Fatalf("management detail=%d %s", detail.Code, detail.Body.String())
	}
	replay := pluginPackageUploadRequest(router, archive, cookie, csrf, "https://academy.example.com")
	if replay.Code != http.StatusOK || replay.Body.String() != created.Body.String() {
		t.Fatalf("idempotent replay=%d %s", replay.Code, replay.Body.String())
	}
	changed, err := plugins.BuildPackageForTesting(manifest, map[string][]byte{"resources/course.js": []byte("changed"), "resources/dashboard.js": []byte("dashboard")}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	conflict := pluginPackageUploadRequest(router, changed, cookie, csrf, "https://academy.example.com")
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "release_conflict") {
		t.Fatalf("immutable conflict=%d %s", conflict.Code, conflict.Body.String())
	}
	if after, err := repository.GetResource(ctx, release.Release, "resources/course.js"); err != nil || string(after.Content) != "course" {
		t.Fatalf("conflict changed registered resource=%#v err=%v", after, err)
	}

	signedManifest := manifest
	signedManifest.ID = "com.example.academy.signed-install-http"
	signedManifest.Version = "2.0.0"
	public, private, err := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{61}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.RegisterKey(ctx, plugins.VerificationKey{ID: "install-http-owned-2026", PublicKey: public, Purpose: plugins.KeyPurposeOwned, AllowedPluginIDs: []plugins.PluginID{signedManifest.ID}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	signedArchive, err := plugins.BuildPackageForTesting(signedManifest, map[string][]byte{"resources/course.js": []byte("course"), "resources/dashboard.js": []byte("dashboard")}, "install-http-owned-2026", private)
	if err != nil {
		t.Fatal(err)
	}
	signed := pluginPackageUploadRequest(router, signedArchive, cookie, csrf, "https://academy.example.com")
	if signed.Code != http.StatusCreated || !strings.Contains(signed.Body.String(), `"currentTrust":"BTG_OWNED"`) {
		t.Fatalf("signed registration=%d %s", signed.Code, signed.Body.String())
	}

	for name, body := range map[string][]byte{
		"non zip":            []byte("not a zip"),
		"malformed manifest": malformedPluginManifestArchive(t),
		"unsafe entry":       unsafePluginArchive(t),
		"checksum mismatch": mutatePluginArchive(t, archive, func(path string, data []byte) []byte {
			if path == "resources/course.js" {
				return []byte("tampered")
			}
			return data
		}),
	} {
		response := pluginPackageUploadRequest(router, body, cookie, csrf, "https://academy.example.com")
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s=%d %s", name, response.Code, response.Body.String())
		}
	}
	wrongPublic, _, err := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{62}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	invalidSignatureManifest := signedManifest
	invalidSignatureManifest.ID = "com.example.academy.invalid-signature-install-http"
	invalidSignatureManifest.Version = "3.0.0"
	if err := repository.RegisterKey(ctx, plugins.VerificationKey{ID: "invalid-install-http-2026", PublicKey: wrongPublic, Purpose: plugins.KeyPurposeOwned, AllowedPluginIDs: []plugins.PluginID{invalidSignatureManifest.ID}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	invalidSignature, err := plugins.BuildPackageForTesting(invalidSignatureManifest, map[string][]byte{"resources/course.js": []byte("course"), "resources/dashboard.js": []byte("dashboard")}, "invalid-install-http-2026", private)
	if err != nil {
		t.Fatal(err)
	}
	invalidSignatureResponse := pluginPackageUploadRequest(router, invalidSignature, cookie, csrf, "https://academy.example.com")
	if invalidSignatureResponse.Code != http.StatusBadRequest || !strings.Contains(invalidSignatureResponse.Body.String(), "invalid_signature") {
		t.Fatalf("invalid signature=%d %s", invalidSignatureResponse.Code, invalidSignatureResponse.Body.String())
	}

	overLimitRequest := httptest.NewRequest(http.MethodPost, "/api/plugins", bytes.NewReader([]byte("small")))
	overLimitRequest.Header.Set("Content-Type", pluginPackageUploadMediaType)
	overLimitRequest.Header.Set("Origin", "https://academy.example.com")
	overLimitRequest.Header.Set("X-CSRF-Token", csrf)
	overLimitRequest.AddCookie(cookie)
	overLimitRequest.ContentLength = plugins.DefaultLimits().MaxCompressedBytes + 1
	overLimitResponse := httptest.NewRecorder()
	router.ServeHTTP(overLimitResponse, overLimitRequest)
	if overLimitResponse.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over limit=%d %s", overLimitResponse.Code, overLimitResponse.Body.String())
	}

	dashboardOnlyRouter, _, dashboardOnlyCookie := pluginManagementHTTPRouter(t, dashboardHTTPFixture{registry: registry}, map[identity.Capability]bool{identity.CapabilityDashboardWidgetsManage: true})
	if response := pluginPackageUploadRequest(dashboardOnlyRouter, archive, dashboardOnlyCookie, csrf, "https://academy.example.com"); response.Code != http.StatusForbidden {
		t.Fatalf("dashboard authority installed plugin=%d", response.Code)
	}
	if response := pluginPackageUploadRequest(router, archive, nil, csrf, "https://academy.example.com"); response.Code != http.StatusUnauthorized {
		t.Fatalf("missing session=%d", response.Code)
	}
	runtimeBearerRequest := httptest.NewRequest(http.MethodPost, "/api/plugins", bytes.NewReader(archive))
	runtimeBearerRequest.Header.Set("Content-Type", pluginPackageUploadMediaType)
	runtimeBearerRequest.Header.Set("Origin", "https://academy.example.com")
	runtimeBearerRequest.Header.Set("X-CSRF-Token", csrf)
	runtimeBearerRequest.Header.Set("Authorization", "Bearer runtime-token-is-not-a-session")
	runtimeBearerResponse := httptest.NewRecorder()
	router.ServeHTTP(runtimeBearerResponse, runtimeBearerRequest)
	if runtimeBearerResponse.Code != http.StatusUnauthorized {
		t.Fatalf("runtime bearer installed plugin=%d", runtimeBearerResponse.Code)
	}
	if response := pluginPackageUploadRequest(router, archive, cookie, "", "https://academy.example.com"); response.Code != http.StatusForbidden {
		t.Fatalf("missing csrf=%d", response.Code)
	}
	if response := pluginPackageUploadRequest(router, archive, cookie, csrf, "https://attacker.example"); response.Code != http.StatusForbidden {
		t.Fatalf("untrusted origin=%d", response.Code)
	}
}

func testPluginLifecycleHTTP(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	repository := pluginspostgres.New(pool)
	registry := plugins.NewRegistryService(repository, plugins.Policy{})
	router, _, cookie := pluginManagementHTTPRouter(t, dashboardHTTPFixture{registry: registry}, map[identity.Capability]bool{identity.CapabilityPluginsManage: true})
	csrf := authTestCSRFToken().Value()
	publicKey, privateKey, err := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{73}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	keyBody := `{"keyId":"lifecycle-http-approval-2026","publicKey":"` + base64.RawURLEncoding.EncodeToString(publicKey) + `","purpose":"BTG_APPROVAL_SIGNING","allowedPluginIds":[]}`
	keyResponse := authRequest(router, http.MethodPost, "/api/plugins/keys", keyBody, cookie, csrf)
	if keyResponse.Code != http.StatusOK || !strings.Contains(keyResponse.Body.String(), `"purpose":"BTG_APPROVAL_SIGNING"`) || !strings.Contains(keyResponse.Body.String(), `"fingerprint":"sha256:`) || strings.Contains(keyResponse.Body.String(), "publicKey") {
		t.Fatalf("key registration=%d %s", keyResponse.Code, keyResponse.Body.String())
	}
	if replay := authRequest(router, http.MethodPost, "/api/plugins/keys", keyBody, cookie, csrf); replay.Code != http.StatusOK {
		t.Fatalf("key replay=%d %s", replay.Code, replay.Body.String())
	}
	otherPublic, _, err := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{74}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	conflictingKeyBody := `{"keyId":"lifecycle-http-approval-2026","publicKey":"` + base64.RawURLEncoding.EncodeToString(otherPublic) + `","purpose":"BTG_APPROVAL_SIGNING","allowedPluginIds":[]}`
	if response := authRequest(router, http.MethodPost, "/api/plugins/keys", conflictingKeyBody, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("key conflict=%d %s", response.Code, response.Body.String())
	}
	privateBody := `{"keyId":"private-expanded-key","publicKey":"` + base64.RawURLEncoding.EncodeToString(privateKey) + `","purpose":"BTG_APPROVAL_SIGNING","allowedPluginIds":[]}`
	if response := authRequest(router, http.MethodPost, "/api/plugins/keys", privateBody, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("expanded private key accepted=%d %s", response.Code, response.Body.String())
	}

	pluginID := plugins.PluginID("com.example.academy.lifecycle-http")
	manifest := plugins.Manifest{Format: plugins.PackageFormat, FormatVersion: 1, ID: pluginID, Version: "1.0.0", Name: "Lifecycle", Description: "Lifecycle fixture", Publisher: plugins.Publisher{Name: "Community"}, PluginTypes: []plugins.PluginType{plugins.TypeCourseWidget, plugins.TypeDashboardWidget}, Entrypoints: []plugins.Entrypoint{{ID: "course", Type: plugins.TypeCourseWidget, Name: "Course", Resource: "resources/course.js"}, {ID: "dashboard", Type: plugins.TypeDashboardWidget, Name: "Dashboard", Resource: "resources/dashboard.js"}}, Permissions: []plugins.Permission{plugins.PermissionNone}}
	archive, err := plugins.BuildPackageForTesting(manifest, map[string][]byte{"resources/course.js": []byte("course"), "resources/dashboard.js": []byte("dashboard")}, "lifecycle-http-approval-2026", privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if response := pluginPackageUploadRequest(router, archive, cookie, csrf, "https://academy.example.com"); response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"currentTrust":"UNKNOWN"`) {
		t.Fatalf("approval-signed registration=%d %s", response.Code, response.Body.String())
	}
	basePath := "/api/plugins/" + string(pluginID) + "/" + manifest.Version
	approved := authRequest(router, http.MethodPost, basePath+"/approval", "", cookie, csrf)
	if approved.Code != http.StatusOK || !strings.Contains(approved.Body.String(), `"currentTrust":"BTG_APPROVED"`) || !strings.Contains(approved.Body.String(), `"approvalState":"ACTIVE"`) || !strings.Contains(approved.Body.String(), `"enabled":false`) {
		t.Fatalf("approve=%d %s", approved.Code, approved.Body.String())
	}
	if replay := authRequest(router, http.MethodPost, basePath+"/approval", "", cookie, csrf); replay.Code != http.StatusOK || !strings.Contains(replay.Body.String(), `"approvalState":"ACTIVE"`) {
		t.Fatalf("approval replay=%d %s", replay.Code, replay.Body.String())
	}
	enabled := authRequest(router, http.MethodPost, basePath+"/enable", "", cookie, csrf)
	if enabled.Code != http.StatusOK || !strings.Contains(enabled.Body.String(), `"enabled":true`) || !strings.Contains(enabled.Body.String(), `"executionPermitted":true`) {
		t.Fatalf("enable=%d %s", enabled.Code, enabled.Body.String())
	}

	release, err := registry.Get(ctx, pluginID, manifest.Version)
	if err != nil {
		t.Fatal(err)
	}
	placements := plugins.NewDashboardPlacementService(registry, repository, time.Now)
	placement, err := placements.Create(ctx, plugins.DashboardPlacement{PluginID: pluginID, PluginVersion: manifest.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "dashboard", Configuration: []byte(`{"lifecycle":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, cleanupErr := pool.Exec(ctx, `DELETE FROM plugins.dashboard_widget_placement WHERE placement_id = $1`, placement.ID); cleanupErr != nil {
			t.Errorf("clean lifecycle placement: %v", cleanupErr)
		}
	}()
	var coursesBefore, dashboardBefore []byte
	if err := pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(v) ORDER BY v.id), '[]'::jsonb) FROM courses.course_version v`).Scan(&coursesBefore); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.placement_id), '[]'::jsonb) FROM plugins.dashboard_widget_placement p`).Scan(&dashboardBefore); err != nil {
		t.Fatal(err)
	}

	tokens, err := plugins.NewRuntimeTokenService("lifecycle-http-runtime-2026", bytes.Repeat([]byte{75}, ed25519.SeedSize), plugins.DefaultTokenLifetime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := plugins.NewRuntimeService(registry, tokens, "https://plugins.academy.example", "https://academy.example")
	if err != nil {
		t.Fatal(err)
	}
	launch, err := runtime.PrepareWidgetRuntime(ctx, pluginID, manifest.Version, "course", plugins.TypeCourseWidget)
	if err != nil {
		t.Fatal(err)
	}
	revoked := authRequest(router, http.MethodDelete, basePath+"/approval", "", cookie, csrf)
	if revoked.Code != http.StatusOK || !strings.Contains(revoked.Body.String(), `"currentTrust":"UNKNOWN"`) || !strings.Contains(revoked.Body.String(), `"approvalState":"REVOKED"`) || !strings.Contains(revoked.Body.String(), `"enabled":true`) || !strings.Contains(revoked.Body.String(), `"executionPermitted":false`) {
		t.Fatalf("revoke=%d %s", revoked.Code, revoked.Body.String())
	}
	if replay := authRequest(router, http.MethodDelete, basePath+"/approval", "", cookie, csrf); replay.Code != http.StatusOK {
		t.Fatalf("revocation replay=%d %s", replay.Code, replay.Body.String())
	}
	if _, err := runtime.PrepareWidgetRuntime(ctx, pluginID, manifest.Version, "course", plugins.TypeCourseWidget); !errors.Is(err, plugins.ErrLaunchDenied) {
		t.Fatalf("launch after revoke=%v", err)
	}
	if _, err := runtime.Refresh(ctx, launch.Token); !errors.Is(err, plugins.ErrLaunchDenied) {
		t.Fatalf("refresh after revoke=%v", err)
	}
	if response := authRequest(router, http.MethodPost, basePath+"/enable", "", cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("ineligible enable=%d %s", response.Code, response.Body.String())
	}
	if response := authRequest(router, http.MethodPost, basePath+"/approval", "", cookie, csrf); response.Code != http.StatusOK {
		t.Fatalf("reapprove=%d %s", response.Code, response.Body.String())
	}
	restored, err := runtime.PrepareWidgetRuntime(ctx, pluginID, manifest.Version, "course", plugins.TypeCourseWidget)
	if err != nil || restored.Token == "" {
		t.Fatalf("launch after restored approval=%#v err=%v", restored, err)
	}
	disabled := authRequest(router, http.MethodPost, basePath+"/disable", "", cookie, csrf)
	if disabled.Code != http.StatusOK || !strings.Contains(disabled.Body.String(), `"enabled":false`) {
		t.Fatalf("disable=%d %s", disabled.Code, disabled.Body.String())
	}
	if _, err := runtime.Refresh(ctx, restored.Token); !errors.Is(err, plugins.ErrLaunchDenied) {
		t.Fatalf("refresh after disable=%v", err)
	}
	if response := authRequest(router, http.MethodPost, basePath+"/enable", "", cookie, csrf); response.Code != http.StatusOK {
		t.Fatalf("re-enable=%d %s", response.Code, response.Body.String())
	}
	keyDisabled := authRequest(router, http.MethodPost, "/api/plugins/keys/lifecycle-http-approval-2026/disable", "", cookie, csrf)
	if keyDisabled.Code != http.StatusOK || !strings.Contains(keyDisabled.Body.String(), `"enabled":false`) {
		t.Fatalf("key disable=%d %s", keyDisabled.Code, keyDisabled.Body.String())
	}
	if _, err := runtime.PrepareWidgetRuntime(ctx, pluginID, manifest.Version, "course", plugins.TypeCourseWidget); !errors.Is(err, plugins.ErrLaunchDenied) {
		t.Fatalf("launch after key disable=%v", err)
	}
	if response := authRequest(router, http.MethodPost, "/api/plugins/keys/lifecycle-http-approval-2026/enable", "", cookie, csrf); response.Code != http.StatusOK {
		t.Fatalf("key re-enable=%d %s", response.Code, response.Body.String())
	}
	if _, err := runtime.PrepareWidgetRuntime(ctx, pluginID, manifest.Version, "course", plugins.TypeCourseWidget); err != nil {
		t.Fatalf("launch after key re-enable=%v", err)
	}

	listedKeys := authRequest(router, http.MethodGet, "/api/plugins/keys", "", cookie)
	if listedKeys.Code != http.StatusOK || listedKeys.Header().Get("Cache-Control") != "no-store" || !strings.Contains(listedKeys.Body.String(), "lifecycle-http-approval-2026") || strings.Contains(listedKeys.Body.String(), "publicKey") || strings.Contains(listedKeys.Body.String(), base64.RawURLEncoding.EncodeToString(privateKey)) {
		t.Fatalf("key list=%d %s", listedKeys.Code, listedKeys.Body.String())
	}
	var coursesAfter, dashboardAfter []byte
	if err := pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(v) ORDER BY v.id), '[]'::jsonb) FROM courses.course_version v`).Scan(&coursesAfter); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.placement_id), '[]'::jsonb) FROM plugins.dashboard_widget_placement p`).Scan(&dashboardAfter); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(coursesBefore, coursesAfter) || !bytes.Equal(dashboardBefore, dashboardAfter) {
		t.Fatal("plugin lifecycle rewrote CourseVersions or Dashboard placements")
	}
	values, err := placements.List(ctx)
	if err != nil || len(values) == 0 || values[len(values)-1].ID != placement.ID || values[len(values)-1].PluginVersion != manifest.Version || values[len(values)-1].ArtifactDigest != release.Release.ArtifactDigest {
		t.Fatalf("placement changed=%#v err=%v", values, err)
	}

	dashboardOnlyRouter, _, dashboardOnlyCookie := pluginManagementHTTPRouter(t, dashboardHTTPFixture{registry: registry}, map[identity.Capability]bool{identity.CapabilityDashboardWidgetsManage: true})
	for _, operation := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/plugins/keys", ""},
		{http.MethodPost, "/api/plugins/keys", keyBody},
		{http.MethodPost, "/api/plugins/keys/lifecycle-http-approval-2026/enable", ""},
		{http.MethodPost, "/api/plugins/keys/lifecycle-http-approval-2026/disable", ""},
		{http.MethodPost, basePath + "/approval", ""},
		{http.MethodDelete, basePath + "/approval", ""},
		{http.MethodPost, basePath + "/enable", ""},
		{http.MethodPost, basePath + "/disable", ""},
	} {
		response := authRequest(dashboardOnlyRouter, operation.method, operation.path, operation.body, dashboardOnlyCookie, csrf)
		if response.Code != http.StatusForbidden {
			t.Fatalf("dashboard authority allowed %s %s: %d", operation.method, operation.path, response.Code)
		}
	}
	if response := authRequest(router, http.MethodPost, basePath+"/disable", "", cookie); response.Code != http.StatusForbidden {
		t.Fatalf("missing csrf lifecycle=%d", response.Code)
	}
}
