package platform

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
	"github.com/go-chi/chi/v5"
)

const runtimeBridgeJS = `const protocol='btg-widget-runtime',version=1;
const meta=(name)=>document.querySelector('meta[name="'+name+'"]')?.content||'';
const hostOrigin=meta('btg-host-origin'),entry=meta('btg-widget-entry');
const runtimeInstanceId=new URLSearchParams(location.hash.slice(1)).get('runtime')||'';
history.replaceState(null,'',location.pathname);
const send=(type,payload={})=>parent.postMessage({protocol,version,type,runtimeInstanceId,payload},hostOrigin);
addEventListener('message',async(event)=>{
  if(event.source!==parent||event.origin!==hostOrigin)return;
  const message=event.data;
  if(!message||message.protocol!==protocol||message.version!==version||message.runtimeInstanceId!==runtimeInstanceId||message.type!=='RUNTIME_INIT')return;
  try {
    const widget=await import(entry);
    if(typeof widget.initialize==='function')await widget.initialize(Object.freeze(message.payload));
    send('RUNTIME_INITIALIZED');
  } catch { send('RUNTIME_ERROR',{code:'initialization_failed'}); }
},{once:true});
send('WIDGET_READY');
`

type pluginRuntimeService interface {
	RuntimeOrigin() string
	HostOrigin() string
	EntrypointResource(context.Context, plugins.ReleaseIdentity, string) (plugins.Entrypoint, error)
	Resource(context.Context, plugins.ReleaseIdentity, string) (plugins.InstalledResource, error)
	VerifyContextToken(string) (plugins.RuntimeClaims, error)
	Refresh(context.Context, string) (plugins.RuntimeLaunch, error)
}

type courseRuntimeContextService interface {
	CourseContext(string) (plugins.CourseWidgetRuntimeContext, error)
}

type pluginRuntimeHTTP struct{ service pluginRuntimeService }

func (h *pluginRuntimeHTTP) requireRuntimeHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.servesHost(r.Host) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *pluginRuntimeHTTP) servesHost(host string) bool {
	expected, _ := url.Parse(h.service.RuntimeOrigin())
	return strings.EqualFold(host, expected.Host)
}

func (h *pluginRuntimeHTTP) securityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; connect-src 'none'; frame-ancestors 'none'")
}

func (h *pluginRuntimeHTTP) handlePage(w http.ResponseWriter, r *http.Request) {
	h.securityHeaders(w)
	release := plugins.ReleaseIdentity{PluginID: plugins.PluginID(chi.URLParam(r, "pluginId")), Version: chi.URLParam(r, "version"), ArtifactDigest: chi.URLParam(r, "digest")}
	entry, err := h.service.EntrypointResource(r.Context(), release, chi.URLParam(r, "widgetId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	resourceURL := "/plugins/runtime/" + url.PathEscape(string(release.PluginID)) + "/" + url.PathEscape(release.Version) + "/" + release.ArtifactDigest + "/resources/" + strings.TrimPrefix(entry.Resource, "resources/")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; media-src 'self'; connect-src 'none'; font-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-src 'none'; worker-src 'none'; manifest-src 'none'; frame-ancestors "+h.service.HostOrigin())
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct{ HostOrigin, Entry, WidgetName string }{h.service.HostOrigin(), resourceURL, entry.Name}
	const page = `<!doctype html><html><head><meta charset="utf-8"><title>{{.WidgetName}}</title><meta name="referrer" content="no-referrer"><meta name="btg-host-origin" content="{{.HostOrigin}}"><meta name="btg-widget-entry" content="{{.Entry}}"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="widget-root"></div><script type="module" src="/plugins/runtime/bridge-v1.js"></script></body></html>`
	_ = template.Must(template.New("runtime").Parse(page)).Execute(w, data)
}

func (h *pluginRuntimeHTTP) handleBridge(w http.ResponseWriter, r *http.Request) {
	h.securityHeaders(w)
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"bridge-v1"`)
	_, _ = w.Write([]byte(runtimeBridgeJS))
}

func (h *pluginRuntimeHTTP) handleResource(w http.ResponseWriter, r *http.Request) {
	h.securityHeaders(w)
	release := plugins.ReleaseIdentity{PluginID: plugins.PluginID(chi.URLParam(r, "pluginId")), Version: chi.URLParam(r, "version"), ArtifactDigest: chi.URLParam(r, "digest")}
	path := "resources/" + strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	resource, err := h.service.Resource(r.Context(), release, path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := controlledPluginMediaType(resource.Path)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"sha256-`+resource.SHA256+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resource.Content)
}

func controlledPluginMediaType(resourcePath string) string {
	lower := strings.ToLower(resourcePath)
	switch {
	case strings.HasSuffix(lower, ".js"), strings.HasSuffix(lower, ".mjs"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(lower, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".avif"):
		return "image/avif"
	case strings.HasSuffix(lower, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(lower, ".mp3"):
		return "audio/mpeg"
	case strings.HasSuffix(lower, ".ogg"):
		return "audio/ogg"
	case strings.HasSuffix(lower, ".wav"):
		return "audio/wav"
	case strings.HasSuffix(lower, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(lower, ".webm"):
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}

func bearerToken(r *http.Request) (string, bool) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(values[0], "Bearer ")
	return token, token != "" && strings.TrimSpace(token) == token
}

func (h *pluginRuntimeHTTP) handleContext(w http.ResponseWriter, r *http.Request) {
	h.securityHeaders(w)
	w.Header().Set("Cache-Control", "no-store")
	token, ok := bearerToken(r)
	if !ok {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return
	}
	if course, ok := h.service.(courseRuntimeContextService); ok {
		context, err := course.CourseContext(token)
		if err == nil {
			writeJSON(w, http.StatusOK, context)
			return
		}
		if !errors.Is(err, plugins.ErrRuntimeCapabilityDenied) {
			problem(w, r, http.StatusUnauthorized, "Unauthenticated")
			return
		}
	}
	claims, err := h.service.VerifyContextToken(token)
	if err != nil {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return
	}
	writeJSON(w, http.StatusOK, claims.Context)
}

func (h *pluginRuntimeHTTP) handleRefresh(w http.ResponseWriter, r *http.Request) {
	h.securityHeaders(w)
	w.Header().Set("Cache-Control", "no-store")
	if r.ContentLength > 0 {
		problem(w, r, http.StatusBadRequest, "Request body is not allowed")
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return
	}
	launch, err := h.service.Refresh(r.Context(), token)
	if err != nil {
		if errors.Is(err, plugins.ErrLaunchDenied) || errors.Is(err, plugins.ErrNotFound) {
			problem(w, r, http.StatusForbidden, "Runtime unavailable")
			return
		}
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(launch)
}
