package platform

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
	"github.com/prometheus/client_golang/prometheus"
)

type runtimeHTTPFake struct {
	disabled bool
}

func (f *runtimeHTTPFake) RuntimeOrigin() string { return "https://plugins.academy.example" }
func (f *runtimeHTTPFake) HostOrigin() string    { return "https://academy.example" }
func (f *runtimeHTTPFake) EntrypointResource(_ context.Context, _ plugins.ReleaseIdentity, widget string) (plugins.Entrypoint, error) {
	if widget != "timeline" {
		return plugins.Entrypoint{}, plugins.ErrNotFound
	}
	return plugins.Entrypoint{ID: widget, Type: plugins.TypeCourseWidget, Name: "Timeline", Resource: "resources/timeline.js"}, nil
}
func (f *runtimeHTTPFake) Resource(_ context.Context, _ plugins.ReleaseIdentity, path string) (plugins.InstalledResource, error) {
	resources := map[string]struct {
		digest  string
		content []byte
	}{
		"resources/timeline.js": {strings.Repeat("a", 64), []byte("export default {}")},
		"resources/widget.css":  {strings.Repeat("b", 64), []byte("body { color: red }")},
		"resources/icon.png":    {strings.Repeat("c", 64), []byte("not-a-real-png")},
	}
	resource, ok := resources[path]
	if !ok {
		return plugins.InstalledResource{}, plugins.ErrNotFound
	}
	return plugins.InstalledResource{Path: path, SHA256: resource.digest, Size: int64(len(resource.content)), Content: resource.content}, nil
}
func (f *runtimeHTTPFake) VerifyContextToken(token string) (plugins.RuntimeClaims, error) {
	if token != "runtime-token" {
		return plugins.RuntimeClaims{}, plugins.ErrInvalidRuntimeToken
	}
	return plugins.RuntimeClaims{Context: plugins.RuntimeContext{RuntimeInstanceID: "11111111-1111-4111-8111-111111111111", PluginID: "com.example.academy.timeline", PluginVersion: "1.0.0", ArtifactDigest: strings.Repeat("a", 64), WidgetID: "timeline", WidgetType: plugins.TypeCourseWidget}}, nil
}
func (f *runtimeHTTPFake) Refresh(_ context.Context, token string) (plugins.RuntimeLaunch, error) {
	if f.disabled {
		return plugins.RuntimeLaunch{}, plugins.ErrLaunchDenied
	}
	if token != "runtime-token" {
		return plugins.RuntimeLaunch{}, plugins.ErrInvalidRuntimeToken
	}
	return plugins.RuntimeLaunch{Token: "refreshed"}, nil
}

func runtimeTestRouter(fake *runtimeHTTPFake) http.Handler {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "runtime_test_requests_total"}, []string{"route", "method", "status_class"})
	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "runtime_test_duration_seconds"}, []string{"route", "method"})
	return newRouter(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), requests, latency, &authHTTP{pluginRuntime: &pluginRuntimeHTTP{service: fake}})
}

func runtimeRequest(router http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Host = "plugins.academy.example"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func TestPluginRuntimePageAndDeclaredResourceHeaders(t *testing.T) {
	router := runtimeTestRouter(&runtimeHTTPFake{})
	digest := strings.Repeat("a", 64)
	page := runtimeRequest(router, http.MethodGet, "/plugins/runtime/com.example.academy.timeline/1.0.0/"+digest+"/widgets/timeline")
	if page.Code != http.StatusOK {
		t.Fatalf("page status = %d", page.Code)
	}
	csp := page.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"default-src 'none'", "connect-src 'none'", "script-src 'self'", "img-src 'self' data:", "object-src 'none'", "frame-ancestors https://academy.example"} {
		if !strings.Contains(csp, directive) {
			t.Fatalf("CSP %q misses %q", csp, directive)
		}
	}
	if strings.Contains(csp, "*") || page.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unsafe page headers: %q", csp)
	}
	if !strings.Contains(page.Body.String(), "<title>Timeline</title>") {
		t.Fatalf("runtime title was not sourced from trusted entry metadata: %s", page.Body.String())
	}

	resource := runtimeRequest(router, http.MethodGet, "/plugins/runtime/com.example.academy.timeline/1.0.0/"+digest+"/resources/timeline.js")
	if resource.Code != http.StatusOK || resource.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || resource.Header().Get("Cross-Origin-Resource-Policy") != "same-origin" {
		t.Fatalf("resource response = %d %#v", resource.Code, resource.Header())
	}
	if !strings.Contains(resource.Header().Get("Content-Security-Policy"), "connect-src 'none'") {
		t.Fatalf("resource CSP = %q", resource.Header().Get("Content-Security-Policy"))
	}
	for path, expected := range map[string]string{
		"resources/widget.css": "text/css; charset=utf-8",
		"resources/icon.png":   "image/png",
	} {
		response := runtimeRequest(router, http.MethodGet, "/plugins/runtime/com.example.academy.timeline/1.0.0/"+digest+"/"+path)
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != expected || response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s response = %d %#v", path, response.Code, response.Header())
		}
	}
	undeclared := runtimeRequest(router, http.MethodGet, "/plugins/runtime/com.example.academy.timeline/1.0.0/"+digest+"/resources/../secret")
	if undeclared.Code != http.StatusNotFound {
		t.Fatalf("undeclared traversal status = %d", undeclared.Code)
	}
}

func TestRuntimeRoutesRequireDedicatedOriginAndBearerToken(t *testing.T) {
	router := runtimeTestRouter(&runtimeHTTPFake{})
	runtimeRoot := runtimeRequest(router, http.MethodGet, "/")
	if runtimeRoot.Code != http.StatusNotFound {
		t.Fatalf("runtime origin served Academy application: %d", runtimeRoot.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/plugin-runtime/context", nil)
	req.Host = "academy.example"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "ordinary-session"})
	wrongHost := httptest.NewRecorder()
	router.ServeHTTP(wrongHost, req)
	if wrongHost.Code != http.StatusNotFound {
		t.Fatalf("Academy origin reached runtime API: %d", wrongHost.Code)
	}

	withoutBearer := runtimeRequest(router, http.MethodGet, "/api/plugin-runtime/context")
	if withoutBearer.Code != http.StatusUnauthorized {
		t.Fatalf("session substitution status = %d", withoutBearer.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/plugin-runtime/context", nil)
	req.Host = "plugins.academy.example"
	req.Header.Set("Authorization", "Bearer runtime-token")
	ok := httptest.NewRecorder()
	router.ServeHTTP(ok, req)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), "runtimeInstanceId") {
		t.Fatalf("runtime context = %d %s", ok.Code, ok.Body.String())
	}
}

func TestRuntimeRefreshReflectsDisablement(t *testing.T) {
	fake := &runtimeHTTPFake{disabled: true}
	handler := &pluginRuntimeHTTP{service: fake}
	req := httptest.NewRequest(http.MethodPost, "/api/plugin-runtime/token/refresh", nil)
	req.Header.Set("Authorization", "Bearer runtime-token")
	response := httptest.NewRecorder()
	handler.handleRefresh(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("disabled refresh status = %d", response.Code)
	}
}
