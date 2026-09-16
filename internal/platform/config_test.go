package platform

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

func TestLoadConfigRequiresDatabaseAndValidAddresses(t *testing.T) {
	t.Setenv("BTG_LMS_MODE", "production")
	t.Setenv("BTG_LMS_DATABASE_URL", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("accepted missing database URL")
	}
	t.Setenv("BTG_LMS_DATABASE_URL", "postgres://localhost/btg")
	t.Setenv("BTG_LMS_HTTP_ADDR", "invalid-address")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("accepted invalid HTTP address")
	}
}

func TestCookieModeRequiresExplicitLoopbackDevelopment(t *testing.T) {
	t.Setenv("BTG_LMS_DATABASE_URL", "postgres://localhost/btg")
	t.Setenv("BTG_LMS_HTTP_ADDR", ":8080")
	t.Setenv("BTG_LMS_MODE", "production")
	production, err := LoadConfig()
	if err != nil || production.DevelopmentHTTP {
		t.Fatalf("production cookie is not Secure: %v", err)
	}
	t.Setenv("BTG_LMS_MODE", "")
	defaultMode, err := LoadConfig()
	if err != nil || defaultMode.DevelopmentHTTP {
		t.Fatalf("default mode did not require Secure cookies: %v", err)
	}
	t.Setenv("BTG_LMS_MODE", "development")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("public development HTTP binding was accepted")
	}
	t.Setenv("BTG_LMS_HTTP_ADDR", "127.0.0.1:8080")
	development, err := LoadConfig()
	if err != nil || !development.DevelopmentHTTP {
		t.Fatalf("loopback development cookie policy is wrong: %v", err)
	}
	t.Setenv("BTG_LMS_MODE", "unknown")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("unknown deployment mode was accepted")
	}
}

func TestIndependentReviewConfigurationDefaultsToRequiredAndCanBeDisabled(t *testing.T) {
	t.Setenv("BTG_LMS_DATABASE_URL", "postgres://localhost/btg")
	t.Setenv("BTG_LMS_HTTP_ADDR", ":8080")
	t.Setenv("BTG_LMS_MODE", "production")
	t.Setenv("BTG_LMS_REQUIRE_INDEPENDENT_REVIEW", "")

	defaultConfig, err := LoadConfig()
	if err != nil || !defaultConfig.RequireIndependentReview {
		t.Fatalf("independent review default = %t, error = %v", defaultConfig.RequireIndependentReview, err)
	}

	t.Setenv("BTG_LMS_REQUIRE_INDEPENDENT_REVIEW", "false")
	disabledConfig, err := LoadConfig()
	if err != nil || disabledConfig.RequireIndependentReview {
		t.Fatalf("independent review disable = %t, error = %v", disabledConfig.RequireIndependentReview, err)
	}

	t.Setenv("BTG_LMS_REQUIRE_INDEPENDENT_REVIEW", "invalid")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("accepted invalid independent-review configuration")
	}
}

func TestConfiguredReviewApplicationUsesIndependentReviewSetting(t *testing.T) {
	actor, err := identity.ActorFromResolvedSession(loginTestCurrent(t))
	if err != nil {
		t.Fatal(err)
	}
	draftID := authoring.DraftID("11111111-1111-4111-8111-111111111111")
	reviewID := authoring.ReviewID("22222222-2222-4222-8222-222222222222")
	load := func(t *testing.T, value string) Config {
		t.Helper()
		t.Setenv("BTG_LMS_DATABASE_URL", "postgres://localhost/btg")
		t.Setenv("BTG_LMS_HTTP_ADDR", ":8080")
		t.Setenv("BTG_LMS_MODE", "production")
		t.Setenv("BTG_LMS_REQUIRE_INDEPENDENT_REVIEW", value)
		cfg, loadErr := LoadConfig()
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		return cfg
	}

	for _, test := range []struct {
		name    string
		setting string
		wantErr error
	}{
		{name: "default enabled policy rejects submitter", setting: "", wantErr: authoring.ErrIndependentReviewerRequired},
		{name: "explicit disable permits submitter subject to authorization", setting: "false"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &authoringReviewRepositoryFake{cycle: authoring.ReviewCycle{
				ID: reviewID, DraftID: draftID, Status: authoring.ReviewInReview, Revision: 1,
				SubmittedByUserID: string(actor.UserID()), SubmittedAt: time.Now().UTC(),
			}}
			memberships := &authoringMembershipsFake{roles: map[authoring.DraftID]authoring.MemberRole{draftID: authoring.MemberMaintainer}}
			service := newAuthoringReviewApplicationService(load(t, test.setting), repository, authoring.NewAuthorizationService(memberships))

			_, decideErr := service.Approve(context.Background(), actor, draftID, reviewID, 1, "")
			if !errors.Is(decideErr, test.wantErr) {
				t.Fatalf("configured decision error = %v, want %v", decideErr, test.wantErr)
			}
			if test.wantErr != nil && repository.cycle.Revision != 1 {
				t.Fatalf("rejected configured policy mutated Review: %#v", repository.cycle)
			}
			if test.wantErr == nil && repository.cycle.Revision != 2 {
				t.Fatalf("disabled configured policy did not reach Review CAS: %#v", repository.cycle)
			}
		})
	}
}
