//go:build integration

package platform

import (
	"context"
	"errors"
	"testing"

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
