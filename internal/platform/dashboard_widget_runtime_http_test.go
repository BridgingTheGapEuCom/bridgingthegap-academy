package platform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
)

type dashboardLaunchHTTPFake struct {
	placementID string
	calls       int
	err         error
}

func (f *dashboardLaunchHTTPFake) Prepare(_ context.Context, placementID string) (plugins.RuntimeLaunch, error) {
	f.calls++
	f.placementID = placementID
	if f.err != nil {
		return plugins.RuntimeLaunch{}, f.err
	}
	return plugins.RuntimeLaunch{Context: plugins.RuntimeContext{RuntimeInstanceID: "33333333-3333-4333-8333-333333333333", PluginID: "com.example.academy.dashboard", PluginVersion: "1.0.0", ArtifactDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", WidgetID: "summary", WidgetType: plugins.TypeDashboardWidget}, Capabilities: []string{plugins.CapabilityBootstrap, plugins.CapabilityContextRead, plugins.CapabilityDashboardContextRead}}, nil
}

func TestDashboardWidgetLaunchUsesOnlyAuthenticatedPlacementIdentity(t *testing.T) {
	launches := &dashboardLaunchHTTPFake{}
	auth := &authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, dashboardWidgets: &dashboardWidgetAPI{}, dashboardWidgetRuntime: launches}
	router := authTestRouter(auth)
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	csrf := authTestCSRFToken().Value()
	placementID := "22222222-2222-4222-8222-222222222222"
	path := "/api/dashboard/widgets/" + placementID + "/widget-runtime"

	if response := authRequest(router, http.MethodPost, path, "", nil, csrf); response.Code != http.StatusUnauthorized || launches.calls != 0 {
		t.Fatalf("anonymous launch=%d calls=%d", response.Code, launches.calls)
	}
	if response := authRequest(router, http.MethodPost, path, `{"pluginId":"forged"}`, cookie, csrf); response.Code != http.StatusBadRequest || launches.calls != 0 {
		t.Fatalf("launch accepted client runtime identity: %d calls=%d", response.Code, launches.calls)
	}
	if response := authRequest(router, http.MethodPost, path, "", cookie); response.Code != http.StatusForbidden || launches.calls != 0 {
		t.Fatalf("launch bypassed CSRF: %d calls=%d", response.Code, launches.calls)
	}
	crossOrigin := httptest.NewRequest(http.MethodPost, path, nil)
	crossOrigin.Header.Set("Origin", "https://attacker.example")
	crossOrigin.Header.Set("X-CSRF-Token", csrf)
	crossOrigin.AddCookie(cookie)
	crossOriginResponse := httptest.NewRecorder()
	router.ServeHTTP(crossOriginResponse, crossOrigin)
	if crossOriginResponse.Code != http.StatusForbidden || launches.calls != 0 {
		t.Fatalf("launch bypassed Origin policy: %d calls=%d", crossOriginResponse.Code, launches.calls)
	}
	response := authRequest(router, http.MethodPost, path, "", cookie, csrf)
	if response.Code != http.StatusOK || launches.calls != 1 || launches.placementID != placementID || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("placement launch=%d calls=%d id=%q body=%s", response.Code, launches.calls, launches.placementID, response.Body.String())
	}
	for _, forbidden := range []string{"dashboard.widgets.manage", "plugins.manage", plugins.CapabilityCourseContextRead} {
		if containsString(response.Body.String(), forbidden) {
			t.Fatalf("launch leaked/granted %q: %s", forbidden, response.Body.String())
		}
	}
}

func containsString(value, fragment string) bool {
	return len(fragment) > 0 && len(value) >= len(fragment) && strings.Contains(value, fragment)
}
