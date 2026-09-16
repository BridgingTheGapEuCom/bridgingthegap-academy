package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/prometheus/client_golang/prometheus"
)

const sessionCookieName = "btg_session"
const maxLoginBodyBytes = 8 * 1024
const maxAuthoringDraftMetadataBodyBytes = 128 * 1024
const maxAuthoringModuleBodyBytes = 64 * 1024
const maxAuthoringLessonBodyBytes = 256 * 1024

// A complete layout of 1,000 Modules and 10,000 UUID Lessons fits within 1 MiB.
const maxAuthoringLessonOrderBodyBytes = 1 << 20

// Canonical LessonContent is capped at 1 MiB. This permits its complete
// semantic document plus a small mutation envelope, never an unbounded editor
// payload.
const maxAuthoringLessonContentBodyBytes = courses.MaxLessonContentBytes + 16*1024
const maxAuthoringMembershipBodyBytes = 16 * 1024
const maxCookieHeaderBytes = 8 * 1024
const maxSessionCookieValueBytes = 128

type loginLogoutService interface {
	LoginWithPassword(context.Context, string, []byte, string) (LoginResult, error)
	Logout(context.Context, identity.ResolvedSession, string) error
}

type sessionResolver interface {
	ResolveSession(context.Context, string) (identity.ResolvedSession, error)
}

type sessionCSRF interface {
	TokenForSession(context.Context, identity.ResolvedSession) (identity.CSRFToken, error)
	Validate(context.Context, identity.ResolvedSession, string) error
}

type authHTTP struct {
	login                       loginLogoutService
	sessions                    sessionResolver
	csrf                        sessionCSRF
	origins                     originPolicy
	loginSources                *loginSourceLimiter
	loginWork                   *loginWorkGuard
	loginMetrics                *prometheus.CounterVec
	loginRejects                *prometheus.CounterVec
	authorizer                  identity.Authorizer
	authzMetrics                *prometheus.CounterVec
	courses                     *courses.ReadService
	authoring                   *authoring.ReadService
	authoringMutations          *authoring.DraftMutationService
	authoringStructureMutations *authoring.ModuleMutationService
	authoringLessonMutations    *authoring.LessonMutationService
	authoringLessonContent      *authoring.LessonContentMutationService
	authoringMemberships        *authoring.MembershipMutationService
	authoringReviews            *authoring.ReviewApplicationService
	cookieSecure                bool
	now                         func() time.Time
}

type resolvedSessionKey struct{}
type authenticatedActorKey struct{}

func currentResolvedSession(ctx context.Context) (identity.ResolvedSession, bool) {
	session, ok := ctx.Value(resolvedSessionKey{}).(identity.ResolvedSession)
	return session, ok && session.SessionID() != "" && session.UserID() != ""
}

func currentAuthenticatedActor(ctx context.Context) (identity.AuthenticatedActor, bool) {
	actor, ok := ctx.Value(authenticatedActorKey{}).(identity.AuthenticatedActor)
	return actor, ok && actor.UserID() != "" && actor.SessionID() != ""
}

