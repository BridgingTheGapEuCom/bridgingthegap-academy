package platform

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

const defaultAssetMaxBytes int64 = 100 * 1024 * 1024

type Config struct {
	DatabaseURL              string
	HTTPAddr                 string
	MetricsAddr              string
	DevelopmentHTTP          bool
	PublicOrigin             string
	RequireIndependentReview bool
	AssetStoragePath         string
	AssetMaxBytes            int64
}

func LoadConfig() (Config, error) {
	requireIndependentReview, err := envBool("BTG_LMS_REQUIRE_INDEPENDENT_REVIEW", true)
	if err != nil {
		return Config{}, err
	}
	assetMaxBytes, err := envPositiveInt64("BTG_LMS_ASSET_MAX_BYTES", defaultAssetMaxBytes)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		DatabaseURL:              os.Getenv("BTG_LMS_DATABASE_URL"),
		HTTPAddr:                 envOr("BTG_LMS_HTTP_ADDR", ":8080"),
		MetricsAddr:              envOr("BTG_LMS_METRICS_ADDR", "127.0.0.1:9090"),
		PublicOrigin:             os.Getenv("BTG_LMS_PUBLIC_ORIGIN"),
		RequireIndependentReview: requireIndependentReview,
		AssetStoragePath:         os.Getenv("BTG_LMS_ASSET_STORAGE_PATH"),
		AssetMaxBytes:            assetMaxBytes,
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
	if cfg.AssetStoragePath == "" {
		return Config{}, errors.New("BTG_LMS_ASSET_STORAGE_PATH is required")
	}
	if !filepath.IsAbs(cfg.AssetStoragePath) {
		return Config{}, errors.New("BTG_LMS_ASSET_STORAGE_PATH must be absolute")
	}
	for name, address := range map[string]string{"BTG_LMS_HTTP_ADDR": cfg.HTTPAddr, "BTG_LMS_METRICS_ADDR": cfg.MetricsAddr} {
		if _, _, err := net.SplitHostPort(address); err != nil {
			return Config{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	return cfg, nil
}

func envPositiveInt64(key string, fallback int64) (int64, error) {
	value, set := os.LookupEnv(key)
	if !set || value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return parsed, nil
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
