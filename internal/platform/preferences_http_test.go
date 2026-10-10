package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/preferences"
)

type preferencesHTTPFake struct {
	current  preferences.UserPreferences
	getUser  identity.UserID
	updateID identity.UserID
	err      error
}

func (f *preferencesHTTPFake) Get(_ context.Context, userID identity.UserID) (preferences.UserPreferences, error) {
	f.getUser = userID
	return f.current, f.err
}
func (f *preferencesHTTPFake) Update(_ context.Context, userID identity.UserID, revision int64, update preferences.Update) (preferences.UserPreferences, error) {
	f.updateID = userID
	if f.err != nil {
		return preferences.UserPreferences{}, f.err
	}
	f.current.UserID, f.current.Revision, f.current.ApplicationLocale, f.current.Appearance = userID, revision+1, update.ApplicationLocale, update.Appearance
	return f.current, nil
}

func TestUserPreferencesHTTPUsesCurrentSessionOnly(t *testing.T) {
	service := &preferencesHTTPFake{current: preferences.UserPreferences{UserID: loginTestUser, Appearance: preferences.AppearanceSystem, Revision: 0}}
	router := authTestRouter(&authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, preferences: service})
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}

	response := authRequest(router, http.MethodGet, "/api/me/preferences", "", cookie)
	if response.Code != http.StatusOK || service.getUser != loginTestUser {
		t.Fatalf("GET status=%d user=%q", response.Code, service.getUser)
	}
	var current userPreferencesDTO
	if err := json.Unmarshal(response.Body.Bytes(), &current); err != nil || current.ApplicationLocale != nil || current.Appearance != preferences.AppearanceSystem || current.Revision != 0 || current.UpdatedAt != nil {
		t.Fatalf("GET body=%s err=%v", response.Body.String(), err)
	}

	response = authRequest(router, http.MethodPut, "/api/me/preferences", `{"applicationLocale":"en","appearance":"DARK","revision":0}`, cookie, authTestCSRFToken().Value())
	if response.Code != http.StatusOK || service.updateID != loginTestUser {
		t.Fatalf("PUT status=%d user=%q", response.Code, service.updateID)
	}
	if err := json.Unmarshal(response.Body.Bytes(), &current); err != nil || current.ApplicationLocale == nil || *current.ApplicationLocale != preferences.ApplicationLocaleEnglish || current.Appearance != preferences.AppearanceDark || current.Revision != 1 {
		t.Fatalf("PUT body=%s err=%v", response.Body.String(), err)
	}

	if response = authRequest(router, http.MethodPut, "/api/me/preferences", `{"applicationLocale":null,"appearance":"SYSTEM","revision":1,"userId":"other"}`, cookie, authTestCSRFToken().Value()); response.Code != http.StatusBadRequest {
		t.Fatalf("request body may not target another user: status=%d", response.Code)
	}
	if response = authRequest(router, http.MethodGet, "/api/me/preferences", "", nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET status=%d", response.Code)
	}
}

func TestUserPreferencesHTTPRejectsInvalidAndStaleUpdates(t *testing.T) {
	service := &preferencesHTTPFake{current: preferences.UserPreferences{UserID: loginTestUser, Appearance: preferences.AppearanceSystem, Revision: 1}}
	router := authTestRouter(&authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, preferences: service})
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	for _, body := range []string{
		`{"applicationLocale":"es","appearance":"SYSTEM","revision":1}`,
		`{"applicationLocale":null,"appearance":"BLUE","revision":1}`,
		`{"appearance":"SYSTEM","revision":1}`,
	} {
		if response := authRequest(router, http.MethodPut, "/api/me/preferences", body, cookie, authTestCSRFToken().Value()); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid preferences status=%d body=%s", response.Code, response.Body.String())
		}
	}
	service.err = preferences.ErrRevisionMismatch
	if response := authRequest(router, http.MethodPut, "/api/me/preferences", `{"applicationLocale":null,"appearance":"SYSTEM","revision":1}`, cookie, authTestCSRFToken().Value()); response.Code != http.StatusConflict {
		t.Fatalf("stale update status=%d", response.Code)
	}
	service.err = errors.New("database unavailable")
	if response := authRequest(router, http.MethodGet, "/api/me/preferences", "", cookie); response.Code != http.StatusInternalServerError {
		t.Fatalf("storage error status=%d", response.Code)
	}
}