func trustedOperationID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// Only this middleware places a resolved Identity session into request context.
// The optional form lets logout clear an absent/invalid cookie idempotently.
func (h *authHTTP) resolveSession(required bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, found := uniqueSessionCookie(r)
			if !found || cookie == "" {
				if required {
					problem(w, r, http.StatusUnauthorized, "Unauthenticated")
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			if h.sessions == nil {
				problem(w, r, http.StatusInternalServerError, "Internal server error")
				return
			}
			current, err := h.sessions.ResolveSession(r.Context(), cookie)
			if errors.Is(err, identity.ErrInvalidSession) {
				if required {
					problem(w, r, http.StatusUnauthorized, "Unauthenticated")
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			if err != nil {
				problem(w, r, http.StatusInternalServerError, "Internal server error")
				return
			}
			if current.SessionID() == "" || current.UserID() == "" {
				problem(w, r, http.StatusInternalServerError, "Internal server error")
				return
			}
			actor, err := identity.ActorFromResolvedSession(current)
			if err != nil {
				problem(w, r, http.StatusInternalServerError, "Internal server error")
				return
			}
			trusted := context.WithValue(r.Context(), resolvedSessionKey{}, current)
			trusted = context.WithValue(trusted, authenticatedActorKey{}, actor)
			next.ServeHTTP(w, r.WithContext(trusted))
		})
	}
}

// Duplicate names are treated as invalid rather than selecting an arbitrary
// cookie when a browser sends cookies with different Domain/Path scopes.
func uniqueSessionCookie(r *http.Request) (string, bool) {
	cookieBytes := 0
	for _, header := range r.Header.Values("Cookie") {
		if len(header) > maxCookieHeaderBytes-cookieBytes {
			return "", false
		}
		cookieBytes += len(header)
	}
	var value string
	found := false
	for _, cookie := range r.Cookies() {
		if cookie.Name != sessionCookieName {
			continue
		}
		if found {
			return "", false
		}
		if len(cookie.Value) > maxSessionCookieValueBytes {
			return "", false
		}
		value, found = cookie.Value, true
	}
	return value, found
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authenticatedSessionResponse struct {
	Authenticated bool            `json:"authenticated"`
	UserID        identity.UserID `json:"user_id"`
	ExpiresAt     time.Time       `json:"expires_at"`
	CSRFToken     string          `json:"csrf_token"`
}

func (h *authHTTP) noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (h *authHTTP) enforceOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.origins.allowsRequest(r) {
			problem(w, r, http.StatusForbidden, "Forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Mount this after session resolution on every cookie-authenticated route.
// Safe methods pass without a token; mutations require the session-bound one.
func (h *authHTTP) csrfProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !mutatingMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		current, ok := currentResolvedSession(r.Context())
		if !ok {
			// Optional logout with no valid session has no server-side mutation.
			next.ServeHTTP(w, r)
			return
		}
		if h.csrf == nil {
			problem(w, r, http.StatusInternalServerError, "Internal server error")
			return
		}
		values := r.Header.Values("X-CSRF-Token")
		if len(values) != 1 {
			problem(w, r, http.StatusForbidden, "Forbidden")
			return
		}
		err := h.csrf.Validate(r.Context(), current, values[0])
		if errors.Is(err, identity.ErrInvalidCSRFToken) {
			problem(w, r, http.StatusForbidden, "Forbidden")
			return
		}
		if errors.Is(err, identity.ErrInvalidSession) {
			problem(w, r, http.StatusUnauthorized, "Unauthenticated")
			return
		}
		if err != nil {
			problem(w, r, http.StatusInternalServerError, "Internal server error")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *authHTTP) authenticated(next http.Handler) http.Handler {
	return h.resolveSession(true)(h.csrfProtection(next))
}

func (h *authHTTP) handleLogin(w http.ResponseWriter, r *http.Request) {
	operationID := trustedOperationID(r.Context())
	if h.login == nil || h.now == nil || operationID == "" {
		problem(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != "application/json" {
		problem(w, r, http.StatusBadRequest, "Invalid request")
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes))
	decoder.DisallowUnknownFields()
	var input loginRequest
	if err := decoder.Decode(&input); err != nil || input.Email == "" || input.Password == "" {
		problem(w, r, http.StatusBadRequest, "Invalid request")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		problem(w, r, http.StatusBadRequest, "Invalid request")
		return
	}
	password := []byte(input.Password)
	input.Password = ""
	defer clear(password)
	if h.loginSources == nil || h.loginWork == nil {
		h.recordLoginOutcome("unavailable")
		problem(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}
	allowed, retryAfter := h.loginSources.Allow(r.RemoteAddr)
	if !allowed {
		if retryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
		}
		h.recordLoginRejection("source_rate")
		problem(w, r, http.StatusTooManyRequests, "Too many requests")
		return
	}
	if !h.loginWork.TryAcquire() {
		h.recordLoginRejection("work_saturated")
		problem(w, r, http.StatusTooManyRequests, "Too many requests")
		return
	}
	defer h.loginWork.Release()
	result, err := h.login.LoginWithPassword(r.Context(), input.Email, password, operationID)
	if errors.Is(err, identity.ErrInvalidCredentials) {
		h.recordLoginOutcome("invalid_credentials")
		problem(w, r, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	if err != nil || result.UserID == "" || result.Token.Value() == "" || result.CSRFToken.Value() == "" || !result.ExpiresAt.After(h.now()) {
		h.recordLoginOutcome("unavailable")
		problem(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}
	h.recordLoginOutcome("success")
	setSessionCookie(w, result.Token.Value(), result.ExpiresAt, h.cookieSecure, h.now())
	writeJSON(w, http.StatusOK, authenticatedSessionResponse{Authenticated: true, UserID: result.UserID, ExpiresAt: result.ExpiresAt, CSRFToken: result.CSRFToken.Value()})
}

func (h *authHTTP) recordLoginOutcome(outcome string) {
	if h.loginMetrics != nil {
		h.loginMetrics.WithLabelValues(outcome).Inc()
	}
}

func (h *authHTTP) recordLoginRejection(reason string) {
	h.recordLoginOutcome("rate_limited")
	if h.loginRejects != nil {
		h.loginRejects.WithLabelValues(reason).Inc()
	}
}

func (h *authHTTP) handleSession(w http.ResponseWriter, r *http.Request) {
	current, ok := currentResolvedSession(r.Context())
	if !ok {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return
	}
	if h.csrf == nil {
		problem(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}
	token, err := h.csrf.TokenForSession(r.Context(), current)
	if errors.Is(err, identity.ErrInvalidSession) {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return
	}
	if err != nil || token.Value() == "" {
		problem(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, authenticatedSessionResponse{Authenticated: true, UserID: current.UserID(), ExpiresAt: current.ExpiresAt(), CSRFToken: token.Value()})
}

func (h *authHTTP) handleLogout(w http.ResponseWriter, r *http.Request) {
	current, ok := currentResolvedSession(r.Context())
	if ok {
		operationID := trustedOperationID(r.Context())
		if h.login == nil || operationID == "" {
			problem(w, r, http.StatusInternalServerError, "Internal server error")
			return
		}
		if err := h.login.Logout(r.Context(), current, operationID); err != nil {
			problem(w, r, http.StatusInternalServerError, "Internal server error")
			return
		}
	}
	clearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// SameSite=Lax remains defense in depth beside Origin and session-bound CSRF.
func setSessionCookie(w http.ResponseWriter, token string, expiry time.Time, secure bool, now time.Time) {
	seconds := int(expiry.Sub(now).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, Expires: expiry.UTC(), MaxAge: seconds})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, Expires: time.Unix(0, 0).UTC(), MaxAge: -1})
}
