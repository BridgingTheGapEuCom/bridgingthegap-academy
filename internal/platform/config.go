package platform

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges/signing"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
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
	CertificateIssuer        credentials.Issuer
	OpenBadgesSubjectSecret  secretBytes
	OpenBadgesKeyID          string
	OpenBadgesSeedFile       string
	openBadgesSeed           secretSeed
	PluginRuntimeOrigin      string
	PluginRuntimeKeyID       string
	PluginRuntimeSeedFile    string
	pluginRuntimeSeed        secretSeed
}

// String keeps credentials out of accidental structured configuration logs.
func (Config) String() string   { return "platform.Config{secrets redacted}" }
func (Config) GoString() string { return "platform.Config{secrets redacted}" }

func LoadConfig() (Config, error) {
	requireIndependentReview, err := envBool("BTG_LMS_REQUIRE_INDEPENDENT_REVIEW", true)
	if err != nil {
		return Config{}, err
	}
	assetMaxBytes, err := envPositiveInt64("BTG_LMS_ASSET_MAX_BYTES", defaultAssetMaxBytes)
	if err != nil {
		return Config{}, err
	}
	seed, seedFile, err := openBadgesSigningSeed()
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
		OpenBadgesSubjectSecret:  secretBytes(os.Getenv("BTG_LMS_OPEN_BADGES_SUBJECT_SECRET")),
		OpenBadgesKeyID:          os.Getenv("BTG_LMS_OPEN_BADGES_KEY_ID"),
		OpenBadgesSeedFile:       seedFile,
		openBadgesSeed:           seed,
		CertificateIssuer: credentials.Issuer{
			ID: os.Getenv("BTG_LMS_CERTIFICATE_ISSUER_ID"), Name: os.Getenv("BTG_LMS_CERTIFICATE_ISSUER_NAME"),
		},
	}
	runtimeSeed, runtimeSeedFile, err := pluginRuntimeSigningSeed()
	if err != nil {
		return Config{}, err
	}
	cfg.PluginRuntimeOrigin = os.Getenv("BTG_LMS_PLUGIN_RUNTIME_ORIGIN")
	cfg.PluginRuntimeKeyID = os.Getenv("BTG_LMS_PLUGIN_RUNTIME_KEY_ID")
	cfg.PluginRuntimeSeedFile = runtimeSeedFile
	cfg.pluginRuntimeSeed = runtimeSeed
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
	if err := cfg.CertificateIssuer.Validate(); err != nil {
		return Config{}, errors.New("BTG_LMS_CERTIFICATE_ISSUER_ID and BTG_LMS_CERTIFICATE_ISSUER_NAME are required")
	}
	if len(cfg.OpenBadgesSubjectSecret) > 0 && len(cfg.OpenBadgesSubjectSecret) < 32 {
		return Config{}, errors.New("BTG_LMS_OPEN_BADGES_SUBJECT_SECRET must contain at least 32 bytes")
	}
	if cfg.openBadgesSeed != "" || cfg.OpenBadgesKeyID != "" {
		if cfg.openBadgesSeed == "" || cfg.OpenBadgesKeyID == "" || len(cfg.OpenBadgesSubjectSecret) < 32 {
			return Config{}, errors.New("signed Open Badges requires a key ID, Ed25519 seed, and subject secret")
		}
		origin, err := normalizeSignedOpenBadgesOrigin(cfg.PublicOrigin, cfg.DevelopmentHTTP)
		if err != nil {
			return Config{}, errors.New("signed Open Badges requires a public HTTPS origin")
		}
		cfg.PublicOrigin = origin
		if cfg.CertificateIssuer.ID != cfg.PublicOrigin+"/open-badges/issuer" {
			return Config{}, errors.New("signed Open Badges issuer must resolve at the public issuer resource")
		}
		if !strings.HasPrefix(cfg.OpenBadgesKeyID, cfg.CertificateIssuer.ID+"#") {
			return Config{}, errors.New("signed Open Badges key ID must identify an issuer assertion method")
		}
		if _, err := signing.NewLocalKey(cfg.OpenBadgesKeyID, cfg.CertificateIssuer.ID, string(cfg.openBadgesSeed)); err != nil {
			return Config{}, errors.New("invalid signed Open Badges key material")
		}
	}
	if cfg.pluginRuntimeSeed != "" || cfg.PluginRuntimeKeyID != "" || cfg.PluginRuntimeOrigin != "" {
		if cfg.pluginRuntimeSeed == "" || cfg.PluginRuntimeKeyID == "" || cfg.PluginRuntimeOrigin == "" || cfg.PublicOrigin == "" {
			return Config{}, errors.New("widget runtime requires a distinct origin, key ID, Ed25519 seed, and public origin")
		}
		runtimeOrigin, runtimeErr := parseOrigin(cfg.PluginRuntimeOrigin)
		publicOrigin, publicErr := parseOrigin(cfg.PublicOrigin)
		if runtimeErr != nil || publicErr != nil || runtimeOrigin == publicOrigin || runtimeOrigin.host == publicOrigin.host {
			return Config{}, errors.New("widget runtime origin must use a distinct host")
		}
		if !cfg.DevelopmentHTTP && (runtimeOrigin.scheme != "https" || publicOrigin.scheme != "https") {
			return Config{}, errors.New("production widget runtime origins must use HTTPS")
		}
		if _, _, err := cfg.widgetRuntimeTokenService(); err != nil {
			return Config{}, err
		}
	}
	for name, address := range map[string]string{"BTG_LMS_HTTP_ADDR": cfg.HTTPAddr, "BTG_LMS_METRICS_ADDR": cfg.MetricsAddr} {
		if _, _, err := net.SplitHostPort(address); err != nil {
			return Config{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	return cfg, nil
}

func pluginRuntimeSigningSeed() (secretSeed, string, error) {
	inline := os.Getenv("BTG_LMS_PLUGIN_RUNTIME_ED25519_SEED_B64URL")
	file := os.Getenv("BTG_LMS_PLUGIN_RUNTIME_ED25519_SEED_FILE")
	if inline != "" && file != "" {
		return "", "", errors.New("configure only one widget runtime Ed25519 seed source")
	}
	if file == "" {
		return secretSeed(inline), "", nil
	}
	info, err := os.Stat(file)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return "", "", errors.New("widget runtime Ed25519 seed file must be a regular owner-only file")
	}
	value, err := os.ReadFile(file)
	if err != nil || strings.TrimSpace(string(value)) == "" {
		return "", "", errors.New("widget runtime Ed25519 seed file is unavailable")
	}
	return secretSeed(strings.TrimSpace(string(value))), file, nil
}

// openBadgesSigningSeed supports exactly one production secret source: a
// base64url seed supplied directly by the environment or by a regular 0600
// (or stricter) secret file mounted for the LMS service account. A file is
// read once at startup; it is never watched, logged, or returned by HTTP.
func openBadgesSigningSeed() (secretSeed, string, error) {
	inline := os.Getenv("BTG_LMS_OPEN_BADGES_ED25519_SEED_B64URL")
	file := os.Getenv("BTG_LMS_OPEN_BADGES_ED25519_SEED_FILE")
	if inline != "" && file != "" {
		return "", "", errors.New("configure only one Open Badges Ed25519 seed source")
	}
	if file == "" {
		return secretSeed(inline), "", nil
	}
	info, err := os.Stat(file)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return "", "", errors.New("Open Badges Ed25519 seed file must be a regular owner-only file")
	}
	bytes, err := os.ReadFile(file)
	if err != nil || strings.TrimSpace(string(bytes)) == "" {
		return "", "", errors.New("Open Badges Ed25519 seed file is unavailable")
	}
	return secretSeed(strings.TrimSpace(string(bytes))), file, nil
}

