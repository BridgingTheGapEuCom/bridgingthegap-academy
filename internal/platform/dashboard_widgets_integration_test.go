//go:build integration

package platform

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
	pluginspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type dashboardHTTPAuthorizer struct {
	allowed map[identity.Capability]bool
	calls   []identity.Capability
}

func (a *dashboardHTTPAuthorizer) Authorize(_ context.Context, _ identity.AuthenticatedActor, capability identity.Capability, _ identity.Resource) error {
	a.calls = append(a.calls, capability)
	if a.allowed[capability] {
		return nil
	}
	return identity.ErrAuthorizationDenied
}

type dashboardHTTPFixture struct {
	service   *plugins.DashboardPlacementService
	registry  *plugins.RegistryService
	dashboard plugins.InstalledRelease
	course    plugins.InstalledRelease
	disabled  plugins.InstalledRelease
}

func newDashboardHTTPFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) dashboardHTTPFixture {
	t.Helper()
	repository := pluginspostgres.New(pool)
	registry := plugins.NewRegistryService(repository, plugins.Policy{})
	public, private, err := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{43}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	ids := []plugins.PluginID{"eu.com.bridgingthegap.widgets.dashboard-http", "eu.com.bridgingthegap.widgets.course-http", "eu.com.bridgingthegap.widgets.disabled-dashboard-http"}
	if err := repository.RegisterKey(ctx, plugins.VerificationKey{ID: "dashboard-http-owned-2026", PublicKey: public, Purpose: plugins.KeyPurposeOwned, AllowedPluginIDs: ids, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	register := func(id plugins.PluginID, widgetID string, kind plugins.PluginType, enabled bool) plugins.InstalledRelease {
		manifest := plugins.Manifest{Format: plugins.PackageFormat, FormatVersion: 1, ID: id, Version: "1.0.0", Name: "HTTP " + widgetID, Description: "safe widget metadata", Publisher: plugins.Publisher{Name: "BTG"}, PluginTypes: []plugins.PluginType{kind}, Entrypoints: []plugins.Entrypoint{{ID: widgetID, Type: kind, Name: "HTTP " + widgetID, Resource: "resources/widget.js"}}, Permissions: []plugins.Permission{plugins.PermissionNone}}
		archive, err := plugins.BuildPackageForTesting(manifest, map[string][]byte{"resources/widget.js": []byte("export default {}")}, "dashboard-http-owned-2026", private)
		if err != nil {
			t.Fatal(err)
		}
		pkg, err := plugins.NewPackageReader(plugins.Limits{}).Read(ctx, bytes.NewReader(archive))
		if err != nil {
			t.Fatal(err)
		}
		release, _, err := registry.Register(ctx, pkg)
		if err != nil {
			t.Fatal(err)
		}
		if enabled {
			release, err = registry.Enable(ctx, id, manifest.Version)
			if err != nil {
				t.Fatal(err)
			}
		}
		return release
	}
	return dashboardHTTPFixture{
		service: plugins.NewDashboardPlacementService(registry, repository, time.Now), registry: registry,
		dashboard: register(ids[0], "dashboard", plugins.TypeDashboardWidget, true),
		course:    register(ids[1], "course", plugins.TypeCourseWidget, true),
		disabled:  register(ids[2], "disabled", plugins.TypeDashboardWidget, false),
	}
}

func dashboardHTTPRouter(t *testing.T, fixture dashboardHTTPFixture, allowed map[identity.Capability]bool) (http.Handler, *dashboardHTTPAuthorizer, *http.Cookie) {
	t.Helper()
	authorizer := &dashboardHTTPAuthorizer{allowed: allowed}
	router := authTestRouter(&authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, authorizer: authorizer, dashboardWidgets: &dashboardWidgetAPI{placements: fixture.service, registry: fixture.registry}})
	return router, authorizer, &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
}

func dashboardPlacementBody(release plugins.InstalledRelease, widget, configuration string) string {
	return `{"pluginId":"` + string(release.Release.PluginID) + `","pluginVersion":"` + release.Release.Version + `","artifactDigest":"` + release.Release.ArtifactDigest + `","widgetId":"` + widget + `","configuration":` + configuration + `}`
}

func decodeDashboardPlacements(t *testing.T, responseBody []byte) []dashboardPlacementResponse {
	t.Helper()
	var out struct {
		Widgets []dashboardPlacementResponse `json:"widgets"`
	}
	if err := json.Unmarshal(responseBody, &out); err != nil {
		t.Fatal(err)
	}
	return out.Widgets
}

func testDashboardWidgetHTTP(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	// Dashboard placement is a single installation-wide ordered list. Start the
	// HTTP contract fixture from an empty list rather than inheriting the
	// persistence subtest's deliberately retained placements.
	if _, err := pool.Exec(ctx, `DELETE FROM plugins.dashboard_widget_placement`); err != nil {
		t.Fatal(err)
	}
	fixture := newDashboardHTTPFixture(t, ctx, pool)
	manager := map[identity.Capability]bool{identity.CapabilityDashboardWidgetsManage: true}
	router, authorizer, cookie := dashboardHTTPRouter(t, fixture, manager)
	csrf := authTestCSRFToken().Value()

	// The exact capability, not the unrelated plugins.manage capability, gates
	// every Dashboard placement endpoint.
	for _, path := range []string{"/api/dashboard/widgets", "/api/dashboard/widgets/available"} {
		deniedRouter, denied, deniedCookie := dashboardHTTPRouter(t, fixture, map[identity.Capability]bool{identity.CapabilityPluginsManage: true})
		response := authRequest(deniedRouter, http.MethodGet, path, "", deniedCookie)
		if response.Code != http.StatusForbidden || len(denied.calls) != 1 || denied.calls[0] != identity.CapabilityDashboardWidgetsManage {
			t.Fatalf("plugins.manage granted %s: status=%d calls=%v", path, response.Code, denied.calls)
		}
	}
	pluginsOnlyRouter, pluginsOnly, pluginsOnlyCookie := dashboardHTTPRouter(t, fixture, map[identity.Capability]bool{identity.CapabilityPluginsManage: true})
	pluginsOnlyCreate := authRequest(pluginsOnlyRouter, http.MethodPost, "/api/dashboard/widgets", dashboardPlacementBody(fixture.dashboard, "dashboard", `{}`), pluginsOnlyCookie, csrf)
	if pluginsOnlyCreate.Code != http.StatusForbidden || len(pluginsOnly.calls) != 1 || pluginsOnly.calls[0] != identity.CapabilityDashboardWidgetsManage {
		t.Fatalf("plugins.manage granted placement creation: status=%d calls=%v", pluginsOnlyCreate.Code, pluginsOnly.calls)
	}
	for _, operation := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/dashboard/widgets", dashboardPlacementBody(fixture.dashboard, "dashboard", `{}`)},
		{http.MethodPatch, "/api/dashboard/widgets/11111111-1111-4111-8111-111111111111", `{"expectedRevision":1,"configuration":{},"enabled":true}`},
		{http.MethodPost, "/api/dashboard/widgets/11111111-1111-4111-8111-111111111111/move-up", `{"expectedRevisions":{}}`},
		{http.MethodPost, "/api/dashboard/widgets/11111111-1111-4111-8111-111111111111/move-down", `{"expectedRevisions":{}}`},
		{http.MethodDelete, "/api/dashboard/widgets/11111111-1111-4111-8111-111111111111", `{"expectedRevision":1}`},
	} {
		deniedRouter, denied, deniedCookie := dashboardHTTPRouter(t, fixture, nil)
		response := authRequest(deniedRouter, operation.method, operation.path, operation.body, deniedCookie, csrf)
		if response.Code != http.StatusForbidden || len(denied.calls) != 1 || denied.calls[0] != identity.CapabilityDashboardWidgetsManage {
			t.Fatalf("unauthorized %s %s = %d calls=%v", operation.method, operation.path, response.Code, denied.calls)
		}
	}

	available := authRequest(router, http.MethodGet, "/api/dashboard/widgets/available", "", cookie)
	if available.Code != http.StatusOK || available.Header().Get("Cache-Control") != "no-store" || strings.Contains(available.Body.String(), `"widgetId":"course"`) || strings.Contains(available.Body.String(), `"widgetId":"disabled"`) || !strings.Contains(available.Body.String(), `"widgetId":"dashboard"`) {
		t.Fatalf("available widgets = %d %s", available.Code, available.Body.String())
	}
	for _, forbidden := range []string{"publicKey", "signature", "approval", "token", "storage"} {
		if strings.Contains(strings.ToLower(available.Body.String()), forbidden) {
			t.Fatalf("discovery leaked %q: %s", forbidden, available.Body.String())
		}
	}

	for _, invalid := range []string{
		`{`,
		dashboardPlacementBody(fixture.dashboard, "dashboard", `{}`)[:len(dashboardPlacementBody(fixture.dashboard, "dashboard", `{}`))-1] + `,"unexpected":true}`,
		dashboardPlacementBody(fixture.dashboard, "missing", `{}`),
		dashboardPlacementBody(fixture.dashboard, "dashboard", `{"padding":"`+strings.Repeat("x", 17<<10)+`"}`),
		dashboardPlacementBody(fixture.course, "course", `{}`),
		dashboardPlacementBody(fixture.disabled, "disabled", `{}`),
		dashboardPlacementBody(fixture.dashboard, "dashboard", `{}`)[:len(dashboardPlacementBody(fixture.dashboard, "dashboard", `{}`))-65] + strings.Repeat("0", 64) + `","widgetId":"dashboard","configuration":{}}`,
	} {
		response := authRequest(router, http.MethodPost, "/api/dashboard/widgets", invalid, cookie, csrf)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid create status=%d body=%s", response.Code, response.Body.String())
		}
	}

	firstCreate := authRequest(router, http.MethodPost, "/api/dashboard/widgets", dashboardPlacementBody(fixture.dashboard, "dashboard", `{"rank":1}`), cookie, csrf)
	if firstCreate.Code != http.StatusCreated || firstCreate.Header().Get("Cache-Control") != "no-store" || strings.Contains(firstCreate.Body.String(), "createdAt") {
		t.Fatalf("first create=%d %s", firstCreate.Code, firstCreate.Body.String())
	}
	var first dashboardPlacementResponse
	if err := json.Unmarshal(firstCreate.Body.Bytes(), &first); err != nil || first.PluginID != fixture.dashboard.Release.PluginID || first.PluginVersion != fixture.dashboard.Release.Version || first.ArtifactDigest != fixture.dashboard.Release.ArtifactDigest || first.WidgetID != "dashboard" || first.Position != 0 || first.Revision != 1 {
		t.Fatalf("first placement=%#v err=%v", first, err)
	}
	secondCreate := authRequest(router, http.MethodPost, "/api/dashboard/widgets", dashboardPlacementBody(fixture.dashboard, "dashboard", `{"rank":2}`), cookie, csrf)
	if secondCreate.Code != http.StatusCreated {
		t.Fatalf("second create=%d %s", secondCreate.Code, secondCreate.Body.String())
	}
	var second dashboardPlacementResponse
	_ = json.Unmarshal(secondCreate.Body.Bytes(), &second)

	listed := authRequest(router, http.MethodGet, "/api/dashboard/widgets", "", cookie)
	placements := decodeDashboardPlacements(t, listed.Body.Bytes())
	if listed.Code != http.StatusOK || listed.Header().Get("Cache-Control") != "no-store" || len(placements) != 2 || placements[0].ID != first.ID || placements[1].ID != second.ID || placements[0].Position != 0 || placements[1].Position != 1 || strings.Contains(listed.Body.String(), "createdAt") {
		t.Fatalf("list=%d %s", listed.Code, listed.Body.String())
	}

	badPatch := authRequest(router, http.MethodPatch, "/api/dashboard/widgets/"+first.ID, `{"expectedRevision":1,"configuration":{},"enabled":true,"pluginId":"forged"}`, cookie, csrf)
	if badPatch.Code != http.StatusBadRequest {
		t.Fatalf("mutable release coordinates were accepted: %d", badPatch.Code)
	}
	malformedPatch := authRequest(router, http.MethodPatch, "/api/dashboard/widgets/"+first.ID, `{`, cookie, csrf)
	if malformedPatch.Code != http.StatusBadRequest {
		t.Fatalf("malformed patch=%d", malformedPatch.Code)
	}
	updated := authRequest(router, http.MethodPatch, "/api/dashboard/widgets/"+first.ID, `{"expectedRevision":1,"configuration":{"rank":3},"enabled":false}`, cookie, csrf)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"enabled":false`) || !strings.Contains(updated.Body.String(), `"revision":2`) {
		t.Fatalf("update=%d %s", updated.Code, updated.Body.String())
	}
	stale := authRequest(router, http.MethodPatch, "/api/dashboard/widgets/"+first.ID, `{"expectedRevision":1,"configuration":{},"enabled":true}`, cookie, csrf)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "Dashboard changed") {
		t.Fatalf("stale update=%d %s", stale.Code, stale.Body.String())
	}

	values := decodeDashboardPlacements(t, authRequest(router, http.MethodGet, "/api/dashboard/widgets", "", cookie).Body.Bytes())
	revisions := `{"expectedRevisions":{"` + values[0].ID + `":` + strconv.FormatInt(values[0].Revision, 10) + `,"` + values[1].ID + `":` + strconv.FormatInt(values[1].Revision, 10) + `}}`
	moveDown := authRequest(router, http.MethodPost, "/api/dashboard/widgets/"+values[0].ID+"/move-down", revisions, cookie, csrf)
	ordered := decodeDashboardPlacements(t, moveDown.Body.Bytes())
	if moveDown.Code != http.StatusOK || ordered[0].ID != second.ID || ordered[1].ID != first.ID || ordered[0].Position != 0 || ordered[1].Position != 1 {
		t.Fatalf("move down=%d %s", moveDown.Code, moveDown.Body.String())
	}
	boundaryRevisions := `{"expectedRevisions":{"` + ordered[0].ID + `":` + strconv.FormatInt(ordered[0].Revision, 10) + `,"` + ordered[1].ID + `":` + strconv.FormatInt(ordered[1].Revision, 10) + `}}`
	boundary := authRequest(router, http.MethodPost, "/api/dashboard/widgets/"+ordered[0].ID+"/move-up", boundaryRevisions, cookie, csrf)
	boundaryValues := decodeDashboardPlacements(t, boundary.Body.Bytes())
	if boundary.Code != http.StatusOK || boundaryValues[0].ID != ordered[0].ID || boundaryValues[1].ID != ordered[1].ID {
		t.Fatalf("boundary move=%d %s", boundary.Code, boundary.Body.String())
	}
	lastBoundaryRevisions := `{"expectedRevisions":{"` + boundaryValues[0].ID + `":` + strconv.FormatInt(boundaryValues[0].Revision, 10) + `,"` + boundaryValues[1].ID + `":` + strconv.FormatInt(boundaryValues[1].Revision, 10) + `}}`
	lastBoundary := authRequest(router, http.MethodPost, "/api/dashboard/widgets/"+boundaryValues[1].ID+"/move-down", lastBoundaryRevisions, cookie, csrf)
	lastBoundaryValues := decodeDashboardPlacements(t, lastBoundary.Body.Bytes())
	if lastBoundary.Code != http.StatusOK || lastBoundaryValues[0].ID != boundaryValues[0].ID || lastBoundaryValues[1].ID != boundaryValues[1].ID {
		t.Fatalf("last boundary move=%d %s", lastBoundary.Code, lastBoundary.Body.String())
	}
	incomplete := authRequest(router, http.MethodPost, "/api/dashboard/widgets/"+ordered[0].ID+"/move-down", `{"expectedRevisions":{"`+ordered[0].ID+`":1}}`, cookie, csrf)
	if incomplete.Code != http.StatusConflict {
		t.Fatalf("incomplete revisions=%d %s", incomplete.Code, incomplete.Body.String())
	}
	staleRevisions := `{"expectedRevisions":{"` + lastBoundaryValues[0].ID + `":` + strconv.FormatInt(lastBoundaryValues[0].Revision-1, 10) + `,"` + lastBoundaryValues[1].ID + `":` + strconv.FormatInt(lastBoundaryValues[1].Revision, 10) + `}}`
	staleReorder := authRequest(router, http.MethodPost, "/api/dashboard/widgets/"+lastBoundaryValues[0].ID+"/move-down", staleRevisions, cookie, csrf)
	if staleReorder.Code != http.StatusConflict {
		t.Fatalf("stale reorder=%d %s", staleReorder.Code, staleReorder.Body.String())
	}

	current := decodeDashboardPlacements(t, authRequest(router, http.MethodGet, "/api/dashboard/widgets", "", cookie).Body.Bytes())
	staleDelete := authRequest(router, http.MethodDelete, "/api/dashboard/widgets/"+current[0].ID, `{"expectedRevision":1}`, cookie, csrf)
	if staleDelete.Code != http.StatusConflict {
		t.Fatalf("stale delete=%d %s", staleDelete.Code, staleDelete.Body.String())
	}
	deleteResponse := authRequest(router, http.MethodDelete, "/api/dashboard/widgets/"+current[0].ID, `{"expectedRevision":`+strconv.FormatInt(current[0].Revision, 10)+`}`, cookie, csrf)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete=%d %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if after := decodeDashboardPlacements(t, authRequest(router, http.MethodGet, "/api/dashboard/widgets", "", cookie).Body.Bytes()); len(after) != 1 || after[0].ID != current[1].ID {
		t.Fatalf("post-delete=%#v", after)
	}
	if _, err := fixture.registry.Get(ctx, fixture.dashboard.Release.PluginID, fixture.dashboard.Release.Version); err != nil {
		t.Fatalf("placement deletion changed registry state: %v", err)
	}
	if len(authorizer.calls) == 0 {
		t.Fatal("manager requests did not reach capability policy")
	}

	// Origin and CSRF checks run before capability or application mutation.
	noCSRF := authRequest(router, http.MethodPost, "/api/dashboard/widgets", dashboardPlacementBody(fixture.dashboard, "dashboard", `{}`), cookie)
	if noCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF create=%d", noCSRF.Code)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/dashboard/widgets", strings.NewReader(dashboardPlacementBody(fixture.dashboard, "dashboard", `{}`)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("X-CSRF-Token", csrf)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin create=%d", response.Code)
	}
}

func testDashboardWidgetPlacements(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	repo := pluginspostgres.New(pool)
	registry := plugins.NewRegistryService(repo, plugins.Policy{})
	pub, priv, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{91}, 64)))
	manifest := plugins.Manifest{Format: plugins.PackageFormat, FormatVersion: 1, ID: "eu.com.bridgingthegap.widgets.dashboard", Version: "1.0.0", Name: "Dashboard", Description: "test", Publisher: plugins.Publisher{Name: "BTG"}, PluginTypes: []plugins.PluginType{plugins.TypeDashboardWidget}, Entrypoints: []plugins.Entrypoint{{ID: "dashboard", Type: plugins.TypeDashboardWidget, Name: "Dashboard", Resource: "resources/widget.js"}}, Permissions: []plugins.Permission{plugins.PermissionNone}}
	if err := repo.RegisterKey(ctx, plugins.VerificationKey{ID: "dashboard-owned-2026", PublicKey: pub, Purpose: plugins.KeyPurposeOwned, AllowedPluginIDs: []plugins.PluginID{manifest.ID}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	archive, err := plugins.BuildPackageForTesting(manifest, map[string][]byte{"resources/widget.js": []byte("export default {}")}, "dashboard-owned-2026", priv)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := plugins.NewPackageReader(plugins.Limits{}).Read(ctx, bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	release, _, err := registry.Register(ctx, pkg)
	if err != nil {
		t.Fatal(err)
	}
	release, err = registry.Enable(ctx, manifest.ID, manifest.Version)
	if err != nil {
		t.Fatal(err)
	}
	service := plugins.NewDashboardPlacementService(registry, repo, func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	create := func(config string) plugins.DashboardPlacement {
		value, err := service.Create(ctx, plugins.DashboardPlacement{PluginID: manifest.ID, PluginVersion: manifest.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "dashboard", Configuration: []byte(config)})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	first := create(`{"one":1}`)
	second := create(`{"two":2}`)
	if first.Position != 0 || second.Position != 1 || first.Revision != 1 || !bytes.Contains(first.Configuration, []byte(`"one"`)) {
		t.Fatalf("creation=%#v %#v", first, second)
	}
	storedFirst, err := repo.GetDashboardPlacement(ctx, first.ID)
	if err != nil || storedFirst.ID != first.ID || storedFirst.PluginID != manifest.ID || storedFirst.ArtifactDigest != release.Release.ArtifactDigest {
		t.Fatalf("placement lookup=%#v %v", storedFirst, err)
	}
	second.Configuration = []byte(`{"updated":true}`)
	updated, err := service.Update(ctx, second, second.Revision)
	if err != nil || updated.Revision != 2 {
		t.Fatalf("update=%#v %v", updated, err)
	}
	if _, err := service.Update(ctx, second, second.Revision); !errors.Is(err, plugins.ErrDashboardPlacementConflict) {
		t.Fatalf("stale update=%v", err)
	}
	values, _ := service.List(ctx)
	revisions := map[string]int64{values[0].ID: values[0].Revision, values[1].ID: values[1].Revision}
	ordered, err := service.Reorder(ctx, []string{values[1].ID, values[0].ID}, revisions)
	if err != nil || ordered[0].ID != second.ID || ordered[0].Position != 0 || ordered[1].Position != 1 {
		t.Fatalf("reorder=%#v %v", ordered, err)
	}
	stale := map[string]int64{ordered[0].ID: ordered[0].Revision, ordered[1].ID: 1}
	if _, err := service.Reorder(ctx, []string{ordered[1].ID, ordered[0].ID}, stale); !errors.Is(err, plugins.ErrDashboardPlacementConflict) {
		t.Fatalf("stale reorder=%v", err)
	}
	if err := service.Delete(ctx, ordered[0].ID, ordered[0].Revision); err != nil {
		t.Fatal(err)
	}
	left, _ := service.List(ctx)
	if len(left) != 1 || left[0].ID != first.ID {
		t.Fatalf("delete=%#v", left)
	}
	// Exact release pinning is placement eligibility, not client metadata.
	for _, invalid := range []plugins.DashboardPlacement{
		{PluginID: manifest.ID, PluginVersion: manifest.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "missing", Configuration: []byte(`{}`)},
		{PluginID: manifest.ID, PluginVersion: manifest.Version, ArtifactDigest: strings.Repeat("0", 64), WidgetID: "dashboard", Configuration: []byte(`{}`)},
	} {
		if _, err := service.Create(ctx, invalid); !errors.Is(err, plugins.ErrLaunchDenied) {
			t.Fatalf("invalid placement accepted: %v", err)
		}
	}
	if _, err := registry.Disable(ctx, manifest.ID, manifest.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, plugins.DashboardPlacement{PluginID: manifest.ID, PluginVersion: manifest.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "dashboard", Configuration: []byte(`{}`)}); !errors.Is(err, plugins.ErrLaunchDenied) {
		t.Fatalf("disabled release placed: %v", err)
	}
}
