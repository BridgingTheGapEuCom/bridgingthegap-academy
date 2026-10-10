package preferences

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

type repositoryFake struct {
	stored UserPreferences
	err    error
}

func (r *repositoryFake) GetUserPreferences(_ context.Context, _ identity.UserID) (UserPreferences, error) {
	return r.stored, r.err
}
func (r *repositoryFake) CreateUserPreferences(_ context.Context, userID identity.UserID, update Update) (UserPreferences, error) {
	if r.err != nil {
		return UserPreferences{}, r.err
	}
	now := time.Now().UTC()
	r.stored = UserPreferences{UserID: userID, ApplicationLocale: update.ApplicationLocale, Appearance: update.Appearance, Revision: 1, CreatedAt: &now, UpdatedAt: &now}
	return r.stored, nil
}
func (r *repositoryFake) UpdateUserPreferences(_ context.Context, userID identity.UserID, revision int64, update Update) (UserPreferences, error) {
	if r.err != nil || r.stored.Revision != revision {
		return UserPreferences{}, identity.ErrNotFound
	}
	r.stored.UserID, r.stored.ApplicationLocale, r.stored.Appearance, r.stored.Revision = userID, update.ApplicationLocale, update.Appearance, revision+1
	return r.stored, nil
}

func TestPreferencesDefaultsAndFirstWrite(t *testing.T) {
	repository := &repositoryFake{err: identity.ErrNotFound}
	service := NewService(repository)
	userID := identity.UserID("11111111-1111-4111-8111-111111111111")
	current, err := service.Get(context.Background(), userID)
	if err != nil || current.Revision != 0 || current.Appearance != AppearanceSystem || current.ApplicationLocale != nil {
		t.Fatalf("default preferences = %#v, %v", current, err)
	}
	repository.err = nil
	english := ApplicationLocaleEnglish
	updated, err := service.Update(context.Background(), userID, 0, Update{ApplicationLocale: &english, Appearance: AppearanceDark})
	if err != nil || updated.Revision != 1 || updated.Appearance != AppearanceDark || updated.ApplicationLocale == nil || *updated.ApplicationLocale != english {
		t.Fatalf("first update = %#v, %v", updated, err)
	}
}

func TestPreferencesValidationAndRevision(t *testing.T) {
	userID := identity.UserID("11111111-1111-4111-8111-111111111111")
	english := ApplicationLocaleEnglish
	repository := &repositoryFake{stored: UserPreferences{UserID: userID, ApplicationLocale: &english, Appearance: AppearanceSystem, Revision: 1}}
	service := NewService(repository)
	if _, err := service.Update(context.Background(), userID, 1, Update{Appearance: Appearance("BLUE")}); !errors.Is(err, ErrInvalidPreferences) {
		t.Fatalf("invalid appearance error = %v", err)
	}
	unsupported := ApplicationLocale("es")
	if _, err := service.Update(context.Background(), userID, 1, Update{ApplicationLocale: &unsupported, Appearance: AppearanceLight}); !errors.Is(err, ErrInvalidPreferences) {
		t.Fatalf("unsupported locale error = %v", err)
	}
	updated, err := service.Update(context.Background(), userID, 1, Update{ApplicationLocale: nil, Appearance: AppearanceLight})
	if err != nil || updated.Revision != 2 || updated.ApplicationLocale != nil {
		t.Fatalf("update = %#v, %v", updated, err)
	}
	if _, err := service.Update(context.Background(), userID, 1, Update{Appearance: AppearanceLight}); !errors.Is(err, ErrRevisionMismatch) {
		t.Fatalf("stale update error = %v", err)
	}
}
