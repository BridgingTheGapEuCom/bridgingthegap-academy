package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/web"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type requestIDKey struct{}

// Bound cookies and other request headers before net/http allocates the
// default 1 MiB header budget for each connection.
const maxRequestHeaderBytes = 16 * 1024

// Login parses a bounded body before source-rate admission. A total read
// deadline prevents clients from holding those pre-admission reads forever.
const maxRequestReadDuration = 10 * time.Second

func Serve(ctx context.Context, cfg Config, log *slog.Logger) error {
	origins, err := newOriginPolicy(cfg)
	if err != nil {
		return err
	}
	pool, err := postgres.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := postgres.SchemaCheck(ctx, pool); err != nil {
		return err
	}
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "btg_http_requests_total", Help: "HTTP requests by route, method, and status class."}, []string{"route", "method", "status_class"})
	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "btg_http_request_duration_seconds", Help: "HTTP request duration by route and method.", Buckets: prometheus.DefBuckets}, []string{"route", "method"})
	loginAttempts := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "btg_login_attempts_total", Help: "Validated login attempts by coarse outcome."}, []string{"outcome"})
	loginRejections := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "btg_login_rejections_total", Help: "Login admission rejections by coarse reason."}, []string{"reason"})
	authorizationDecisions := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "btg_authorization_decisions_total", Help: "Authorization decisions by static capability and coarse outcome."}, []string{"capability", "outcome"})
	registry.MustRegister(requests, latency, loginAttempts, loginRejections, authorizationDecisions)
	metricsListener, err := net.Listen("tcp", cfg.MetricsAddr)
	if err != nil {
		return err
	}
	defer func() { _ = metricsListener.Close() }()
	metricsServer := &http.Server{Handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: maxRequestHeaderBytes}
	go func() { _ = metricsServer.Serve(metricsListener) }()

	auth := &authHTTP{
		login:        NewPostgresLoginOrchestrator(pool, nil, nil),
		sessions:     identity.NewSessionService(identitypostgres.New(pool), nil, nil),
		csrf:         identity.NewSessionCSRFService(identitypostgres.New(pool), nil),
		origins:      origins,
		loginSources: newLoginSourceLimiter(defaultLoginRatePolicy, nil),
		loginWork:    newLoginWorkGuard(maxConcurrentLogins),
		loginMetrics: loginAttempts,
		loginRejects: loginRejections,
		authorizer:   identity.NewAuthorizationService(identitypostgres.New(pool)),
		courses:      courses.NewReadService(coursespostgres.New(pool)),
		authzMetrics: authorizationDecisions,
		cookieSecure: !cfg.DevelopmentHTTP,
		now:          time.Now,
	}
	router := newRouter(pool, log, requests, latency, auth)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: maxRequestReadDuration, MaxHeaderBytes: maxRequestHeaderBytes}
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()
	log.Info("server listening", "address", cfg.HTTPAddr, "metrics_address", cfg.MetricsAddr)
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = metricsServer.Shutdown(shutdownCtx)
		return server.Shutdown(shutdownCtx)
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newRouter(pool *pgxpool.Pool, log *slog.Logger, requests *prometheus.CounterVec, latency *prometheus.HistogramVec, auth *authHTTP) http.Handler {
	r := chi.NewRouter()
	r.Use(correlationID)
	r.Use(observe(log, requests, latency))
	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})
	r.Get("/health/ready", func(w http.ResponseWriter, req *http.Request) {
		checkCtx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
		defer cancel()
		if err := pool.Ping(checkCtx); err != nil {
			problem(w, req, http.StatusServiceUnavailable, "PostgreSQL unavailable")
			return
		}
		if err := postgres.SchemaCheck(checkCtx, pool); err != nil {
			problem(w, req, http.StatusServiceUnavailable, "Schema unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	if auth != nil {
		r.Route("/api", func(api chi.Router) {
			api.Use(auth.noStore)
			api.Use(auth.enforceOrigin)
			api.Post("/auth/login", auth.handleLogin)
			if auth.courses != nil {
				api.Get("/courses", auth.handleCourseList)
				api.Get("/courses/{slug}", auth.handleCourseCurrent)
				api.Get("/courses/{slug}/versions/{version}", auth.handleCourseVersion)
				api.Get("/courses/{slug}/versions/{version}/lessons/{lessonKey}", auth.handleLesson)
			}
			api.With(auth.resolveSession(false), auth.csrfProtection).Post("/auth/logout", auth.handleLogout)
			api.Group(func(protected chi.Router) {
				// Future cookie-authenticated APIs belong in this group: both
				// per-request resolution and unsafe-method CSRF are inherited.
				protected.Use(auth.authenticated)
				protected.Get("/auth/session", auth.handleSession)
				protected.With(auth.requireCapability(identity.CapabilityInstanceManage, identity.InstanceResource())).Get("/admin/status", auth.handleAdminStatus)
			})
			api.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				problem(w, req, http.StatusNotFound, "Not found")
			}))
		})
	}
	if auth == nil {
		r.Handle("/api/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			problem(w, req, http.StatusNotFound, "Not found")
		}))
	}
	r.Get("/*", web.Serve)
	return r
}

func correlationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bytes [16]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			http.Error(w, "request ID unavailable", http.StatusInternalServerError)
			return
		}
		id := hex.EncodeToString(bytes[:])
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func observedMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return method
	default:
		return "OTHER"
	}
}

func observe(log *slog.Logger, requests *prometheus.CounterVec, latency *prometheus.HistogramVec) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(wrapped, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			method := observedMethod(r.Method)
			statusClass := string(rune('0'+wrapped.status/100)) + "xx"
			requests.WithLabelValues(route, method, statusClass).Inc()
			latency.WithLabelValues(route, method).Observe(time.Since(start).Seconds())
			log.Info("http request", "request_id", r.Context().Value(requestIDKey{}), "method", method, "route", route, "status", wrapped.status, "duration_ms", time.Since(start).Milliseconds())
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func problem(w http.ResponseWriter, r *http.Request, status int, title string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "about:blank", "title": title, "status": status,
		"instance": strings.TrimSpace(r.URL.Path), "request_id": r.Context().Value(requestIDKey{}),
	})
}
