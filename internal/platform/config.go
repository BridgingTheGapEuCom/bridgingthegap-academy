package platform

import (
	"errors"
	"fmt"
	"net"
	"os"
)

type Config struct {
	DatabaseURL string
	HTTPAddr    string
	MetricsAddr string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("BTG_LMS_DATABASE_URL"),
		HTTPAddr:    envOr("BTG_LMS_HTTP_ADDR", ":8080"),
		MetricsAddr: envOr("BTG_LMS_METRICS_ADDR", "127.0.0.1:9090"),
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
