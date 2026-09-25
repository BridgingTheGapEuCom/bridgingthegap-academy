package plugins

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	RuntimeAudience        = "widget-runtime"
	RuntimeProtocol        = "btg-widget-runtime"
	RuntimeProtocolVersion = 1
	CapabilityBootstrap    = "widget.runtime.bootstrap"
	CapabilityContextRead  = "widget.runtime.context.read"
	DefaultTokenLifetime   = 5 * time.Minute
)

var (
	ErrInvalidRuntimeConfiguration = errors.New("invalid plugin runtime configuration")
	ErrInvalidRuntimeToken         = errors.New("invalid plugin runtime token")
	ErrRuntimeTokenExpired         = errors.New("plugin runtime token expired")
	ErrRuntimeCapabilityDenied     = errors.New("plugin runtime capability denied")
)

type RuntimeContext struct {
	RuntimeInstanceID string     `json:"runtimeInstanceId"`
	PluginID          PluginID   `json:"pluginId"`
	PluginVersion     string     `json:"pluginVersion"`
	ArtifactDigest    string     `json:"artifactDigest"`
	WidgetID          string     `json:"widgetId"`
	WidgetType        PluginType `json:"widgetType"`
}

type RuntimeLaunch struct {
	Context       RuntimeContext `json:"context"`
	RuntimeURL    string         `json:"runtimeUrl"`
	RuntimeOrigin string         `json:"runtimeOrigin"`
	Token         string         `json:"token"`
	ExpiresAt     time.Time      `json:"expiresAt"`
	Capabilities  []string       `json:"capabilities"`
}

type RuntimeClaims struct {
	Audience     string         `json:"aud"`
	IssuedAt     int64          `json:"iat"`
	ExpiresAt    int64          `json:"exp"`
	Context      RuntimeContext `json:"context"`
	Capabilities []string       `json:"capabilities"`
}

type RuntimeTokenService struct {
	keyID    string
	private  ed25519.PrivateKey
	public   ed25519.PublicKey
	lifetime time.Duration
	now      func() time.Time
}

func NewRuntimeTokenService(keyID string, seed []byte, lifetime time.Duration) (*RuntimeTokenService, error) {
	if !keyIDPattern.MatchString(keyID) || len(seed) != ed25519.SeedSize {
		return nil, ErrInvalidRuntimeConfiguration
	}
	if lifetime <= 0 {
		lifetime = DefaultTokenLifetime
	}
	private := ed25519.NewKeyFromSeed(append([]byte(nil), seed...))
	return &RuntimeTokenService{keyID: keyID, private: private, public: append(ed25519.PublicKey(nil), private.Public().(ed25519.PublicKey)...), lifetime: lifetime, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *RuntimeTokenService) Issue(context RuntimeContext, capabilities []string) (string, time.Time, error) {
	if s == nil || validateRuntimeContext(context) != nil || !validCapabilities(capabilities) {
		return "", time.Time{}, ErrInvalidRuntimeToken
	}
	now := s.now().UTC().Truncate(time.Second)
	expires := now.Add(s.lifetime)
	header, _ := json.Marshal(struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}{"EdDSA", s.keyID, "BTG-WIDGET-RUNTIME"})
	claims, _ := json.Marshal(RuntimeClaims{Audience: RuntimeAudience, IssuedAt: now.Unix(), ExpiresAt: expires.Unix(), Context: context, Capabilities: append([]string(nil), capabilities...)})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	signature := ed25519.Sign(s.private, []byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), expires, nil
}

type RuntimeTokenExpectation struct {
	RuntimeInstanceID string
	PluginID          PluginID
	PluginVersion     string
	WidgetID          string
	Capability        string
}

