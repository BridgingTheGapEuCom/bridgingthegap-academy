// Package preferences owns authenticated-user application preferences.
// It deliberately excludes account/profile attributes and authored Course data.
package preferences

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

var (
	ErrInvalidPreferences = errors.New("invalid user preferences")
	ErrRevisionMismatch   = errors.New("user preferences revision mismatch")
)

type Appearance string

const (
	AppearanceSystem Appearance = "SYSTEM"
	AppearanceLight  Appearance = "LIGHT"
	AppearanceDark   Appearance = "DARK"
)

func ParseAppearance(value string) (Appearance, error) {
	appearance := Appearance(value)
	switch appearance {
	case AppearanceSystem, AppearanceLight, AppearanceDark:
		return appearance, nil
	default:
		return "", fmt.Errorf("%w: appearance", ErrInvalidPreferences)
	}
}

// ApplicationLocale is intentionally distinct from authored Course languages.
// Keep this backend list in sync with production application locales only.
type ApplicationLocale string

const ApplicationLocaleEnglish ApplicationLocale = "en"

func ParseApplicationLocale(value string) (ApplicationLocale, error) {
	if ApplicationLocale(value) != ApplicationLocaleEnglish {
		return "", fmt.Errorf("%w: application locale", ErrInvalidPreferences)
	}
	return ApplicationLocale(value), nil
}

type UserPreferences struct {
	UserID            identity.UserID
	ApplicationLocale *ApplicationLocale
	Appearance        Appearance
	Revision          int64
	CreatedAt         *time.Time
	UpdatedAt         *time.Time
}

type Update struct {
	ApplicationLocale *ApplicationLocale
	Appearance        Appearance
}

func (update Update) Validate() error {
	if _, err := ParseAppearance(string(update.Appearance)); err != nil {
		return err
	}
	if update.ApplicationLocale != nil {
		if _, err := ParseApplicationLocale(string(*update.ApplicationLocale)); err != nil {
			return err
		}
	}
	return nil
}

type Repository interface {
	GetUserPreferences(context.Context, identity.UserID) (UserPreferences, error)
	CreateUserPreferences(context.Context, identity.UserID, Update) (UserPreferences, error)
	UpdateUserPreferences(context.Context, identity.UserID, int64, Update) (UserPreferences, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

// Get returns a logical, unpersisted default when the user has never saved a
// preference. Revision zero is reserved for that first-write CAS operation.
func (s *Service) Get(ctx context.Context, userID identity.UserID) (UserPreferences, error) {
	if s == nil || s.repository == nil || userID == "" {
		return UserPreferences{}, ErrInvalidPreferences
	}
	preferences, err := s.repository.GetUserPreferences(ctx, userID)
	if errors.Is(err, identity.ErrNotFound) {
		return UserPreferences{UserID: userID, Appearance: AppearanceSystem, Revision: 0}, nil
	}
	return preferences, err
}

func (s *Service) Update(ctx context.Context, userID identity.UserID, expectedRevision int64, update Update) (UserPreferences, error) {
	if s == nil || s.repository == nil || userID == "" || expectedRevision < 0 {
		return UserPreferences{}, ErrInvalidPreferences
	}
	if err := update.Validate(); err != nil {
		return UserPreferences{}, err
	}
	preferences, err := s.repository.UpdateUserPreferences(ctx, userID, expectedRevision, update)
	if err == nil {
		return preferences, nil
	}
	if !errors.Is(err, identity.ErrNotFound) {
		return UserPreferences{}, err
	}
	if expectedRevision != 0 {
		return UserPreferences{}, ErrRevisionMismatch
	}
	preferences, err = s.repository.CreateUserPreferences(ctx, userID, update)
	if errors.Is(err, identity.ErrConflict) {
		return UserPreferences{}, ErrRevisionMismatch
	}
	return preferences, err
}
