package platform

import (
	"errors"
	"net/http"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

// Applied after authenticated(): session resolution creates the trusted actor,
// and unsafe requests pass CSRF before this capability decision.
func (h *authHTTP) requireCapability(capability identity.Capability, resource identity.Resource) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := currentAuthenticatedActor(r.Context())
			if !ok {
				problem(w, r, http.StatusUnauthorized, "Unauthenticated")
				return
			}
			if h.authorizer == nil {
				h.recordAuthorization(capability, "unavailable")
				problem(w, r, http.StatusInternalServerError, "Internal server error")
				return
			}
			err := h.authorizer.Authorize(r.Context(), actor, capability, resource)
			if errors.Is(err, identity.ErrAuthorizationDenied) {
				h.recordAuthorization(capability, "denied")
				problem(w, r, http.StatusForbidden, "Forbidden")
				return
			}
			if err != nil {
				h.recordAuthorization(capability, "unavailable")
				problem(w, r, http.StatusInternalServerError, "Internal server error")
				return
			}
			h.recordAuthorization(capability, "allowed")
			next.ServeHTTP(w, r)
		})
	}
}

func (h *authHTTP) recordAuthorization(capability identity.Capability, outcome string) {
	if h.authzMetrics != nil {
		h.authzMetrics.WithLabelValues(string(capability), outcome).Inc()
	}
}

func (*authHTTP) handleAdminStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
	}{Status: "ok"})
}
