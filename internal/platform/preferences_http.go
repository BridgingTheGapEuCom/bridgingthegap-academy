package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/preferences"
)

const maxUserPreferencesBodyBytes int64 = 4 * 1024

type userPreferencesService interface {
	Get(context.Context, identity.UserID) (preferences.UserPreferences, error)
	Update(context.Context, identity.UserID, int64, preferences.Update) (preferences.UserPreferences, error)
}

type userPreferencesDTO struct {
	ApplicationLocale *preferences.ApplicationLocale `json:"applicationLocale"`
	Appearance        preferences.Appearance         `json:"appearance"`
	Revision          int64                          `json:"revision"`
	UpdatedAt         *time.Time                     `json:"updatedAt"`
}

type updateUserPreferencesRequest struct {
	ApplicationLocale json.RawMessage `json:"applicationLocale"`
	Appearance        *string         `json:"appearance"`
	Revision          *int64          `json:"revision"`
}

func (h *authHTTP) handleUserPreferencesGet(w http.ResponseWriter, r *http.Request) {
	current, ok := currentResolvedSession(r.Context())
	if !ok {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return
	}
	if h.preferences == nil {
		problem(w, r, http.StatusServiceUnavailable, "Preferences unavailable")
		return
	}
	value, err := h.preferences.Get(r.Context(), current.UserID())
	if err != nil {
		userPreferencesProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, userPreferencesResponse(value))
}

func (h *authHTTP) handleUserPreferencesUpdate(w http.ResponseWriter, r *http.Request) {
	current, ok := currentResolvedSession(r.Context())
	if !ok {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return
	}
	if h.preferences == nil {
		problem(w, r, http.StatusServiceUnavailable, "Preferences unavailable")
		return
	}
	revision, update, err := decodeUserPreferencesUpdate(w, r)
	if err != nil {
		problemCode(w, r, http.StatusBadRequest, "Invalid preferences request", "invalid_preferences")
		return
	}
	value, err := h.preferences.Update(r.Context(), current.UserID(), revision, update)
	if err != nil {
		userPreferencesProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, userPreferencesResponse(value))
}

func userPreferencesResponse(value preferences.UserPreferences) userPreferencesDTO {
	return userPreferencesDTO{ApplicationLocale: value.ApplicationLocale, Appearance: value.Appearance, Revision: value.Revision, UpdatedAt: value.UpdatedAt}
}

func decodeUserPreferencesUpdate(w http.ResponseWriter, r *http.Request) (int64, preferences.Update, error) {
	var request updateUserPreferencesRequest
	if err := decodeAuthoringBody(w, r, maxUserPreferencesBodyBytes, &request); err != nil || request.Appearance == nil || request.Revision == nil || len(request.ApplicationLocale) == 0 || *request.Revision < 0 {
		return 0, preferences.Update{}, preferences.ErrInvalidPreferences
	}
	appearance, err := preferences.ParseAppearance(*request.Appearance)
	if err != nil {
		return 0, preferences.Update{}, err
	}
	var locale *preferences.ApplicationLocale
	if string(request.ApplicationLocale) != "null" {
		var raw string
		if err := json.Unmarshal(request.ApplicationLocale, &raw); err != nil {
			return 0, preferences.Update{}, preferences.ErrInvalidPreferences
		}
		parsed, err := preferences.ParseApplicationLocale(raw)
		if err != nil {
			return 0, preferences.Update{}, err
		}
		locale = &parsed
	}
	return *request.Revision, preferences.Update{ApplicationLocale: locale, Appearance: appearance}, nil
}

func userPreferencesProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, preferences.ErrInvalidPreferences):
		problemCode(w, r, http.StatusBadRequest, "Invalid preferences request", "invalid_preferences")
	case errors.Is(err, preferences.ErrRevisionMismatch):
		problemCode(w, r, http.StatusConflict, "Preferences changed elsewhere", "preferences_revision_mismatch")
	default:
		problem(w, r, http.StatusInternalServerError, "Preferences unavailable")
	}
}