func normalizeSignedOpenBadgesOrigin(value string, development bool) (string, error) {
	origin, err := url.Parse(value)
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || (origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.Fragment != "" {
		return "", errors.New("invalid public origin")
	}
	host := origin.Hostname()
	if !development && (strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()) {
		return "", errors.New("production public origin must not be loopback")
	}
	origin.Path, origin.RawPath = "", ""
	return strings.TrimRight(origin.String(), "/"), nil
}

func (cfg Config) signedOpenBadgesKey() (signing.KeyProvider, bool, error) {
	if cfg.openBadgesSeed == "" {
		return nil, false, nil
	}
	key, err := signing.NewLocalKey(cfg.OpenBadgesKeyID, cfg.CertificateIssuer.ID, string(cfg.openBadgesSeed))
	if err != nil {
		return nil, true, errors.New("invalid signed Open Badges key material")
	}
	return key, true, nil
}

func (cfg Config) widgetRuntimeTokenService() (*plugins.RuntimeTokenService, bool, error) {
	if cfg.pluginRuntimeSeed == "" {
		return nil, false, nil
	}
	seed, err := base64.RawURLEncoding.DecodeString(string(cfg.pluginRuntimeSeed))
	if err != nil {
		return nil, true, errors.New("invalid widget runtime Ed25519 seed")
	}
	service, err := plugins.NewRuntimeTokenService(cfg.PluginRuntimeKeyID, seed, plugins.DefaultTokenLifetime)
	if err != nil {
		return nil, true, errors.New("invalid widget runtime signing configuration")
	}
	return service, true, nil
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

// secretSeed redacts private key configuration in formatted diagnostics.
type secretSeed string

func (secretSeed) String() string   { return "[redacted]" }
func (secretSeed) GoString() string { return "[redacted]" }

type secretBytes []byte

func (secretBytes) String() string   { return "[redacted]" }
func (secretBytes) GoString() string { return "[redacted]" }