func (s *RuntimeTokenService) Verify(token string, expected RuntimeTokenExpectation) (RuntimeClaims, error) {
	if s == nil || strings.TrimSpace(token) != token {
		return RuntimeClaims{}, ErrInvalidRuntimeToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return RuntimeClaims{}, ErrInvalidRuntimeToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return RuntimeClaims{}, ErrInvalidRuntimeToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}
	if strictDecode(headerBytes, &header) != nil || header.Algorithm != "EdDSA" || header.KeyID != s.keyID || header.Type != "BTG-WIDGET-RUNTIME" {
		return RuntimeClaims{}, ErrInvalidRuntimeToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(s.public, []byte(parts[0]+"."+parts[1]), signature) {
		return RuntimeClaims{}, ErrInvalidRuntimeToken
	}
	claimBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	var claims RuntimeClaims
	if err != nil || strictDecode(claimBytes, &claims) != nil || claims.Audience != RuntimeAudience || validateRuntimeContext(claims.Context) != nil || !validCapabilities(claims.Capabilities) || claims.IssuedAt <= 0 || claims.ExpiresAt <= claims.IssuedAt {
		return RuntimeClaims{}, ErrInvalidRuntimeToken
	}
	now := s.now().UTC().Unix()
	if now < claims.IssuedAt-30 || now >= claims.ExpiresAt {
		return RuntimeClaims{}, ErrRuntimeTokenExpired
	}
	if expected.RuntimeInstanceID != "" && claims.Context.RuntimeInstanceID != expected.RuntimeInstanceID || expected.PluginID != "" && claims.Context.PluginID != expected.PluginID || expected.PluginVersion != "" && claims.Context.PluginVersion != expected.PluginVersion || expected.WidgetID != "" && claims.Context.WidgetID != expected.WidgetID {
		return RuntimeClaims{}, ErrInvalidRuntimeToken
	}
	if expected.Capability != "" && !slices.Contains(claims.Capabilities, expected.Capability) {
		return RuntimeClaims{}, ErrRuntimeCapabilityDenied
	}
	return claims, nil
}

type RuntimeService struct {
	registry      *RegistryService
	tokens        *RuntimeTokenService
	runtimeOrigin string
	hostOrigin    string
}

func NewRuntimeService(registry *RegistryService, tokens *RuntimeTokenService, runtimeOrigin, hostOrigin string) (*RuntimeService, error) {
	runtimeURL, runtimeErr := exactOrigin(runtimeOrigin)
	hostURL, hostErr := exactOrigin(hostOrigin)
	if registry == nil || tokens == nil || runtimeErr != nil || hostErr != nil || strings.EqualFold(runtimeURL, hostURL) {
		return nil, ErrInvalidRuntimeConfiguration
	}
	return &RuntimeService{registry: registry, tokens: tokens, runtimeOrigin: runtimeURL, hostOrigin: hostURL}, nil
}

func (s *RuntimeService) PrepareWidgetRuntime(ctx context.Context, id PluginID, version, widgetID string, widgetType PluginType) (RuntimeLaunch, error) {
	release, entry, err := s.resolve(ctx, id, version, widgetID, widgetType)
	if err != nil {
		return RuntimeLaunch{}, err
	}
	runtime := RuntimeContext{RuntimeInstanceID: uuid.NewString(), PluginID: id, PluginVersion: version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: entry.ID, WidgetType: entry.Type}
	return s.issue(runtime)
}

func (s *RuntimeService) Refresh(ctx context.Context, token string) (RuntimeLaunch, error) {
	claims, err := s.tokens.Verify(token, RuntimeTokenExpectation{Capability: CapabilityBootstrap})
	if err != nil {
		return RuntimeLaunch{}, err
	}
	release, _, err := s.resolve(ctx, claims.Context.PluginID, claims.Context.PluginVersion, claims.Context.WidgetID, claims.Context.WidgetType)
	if err != nil {
		return RuntimeLaunch{}, err
	}
	if release.Release.ArtifactDigest != claims.Context.ArtifactDigest {
		return RuntimeLaunch{}, ErrInvalidRuntimeToken
	}
	return s.issue(claims.Context)
}

func (s *RuntimeService) VerifyContextToken(token string) (RuntimeClaims, error) {
	return s.tokens.Verify(token, RuntimeTokenExpectation{Capability: CapabilityContextRead})
}

func (s *RuntimeService) Resource(ctx context.Context, release ReleaseIdentity, resourcePath string) (InstalledResource, error) {
	resource, err := s.registry.Resource(ctx, release, resourcePath)
	if err != nil {
		return InstalledResource{}, err
	}
	if int64(len(resource.Content)) != resource.Size || sha256Hex(resource.Content) != resource.SHA256 {
		return InstalledResource{}, ErrResourceInvalid
	}
	return resource, nil
}

func (s *RuntimeService) resolve(ctx context.Context, id PluginID, version, widgetID string, widgetType PluginType) (InstalledRelease, Entrypoint, error) {
	release, err := s.registry.RuntimeRelease(ctx, id, version)
	if err != nil {
		return InstalledRelease{}, Entrypoint{}, err
	}
	for _, entry := range release.Manifest.Entrypoints {
		if entry.ID != widgetID {
			continue
		}
		if entry.Type != widgetType {
			return InstalledRelease{}, Entrypoint{}, ErrLaunchDenied
		}
		resource, err := s.Resource(ctx, release.Release, entry.Resource)
		if err != nil || resource.SHA256 != declaredDigest(release.Manifest, entry.Resource) {
			return InstalledRelease{}, Entrypoint{}, ErrResourceInvalid
		}
		return release, entry, nil
	}
	return InstalledRelease{}, Entrypoint{}, ErrLaunchDenied
}

func (s *RuntimeService) issue(runtime RuntimeContext) (RuntimeLaunch, error) {
	capabilities := []string{CapabilityBootstrap, CapabilityContextRead}
	token, expires, err := s.tokens.Issue(runtime, capabilities)
	if err != nil {
		return RuntimeLaunch{}, err
	}
	path := "/plugins/runtime/" + url.PathEscape(string(runtime.PluginID)) + "/" + url.PathEscape(runtime.PluginVersion) + "/" + runtime.ArtifactDigest + "/widgets/" + url.PathEscape(runtime.WidgetID) + "#runtime=" + url.QueryEscape(runtime.RuntimeInstanceID)
	return RuntimeLaunch{Context: runtime, RuntimeURL: s.runtimeOrigin + path, RuntimeOrigin: s.runtimeOrigin, Token: token, ExpiresAt: expires, Capabilities: capabilities}, nil
}

func (s *RuntimeService) RuntimeOrigin() string { return s.runtimeOrigin }
func (s *RuntimeService) HostOrigin() string    { return s.hostOrigin }

// EntrypointResource resolves immutable launch-page metadata. It intentionally
// does not grant runtime authority; only PrepareWidgetRuntime and Refresh do.
func (s *RuntimeService) EntrypointResource(ctx context.Context, release ReleaseIdentity, widgetID string) (Entrypoint, error) {
	stored, err := s.registry.Get(ctx, release.PluginID, release.Version)
	if err != nil || stored.Release.ArtifactDigest != release.ArtifactDigest {
		return Entrypoint{}, ErrNotFound
	}
	for _, entry := range stored.Manifest.Entrypoints {
		if entry.ID == widgetID {
			resource, resourceErr := s.Resource(ctx, release, entry.Resource)
			if resourceErr != nil || resource.SHA256 != declaredDigest(stored.Manifest, entry.Resource) {
				return Entrypoint{}, ErrResourceInvalid
			}
			return entry, nil
		}
	}
	return Entrypoint{}, ErrNotFound
}

func validateRuntimeContext(value RuntimeContext) error {
	if _, err := uuid.Parse(value.RuntimeInstanceID); err != nil || !pluginIDPattern.MatchString(string(value.PluginID)) || validateVersion(value.PluginVersion) != nil || !validDigest(value.ArtifactDigest) || !widgetIDPattern.MatchString(value.WidgetID) || value.WidgetType != TypeCourseWidget && value.WidgetType != TypeDashboardWidget {
		return ErrInvalidRuntimeToken
	}
	return nil
}

func validCapabilities(values []string) bool {
	if len(values) == 0 || len(values) > 2 {
		return false
	}
	seen := map[string]bool{}
	for _, value := range values {
		if value != CapabilityBootstrap && value != CapabilityContextRead || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func exactOrigin(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", ErrInvalidRuntimeConfiguration
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func declaredDigest(manifest Manifest, path string) string {
	for _, resource := range manifest.Resources {
		if resource.Path == path {
			return resource.SHA256
		}
	}
	return ""
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func RandomRuntimeSigningSeed() ([]byte, error) {
	seed := make([]byte, ed25519.SeedSize)
	_, err := rand.Read(seed)
	return seed, err
}
