package platform

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

func setValidCertificateIssuer(t *testing.T) {
	t.Helper()
	t.Setenv("BTG_LMS_CERTIFICATE_ISSUER_ID", "https://academy.example.com")
	t.Setenv("BTG_LMS_CERTIFICATE_ISSUER_NAME", "Academy")
}

func TestLoadConfigRequiresDatabaseAndValidAddresses(t *testing.T) {
	setValidCertificateIssuer(t)
	t.Setenv("BTG_LMS_MODE", "production")
	t.Setenv("BTG_LMS_ASSET_STORAGE_PATH", t.TempDir())
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

func TestCertificateIssuerConfigurationIsRequiredAndValidated(t *testing.T) {
	t.Setenv("BTG_LMS_MODE", "production")
	t.Setenv("BTG_LMS_DATABASE_URL", "postgres://localhost/btg")
	t.Setenv("BTG_LMS_ASSET_STORAGE_PATH", t.TempDir())
	t.Setenv("BTG_LMS_CERTIFICATE_ISSUER_ID", "")
	t.Setenv("BTG_LMS_CERTIFICATE_ISSUER_NAME", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("accepted missing certificate issuer")
	}
	setValidCertificateIssuer(t)
	cfg, err := LoadConfig()
	if err != nil || cfg.CertificateIssuer.ID != "https://academy.example.com" || cfg.CertificateIssuer.Name != "Academy" {
		t.Fatalf("certificate issuer = %#v, %v", cfg.CertificateIssuer, err)
	}
}

func TestCookieModeRequiresExplicitLoopbackDevelopment(t *testing.T) {
	setValidCertificateIssuer(t)
	t.Setenv("BTG_LMS_ASSET_STORAGE_PATH", t.TempDir())
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
	setValidCertificateIssuer(t)
	t.Setenv("BTG_LMS_ASSET_STORAGE_PATH", t.TempDir())
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
		t.Setenv("BTG_LMS_ASSET_STORAGE_PATH", t.TempDir())
		setValidCertificateIssuer(t)
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

func TestAssetStorageConfigurationIsExplicitAndBounded(t *testing.T) {
	setValidCertificateIssuer(t)
	t.Setenv("BTG_LMS_DATABASE_URL", "postgres://localhost/btg")
	t.Setenv("BTG_LMS_HTTP_ADDR", ":8080")
	t.Setenv("BTG_LMS_MODE", "production")
	t.Setenv("BTG_LMS_ASSET_STORAGE_PATH", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("accepted missing asset storage path")
	}
	t.Setenv("BTG_LMS_ASSET_STORAGE_PATH", "relative/assets")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("accepted relative asset storage path")
	}
	t.Setenv("BTG_LMS_ASSET_STORAGE_PATH", t.TempDir())
	t.Setenv("BTG_LMS_ASSET_MAX_BYTES", "")
	cfg, err := LoadConfig()
	if err != nil || cfg.AssetMaxBytes != defaultAssetMaxBytes {
		t.Fatalf("default max bytes = %d, %v", cfg.AssetMaxBytes, err)
	}
	t.Setenv("BTG_LMS_ASSET_MAX_BYTES", "4096")
	cfg, err = LoadConfig()
	if err != nil || cfg.AssetMaxBytes != 4096 {
		t.Fatalf("configured max bytes = %d, %v", cfg.AssetMaxBytes, err)
	}
	for _, invalid := range []string{"0", "-1", "many"} {
		t.Setenv("BTG_LMS_ASSET_MAX_BYTES", invalid)
		if _, err := LoadConfig(); err == nil {
			t.Fatalf("accepted invalid asset maximum %q", invalid)
		}
	}
}

func TestOpenBadgesSubjectSecretIsOptionalUntilExportIsEnabledButNeverWeak(t *testing.T) {
	setValidCertificateIssuer(t)
	t.Setenv("BTG_LMS_MODE", "production")
	t.Setenv("BTG_LMS_DATABASE_URL", "postgres://localhost/btg")
	t.Setenv("BTG_LMS_ASSET_STORAGE_PATH", t.TempDir())
	t.Setenv("BTG_LMS_OPEN_BADGES_SUBJECT_SECRET", "short")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("accepted weak Open Badges subject secret")
	}
	t.Setenv("BTG_LMS_OPEN_BADGES_SUBJECT_SECRET", "01234567890123456789012345678901")
	cfg, err := LoadConfig()
	if err != nil || len(cfg.OpenBadgesSubjectSecret) != 32 {
		t.Fatalf("secret configuration=%d,%v", len(cfg.OpenBadgesSubjectSecret), err)
	}
}
