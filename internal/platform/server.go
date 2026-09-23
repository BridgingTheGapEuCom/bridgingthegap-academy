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

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	assessmentspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	assetslocal "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/localstorage"
	assetspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/community"
	communitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/community/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	credentialspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/postgres"
	progresspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/progress/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/web"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
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
	assetStorage, err := assetslocal.New(cfg.AssetStoragePath)
	if err != nil {
		return err
	}
	defer func() { _ = assetStorage.Close() }()
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

	authoringRepository := authoringpostgres.New(pool).WithAssessmentSnapshotReader(func(tx pgx.Tx) authoringpostgres.AssessmentSnapshotReader {
		return assessmentspostgres.New(tx)
	})
	authoringAuthorizer := authoring.NewAuthorizationService(authoringRepository)
	assetRepository := assetspostgres.New(pool)
	assessmentRepository := assessmentspostgres.New(pool)
	assetIngestion, err := assets.NewIngestionService(assetRepository, assetStorage, cfg.AssetMaxBytes)
	if err != nil {
		return err
	}
	coursesRepository := coursespostgres.New(pool)
	certificateRepository := credentialspostgres.New(pool)
	certificateIssuance, err := credentials.NewCertificateIssuanceService(certificateRepository, coursesRepository, progresspostgres.New(pool), cfg.CertificateIssuer, time.Now)
	if err != nil {
		return err
	}
	publicationAssetResolver := authoring.NewAssetPublicationResolver(assetRepository)
	publicationService := authoring.NewPublicationServiceWithAssets(authoringRepository, publicationAssetResolver, courses.NewCourseVersionStore(coursesRepository), authoringRepository)
	auth := &authHTTP{
		login:                       NewPostgresLoginOrchestrator(pool, nil, nil),
		sessions:                    identity.NewSessionService(identitypostgres.New(pool), nil, nil),
		csrf:                        identity.NewSessionCSRFService(identitypostgres.New(pool), nil),
		origins:                     origins,
		loginSources:                newLoginSourceLimiter(defaultLoginRatePolicy, nil),
		loginWork:                   newLoginWorkGuard(maxConcurrentLogins),
		loginMetrics:                loginAttempts,
		loginRejects:                loginRejections,
		authorizer:                  identity.NewAuthorizationService(identitypostgres.New(pool)),
		courses:                     courses.NewReadService(coursesRepository),
		publishedCourses:            courses.NewPublishedReadService(coursesRepository),
		publishedAssets:             courses.NewPublishedAssetReadService(coursesRepository),
		publishedCatalog:            courses.NewPublishedCatalogService(coursesRepository),
		assetStorage:                assetStorage,
		authoring:                   authoring.NewReadService(authoringRepository, authoringAuthorizer),
		authoringCreation:           authoring.NewDraftCreationService(authoringRepository),
		authoringMutations:          authoring.NewDraftMutationService(authoringRepository, authoringAuthorizer),
		authoringStructureMutations: authoring.NewModuleMutationService(authoringRepository, authoringAuthorizer),
		authoringLessonMutations:    authoring.NewLessonMutationService(authoringRepository, authoringAuthorizer),
		authoringLessonContent:      authoring.NewLessonContentMutationService(authoringRepository, authoringAuthorizer),
		authoringMemberships:        authoring.NewMembershipMutationService(authoringRepository, authoringAuthorizer),
		authoringReviews:            newAuthoringReviewApplicationService(cfg, authoringRepository, authoringAuthorizer),
		authoringPublicationStatus:  authoring.NewReviewPublicationStatusServiceWithAssets(authoringRepository, authoringAuthorizer, publicationAssetResolver),
		authoringPublications:       authoring.NewPublicationApplicationService(publicationService, authoringAuthorizer, time.Now),
		authoringAssetUploads:       authoring.NewAssetUploadService(assetIngestion, authoringAuthorizer),
		authoringAssets:             authoring.NewAssetListService(assetRepository, authoringAuthorizer),
		authoringAssessments:        authoring.NewAssessmentManagementService(assessmentRepository, authoringAuthorizer),
		learnerAttempts:             assessments.NewLearnerAttemptService(assessmentRepository, coursesRepository, time.Now),
		community:                   community.NewService(communitypostgres.New(pool), coursesRepository),
		certificateIssuance:         certificateIssuance,
		certificates:                certificateRepository,
		assetMaxBytes:               cfg.AssetMaxBytes,
		authzMetrics:                authorizationDecisions,
		cookieSecure:                !cfg.DevelopmentHTTP,
		now:                         time.Now,
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

// newAuthoringReviewApplicationService keeps independent-review configuration
// at composition time. The Review application remains responsible for the
// decision policy itself; transports never receive deployment policy details.
func newAuthoringReviewApplicationService(cfg Config, repository authoring.ReviewRepository, authorizer authoring.Authorizer) *authoring.ReviewApplicationService {
	return authoring.NewReviewApplicationServiceWithDecisionPolicy(repository, authorizer, authoring.NewReviewDecisionPolicy(cfg.RequireIndependentReview))
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
			if auth.publishedCatalog != nil {
				api.Get("/courses/catalog", auth.handlePublishedCourseCatalog)
			}
			if auth.courses != nil {
				api.Get("/courses", auth.handleCourseList)
				api.Get("/courses/{slug}", auth.handleCourseCurrent)
				api.Get("/courses/{slug}/versions/{version}", auth.handleCourseVersion)
				api.Get("/courses/{slug}/versions/{version}/lessons/{lessonKey}", auth.handleLesson)
			}
			if auth.publishedCourses != nil {
				// The legacy public Courses routes are slug-addressed. The ID
				// namespace keeps those stable while exposing complete immutable
				// M4.5 publications through exact Courses-owned IDs.
				api.Get("/courses/by-id/{courseId}/versions/{version}", auth.handlePublishedCourseVersion)
				api.Get("/courses/by-id/{courseId}/latest", auth.handleLatestPublishedCourseVersion)
			}
			if auth.certificates != nil {
				api.Get("/public/certificates/{certificateId}", auth.handlePublicCertificate)
			}
			if auth.publishedAssets != nil && auth.assetStorage != nil {
				api.Get("/courses/by-id/{courseId}/versions/{version}/assets/{assetKey}", auth.handlePublishedCourseAsset)
				api.Head("/courses/by-id/{courseId}/versions/{version}/assets/{assetKey}", auth.handlePublishedCourseAsset)
			}
			api.With(auth.resolveSession(false), auth.csrfProtection).Post("/auth/logout", auth.handleLogout)
			api.Group(func(protected chi.Router) {
				// Future cookie-authenticated APIs belong in this group: both
				// per-request resolution and unsafe-method CSRF are inherited.
				protected.Use(auth.authenticated)
				protected.Get("/auth/session", auth.handleSession)
				if auth.certificateIssuance != nil && auth.certificates != nil {
					protected.Post("/courses/by-id/{courseId}/versions/{version}/certificate", auth.handleCertificateIssue)
					protected.Get("/learner/certificates/{certificateId}", auth.handleLearnerCertificateGet)
				}
				if auth.learnerAttempts != nil {
					protected.Post("/courses/by-id/{courseId}/versions/{version}/assessments/{assessmentKey}/attempts", auth.handleLearnerAttemptCreate)
					protected.Get("/learner/assessment-attempts/{attemptId}", auth.handleLearnerAttemptGet)
					protected.Put("/learner/assessment-attempts/{attemptId}", auth.handleLearnerAttemptUpdate)
					protected.Post("/learner/assessment-attempts/{attemptId}/submit", auth.handleLearnerAttemptSubmit)
				}
				if auth.community != nil {
					protected.Get("/courses/by-id/{courseId}/community/moderation", auth.handleCommunityModerationProbe)
					protected.Get("/courses/by-id/{courseId}/community/moderation/threads", auth.handleCommunityModeratorThreads)
					protected.Get("/courses/by-id/{courseId}/community/moderation/threads/{threadId}", auth.handleCommunityModeratorThread)
					protected.Get("/courses/by-id/{courseId}/community", auth.handleCommunity)
					protected.Get("/courses/by-id/{courseId}/community/threads", auth.handleCommunityThreads)
					protected.Post("/courses/by-id/{courseId}/community/threads", auth.handleCommunityCreateThread)
					protected.Get("/courses/by-id/{courseId}/community/threads/{threadId}", auth.handleCommunityThread)
					protected.Post("/courses/by-id/{courseId}/community/threads/{threadId}/posts", auth.handleCommunityCreatePost)
					protected.Post("/courses/by-id/{courseId}/community/threads/{threadId}/{action:hide|unhide}", auth.handleCommunityModerateThread)
					protected.Post("/courses/by-id/{courseId}/community/threads/{threadId}/posts/{postId}/{action:hide|unhide}", auth.handleCommunityModeratePost)
				}
				protected.With(auth.requireCapability(identity.CapabilityInstanceManage, identity.InstanceResource())).Get("/admin/status", auth.handleAdminStatus)
				if auth.authoring != nil {
					protected.Get("/authoring/drafts", auth.handleAuthoringDraftList)
					protected.Get("/authoring/drafts/{draftId}", auth.handleAuthoringDraft)
					protected.Get("/authoring/drafts/{draftId}/workspace", auth.handleAuthoringWorkspace)
					protected.Get("/authoring/drafts/{draftId}/members", auth.handleAuthoringMembers)
					protected.Get("/authoring/drafts/{draftId}/structure", auth.handleAuthoringStructure)
					protected.Get("/authoring/drafts/{draftId}/lessons/{lessonId}", auth.handleAuthoringLesson)
				}
				if auth.authoringAssets != nil {
					protected.Get("/authoring/drafts/{draftId}/assets", auth.handleAuthoringAssetList)
				}
				if auth.authoringAssessments != nil {
					protected.Get("/authoring/drafts/{draftId}/assessments", auth.handleAuthoringAssessmentList)
					protected.Post("/authoring/drafts/{draftId}/assessments", auth.handleAuthoringAssessmentCreate)
					protected.Get("/authoring/drafts/{draftId}/assessments/{assessmentId}", auth.handleAuthoringAssessmentGet)
					protected.Put("/authoring/drafts/{draftId}/assessments/{assessmentId}", auth.handleAuthoringAssessmentUpdate)
				}
				if auth.authoringCreation != nil {
					protected.Post("/authoring/drafts", auth.handleAuthoringDraftCreate)
				}
				if auth.authoringMutations != nil {
					protected.Patch("/authoring/drafts/{draftId}", auth.handleAuthoringDraftUpdate)
				}
				if auth.authoringStructureMutations != nil {
					protected.Post("/authoring/drafts/{draftId}/modules", auth.handleAuthoringModuleCreate)
					protected.Put("/authoring/drafts/{draftId}/modules/order", auth.handleAuthoringModuleReorder)
					protected.Patch("/authoring/drafts/{draftId}/modules/{moduleId}", auth.handleAuthoringModuleUpdate)
					protected.Delete("/authoring/drafts/{draftId}/modules/{moduleId}", auth.handleAuthoringModuleDelete)
				}
				if auth.authoringLessonMutations != nil {
					protected.Post("/authoring/drafts/{draftId}/modules/{moduleId}/lessons", auth.handleAuthoringLessonCreate)
					protected.Put("/authoring/drafts/{draftId}/lessons/order", auth.handleAuthoringLessonReorder)
					protected.Patch("/authoring/drafts/{draftId}/lessons/{lessonId}", auth.handleAuthoringLessonUpdate)
					protected.Put("/authoring/drafts/{draftId}/lessons/{lessonId}/prerequisites", auth.handleAuthoringLessonPrerequisites)
					protected.Delete("/authoring/drafts/{draftId}/lessons/{lessonId}", auth.handleAuthoringLessonDelete)
				}
				if auth.authoringLessonContent != nil {
					protected.Put("/authoring/drafts/{draftId}/lessons/{lessonId}/content", auth.handleAuthoringLessonContent)
				}
				if auth.authoringMemberships != nil {
					protected.Post("/authoring/drafts/{draftId}/members", auth.handleAuthoringMemberAdd)
					protected.Patch("/authoring/drafts/{draftId}/members/{userId}", auth.handleAuthoringMemberRole)
					protected.Delete("/authoring/drafts/{draftId}/members/{userId}", auth.handleAuthoringMemberRevoke)
				}
				if auth.authoringReviews != nil {
					protected.Post("/authoring/drafts/{draftId}/reviews", auth.handleAuthoringReviewSubmit)
					protected.Get("/authoring/drafts/{draftId}/reviews", auth.handleAuthoringReviewHistory)
					protected.Get("/authoring/drafts/{draftId}/reviews/active", auth.handleAuthoringReviewActive)
					protected.Get("/authoring/drafts/{draftId}/reviews/latest", auth.handleAuthoringReviewLatest)
					protected.Get("/authoring/drafts/{draftId}/reviews/{reviewId}", auth.handleAuthoringReview)
					protected.Post("/authoring/drafts/{draftId}/reviews/{reviewId}/approve", auth.handleAuthoringReviewApprove)
					protected.Post("/authoring/drafts/{draftId}/reviews/{reviewId}/request-changes", auth.handleAuthoringReviewRequestChanges)
				}
				if auth.authoringPublications != nil {
					protected.Post("/authoring/drafts/{draftId}/reviews/{reviewId}/publish", auth.handleAuthoringReviewPublish)
				}
				if auth.authoringAssetUploads != nil {
					protected.Post("/authoring/drafts/{draftId}/assets", auth.handleAuthoringAssetUpload)
				}
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
	problemCode(w, r, status, title, "")
}

func problemCode(w http.ResponseWriter, r *http.Request, status int, title, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	response := map[string]any{
		"type": "about:blank", "title": title, "status": status,
		"instance": strings.TrimSpace(r.URL.Path), "request_id": r.Context().Value(requestIDKey{}),
	}
	if code != "" {
		response["code"] = code
	}
	_ = json.NewEncoder(w).Encode(response)
}
