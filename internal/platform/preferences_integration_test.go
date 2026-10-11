//go:build integration

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/preferences"
	preferencespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/preferences/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testUserPreferencesPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	identities := identitypostgres.New(pool)
	first, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	second, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	service := preferences.NewService(preferencespostgres.New(pool))
	current, err := service.Get(ctx, first.ID)
	if err != nil || current.Revision != 0 || current.ApplicationLocale != nil || current.Appearance != preferences.AppearanceSystem {
		t.Fatalf("default preferences = %#v, %v", current, err)
	}
	english := preferences.ApplicationLocaleEnglish
	created, err := service.Update(ctx, first.ID, 0, preferences.Update{ApplicationLocale: &english, Appearance: preferences.AppearanceDark})
	if err != nil || created.Revision != 1 || created.UpdatedAt == nil {
		t.Fatalf("first persisted preferences = %#v, %v", created, err)
	}
	read, err := service.Get(ctx, first.ID)
	if err != nil || read.Revision != 1 || read.ApplicationLocale == nil || *read.ApplicationLocale != english {
		t.Fatalf("persisted read = %#v, %v", read, err)
	}
	updated, err := service.Update(ctx, first.ID, 1, preferences.Update{Appearance: preferences.AppearanceLight})
	if err != nil || updated.Revision != 2 || updated.ApplicationLocale != nil || updated.Appearance != preferences.AppearanceLight {
		t.Fatalf("persisted update = %#v, %v", updated, err)
	}
	if _, err := service.Update(ctx, first.ID, 1, preferences.Update{Appearance: preferences.AppearanceDark}); !errors.Is(err, preferences.ErrRevisionMismatch) {
		t.Fatalf("stale update error = %v", err)
	}
	other, err := service.Get(ctx, second.ID)
	if err != nil || other.Revision != 0 || other.ApplicationLocale != nil || other.Appearance != preferences.AppearanceSystem {
		t.Fatalf("per-user isolation = %#v, %v", other, err)
	}
}

func testUserPreferencesHTTPIntegration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	identities := identitypostgres.New(pool)
	user, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	rawToken := mustRawToken().Value()
	csrfToken := authTestCSRFToken().Value()
	if _, err := pool.Exec(ctx, "INSERT INTO identity.sessions (user_id, token_digest, csrf_token, expires_at) VALUES ($1, $2, $3, $4)", user.ID, sha256Digest(rawToken), csrfToken, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	service := preferences.NewService(preferencespostgres.New(pool))
	auth := &authHTTP{
		sessions:    identity.NewSessionService(identities, nil, time.Now),
		csrf:        identity.NewSessionCSRFService(identities, time.Now),
		preferences: service,
	}
	router := authTestRouter(auth)
	cookie := &http.Cookie{Name: sessionCookieName, Value: rawToken}

	var rows int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM preferences.user_preferences WHERE user_id = $1", user.ID).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("first-use setup has %d preference rows: %v", rows, err)
	}
	response := authRequest(router, http.MethodGet, "/api/me/preferences", "", cookie)
	var current userPreferencesDTO
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &current) != nil || current.ApplicationLocale != nil || current.Appearance != preferences.AppearanceSystem || current.Revision != 0 {
		t.Fatalf("first-use GET status=%d body=%s", response.Code, response.Body.String())
	}
	response = authRequest(router, http.MethodPut, "/api/me/preferences", `{"applicationLocale":"en","appearance":"DARK","revision":0}`, cookie, csrfToken)
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &current) != nil || current.Revision != 1 || current.Appearance != preferences.AppearanceDark {
		t.Fatalf("create PUT status=%d body=%s", response.Code, response.Body.String())
	}
	response = authRequest(router, http.MethodGet, "/api/me/preferences", "", cookie)
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &current) != nil || current.Revision != 1 || current.ApplicationLocale == nil {
		t.Fatalf("persisted GET status=%d body=%s", response.Code, response.Body.String())
	}
	response = authRequest(router, http.MethodPut, "/api/me/preferences", `{"applicationLocale":null,"appearance":"LIGHT","revision":1}`, cookie, csrfToken)
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &current) != nil || current.Revision != 2 || current.Appearance != preferences.AppearanceLight {
		t.Fatalf("update PUT status=%d body=%s", response.Code, response.Body.String())
	}
	if stale := authRequest(router, http.MethodPut, "/api/me/preferences", `{"applicationLocale":null,"appearance":"SYSTEM","revision":1}`, cookie, csrfToken); stale.Code != http.StatusConflict {
		t.Fatalf("stale PUT status=%d body=%s", stale.Code, stale.Body.String())
	}
}
