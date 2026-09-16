package platform

import "testing"

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
