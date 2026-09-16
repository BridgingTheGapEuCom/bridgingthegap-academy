package platform

import (
	"errors"
	"fmt"
	"net"
	"os"
)

type Config struct {
	DatabaseURL              string
	HTTPAddr                 string
	MetricsAddr              string
	DevelopmentHTTP          bool
	PublicOrigin             string
	RequireIndependentReview bool
}

func LoadConfig() (Config, error) {
	requireIndependentReview, err := envBool("BTG_LMS_REQUIRE_INDEPENDENT_REVIEW", true)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		DatabaseURL:              os.Getenv("BTG_LMS_DATABASE_URL"),
		HTTPAddr:                 envOr("BTG_LMS_HTTP_ADDR", ":8080"),
		MetricsAddr:              envOr("BTG_LMS_METRICS_ADDR", "127.0.0.1:9090"),
		PublicOrigin:             os.Getenv("BTG_LMS_PUBLIC_ORIGIN"),
		RequireIndependentReview: requireIndependentReview,
	}
	switch envOr("BTG_LMS_MODE", "production") {
	case "production":
	case "development":
		host, _, err := net.SplitHostPort(cfg.HTTPAddr)
		if err != nil || host != "localhost" && (net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback()) {
			return Config{}, errors.New("development HTTP must bind to a loopback address")
		}
		cfg.DevelopmentHTTP = true
	default:
		return Config{}, errors.New("BTG_LMS_MODE must be production or development")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("BTG_LMS_DATABASE_URL is required")
	}
	for name, address := range map[string]string{"BTG_LMS_HTTP_ADDR": cfg.HTTPAddr, "BTG_LMS_METRICS_ADDR": cfg.MetricsAddr} {
		if _, _, err := net.SplitHostPort(address); err != nil {
			return Config{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) (bool, error) {
	value, set := os.LookupEnv(key)
	if !set || value == "" {
		return fallback, nil
	}
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false", key)
	}
}
