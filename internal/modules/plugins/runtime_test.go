package plugins

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type courseRuntimeVersionFake struct {
	value courses.ImmutableCourseVersion
}

func (f courseRuntimeVersionFake) GetPublishedImmutableCourseVersionByCourseAndVersion(_ context.Context, courseID courses.CourseID, version courses.Version) (courses.ImmutableCourseVersion, error) {
	if f.value.CourseVersion.CourseID != courseID || f.value.CourseVersion.Version != version {
		return courses.ImmutableCourseVersion{}, courses.ErrNotFound
	}
	return f.value, nil
}

func runtimeFixture(t *testing.T, policy Policy) (*RegistryService, *RuntimeService, *RuntimeTokenService, InstalledRelease) {
	t.Helper()
	ctx := context.Background()
	repository := newMemoryRepository()
	registry := NewRegistryService(repository, policy)
	public, private, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{31}, 64)))
	key := VerificationKey{ID: "owned-runtime-2026", PublicKey: public, Purpose: KeyPurposeOwned, AllowedPluginIDs: []PluginID{"com.example.academy.timeline"}, Enabled: true}
	if err := repository.RegisterKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	archive, err := BuildPackageForTesting(testManifest("1.0.0"), map[string][]byte{"resources/timeline.js": []byte("export function initialize() {}")}, key.ID, private)
	if err != nil {
		t.Fatal(err)
	}
	release, _, err := registry.Register(ctx, readTestPackage(t, archive))
	if err != nil {
		t.Fatal(err)
	}
	release, err = registry.Enable(ctx, release.Release.PluginID, release.Release.Version)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := NewRuntimeTokenService("widget-runtime-2026", bytes.Repeat([]byte{18}, ed25519.SeedSize), DefaultTokenLifetime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntimeService(registry, tokens, "https://plugins.academy.example", "https://academy.example")
	if err != nil {
		t.Fatal(err)
	}
	return registry, runtime, tokens, release
}

func TestRuntimeLaunchUsesFreshIdentityAndExplicitCapabilities(t *testing.T) {
	_, runtime, _, release := runtimeFixture(t, Policy{})
	first, err := runtime.PrepareWidgetRuntime(context.Background(), release.Release.PluginID, release.Release.Version, "timeline", TypeCourseWidget)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtime.PrepareWidgetRuntime(context.Background(), release.Release.PluginID, release.Release.Version, "timeline", TypeCourseWidget)
	if err != nil {
		t.Fatal(err)
	}
	if first.Context.RuntimeInstanceID == second.Context.RuntimeInstanceID || first.RuntimeOrigin != "https://plugins.academy.example" || !strings.Contains(first.RuntimeURL, release.Release.ArtifactDigest) {
		t.Fatalf("invalid launch descriptors: %#v %#v", first, second)
	}
	if first.WidgetName != "Timeline" {
		t.Fatalf("trusted widget name = %q", first.WidgetName)
	}
	if len(first.Capabilities) != 2 || first.Capabilities[0] != CapabilityBootstrap || first.Capabilities[1] != CapabilityContextRead {
		t.Fatalf("unexpected grants: %v", first.Capabilities)
	}
	if _, err := runtime.PrepareWidgetRuntime(context.Background(), release.Release.PluginID, release.Release.Version, "timeline", TypeDashboardWidget); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("widget type mismatch = %v", err)
	}
}

func TestCourseWidgetRuntimePinsPlacementContextAndGrant(t *testing.T) {
	_, runtime, tokens, release := runtimeFixture(t, Policy{})
	placement := CourseWidgetRuntimePlacement{
		Placement: courses.PluginWidgetBlockPayload{PluginID: string(release.Release.PluginID), PluginVersion: release.Release.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "timeline", WidgetType: string(TypeCourseWidget), Configuration: json.RawMessage(`{"theme":"light"}`)},
		Context:   CourseWidgetRuntimeContext{CourseID: "11111111-1111-4111-8111-111111111111", CourseVersionID: "22222222-2222-4222-8222-222222222222", CourseVersion: "1.0.0", LessonKey: "lesson-one", PlacementKey: "timeline-widget", PresentationLanguage: "en", Configuration: json.RawMessage(`{"theme":"light"}`)},
	}
	launch, err := runtime.PrepareCourseWidgetRuntime(context.Background(), placement)
	if err != nil {
		t.Fatal(err)
	}
	if launch.CourseContext == nil || launch.CourseContext.CourseID != placement.Context.CourseID || !hasCapability(launch.Capabilities, CapabilityCourseContextRead) {
		t.Fatalf("course launch omitted pinned context/grant: %#v", launch)
	}
	if _, err := tokens.Verify(launch.Token, RuntimeTokenExpectation{Capability: CapabilityCourseContextRead}); err != nil {
		t.Fatalf("course capability missing: %v", err)
	}
	if hasCapability(launch.Capabilities, CapabilityDashboardContextRead) {
		t.Fatalf("course runtime received Dashboard context capability: %v", launch.Capabilities)
	}
	if _, err := runtime.DashboardContext(launch.Token); !errors.Is(err, ErrRuntimeCapabilityDenied) {
		t.Fatalf("course token read Dashboard context: %v", err)
	}
	placementContext, err := runtime.CourseContext(launch.Token)
	if err != nil || string(placementContext.Configuration) != `{"theme":"light"}` {
		t.Fatalf("context = %#v, %v", placementContext, err)
	}
	generic, _ := runtime.PrepareWidgetRuntime(context.Background(), release.Release.PluginID, release.Release.Version, "timeline", TypeCourseWidget)
	if _, err := runtime.CourseContext(generic.Token); !errors.Is(err, ErrRuntimeCapabilityDenied) {
		t.Fatalf("generic launch received Course context: %v", err)
	}
}

func TestPublishedCourseWidgetLifecycleKeepsPinnedCourseImmutable(t *testing.T) {
	ctx := context.Background()
	registry, runtime, _, release := runtimeFixture(t, Policy{})
	version, err := courses.ParseVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	published := courses.ImmutableCourseVersion{
		ID:            "22222222-2222-4222-8222-222222222222",
		CourseVersion: courses.CourseVersionInput{CourseID: "11111111-1111-4111-8111-111111111111", Version: version, Status: courses.CourseVersionPublished, SourceLanguage: "en", Title: "Plugin course"},
		Modules:       []courses.ImmutableCourseVersionModule{{StableKey: "module-one", Position: 0, Lessons: []courses.ImmutableCourseVersionLesson{{StableKey: "lesson-one", Position: 0, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "timeline-widget", Type: courses.BlockPluginWidget, Payload: courses.PluginWidgetBlockPayload{PluginID: string(release.Release.PluginID), PluginVersion: release.Release.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "timeline", WidgetType: string(TypeCourseWidget), Configuration: json.RawMessage(`{"theme":"light"}`)}}}}}}}},
	}
	before, _ := json.Marshal(published)
	launches := NewCourseWidgetLaunchService(courseRuntimeVersionFake{value: published}, runtime)
	launch, err := launches.Prepare(ctx, published.CourseVersion.CourseID, version, "lesson-one", "timeline-widget")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Disable(ctx, release.Release.PluginID, release.Release.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := launches.Prepare(ctx, published.CourseVersion.CourseID, version, "lesson-one", "timeline-widget"); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("disabled release launched from published CourseVersion: %v", err)
	}
	if _, err := runtime.Refresh(ctx, launch.Token); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("disabled Course runtime refreshed: %v", err)
	}
	if _, err := runtime.CourseContext(launch.Token); err != nil {
		t.Fatalf("existing short-lived Course token stopped immediately: %v", err)
	}
	if _, err := registry.Enable(ctx, release.Release.PluginID, release.Release.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.SetKeyState(ctx, "owned-runtime-2026", false); err != nil {
		t.Fatal(err)
	}
	if _, err := launches.Prepare(ctx, published.CourseVersion.CourseID, version, "lesson-one", "timeline-widget"); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("untrusted release launched from published CourseVersion: %v", err)
	}
	if _, err := registry.SetKeyState(ctx, "owned-runtime-2026", true); err != nil {
		t.Fatal(err)
	}
	if restored, err := launches.Prepare(ctx, published.CourseVersion.CourseID, version, "lesson-one", "timeline-widget"); err != nil || restored.Token == "" {
		t.Fatalf("future Course launch did not recover: %#v %v", restored, err)
	}
	after, _ := json.Marshal(published)
	if !bytes.Equal(before, after) || !strings.Contains(string(after), release.Release.ArtifactDigest) || !strings.Contains(string(after), `"timeline-widget"`) {
		t.Fatalf("plugin lifecycle rewrote immutable CourseVersion: %s", after)
	}
}

func TestRuntimeTokenStrictVerification(t *testing.T) {
	_, runtime, tokens, release := runtimeFixture(t, Policy{})
	launch, _ := runtime.PrepareWidgetRuntime(context.Background(), release.Release.PluginID, release.Release.Version, "timeline", TypeCourseWidget)
	base := RuntimeTokenExpectation{RuntimeInstanceID: launch.Context.RuntimeInstanceID, PluginID: release.Release.PluginID, PluginVersion: release.Release.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "timeline", WidgetType: TypeCourseWidget, Capability: CapabilityContextRead}
	if _, err := tokens.Verify(launch.Token, base); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*RuntimeTokenExpectation){
		"runtime": func(v *RuntimeTokenExpectation) { v.RuntimeInstanceID = "22222222-2222-4222-8222-222222222222" },
		"plugin":  func(v *RuntimeTokenExpectation) { v.PluginID = "com.example.other.widget" },
		"version": func(v *RuntimeTokenExpectation) { v.PluginVersion = "2.0.0" },
		"digest":  func(v *RuntimeTokenExpectation) { v.ArtifactDigest = strings.Repeat("b", 64) },
		"widget":  func(v *RuntimeTokenExpectation) { v.WidgetID = "other" },
		"type":    func(v *RuntimeTokenExpectation) { v.WidgetType = TypeDashboardWidget },
	} {
		t.Run(name, func(t *testing.T) {
			expected := base
			mutate(&expected)
			if _, err := tokens.Verify(launch.Token, expected); !errors.Is(err, ErrInvalidRuntimeToken) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if _, err := tokens.Verify(launch.Token, RuntimeTokenExpectation{Capability: "course.write"}); !errors.Is(err, ErrRuntimeCapabilityDenied) {
		t.Fatalf("missing capability = %v", err)
	}

	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	tokens.now = func() time.Time { return now }
	token, _, _ := tokens.Issue(launch.Context, []string{CapabilityBootstrap})
	tokens.now = func() time.Time { return now.Add(DefaultTokenLifetime) }
	if _, err := tokens.Verify(token, RuntimeTokenExpectation{}); !errors.Is(err, ErrRuntimeTokenExpired) {
		t.Fatalf("expired token = %v", err)
	}

	parts := strings.Split(launch.Token, ".")
	claimBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var wrongAudience RuntimeClaims
	_ = json.Unmarshal(claimBytes, &wrongAudience)
	wrongAudience.Audience = "academy-session"
	claimBytes, _ = json.Marshal(wrongAudience)
	unsigned := parts[0] + "." + base64.RawURLEncoding.EncodeToString(claimBytes)
	wrongAudienceToken := unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(tokens.private, []byte(unsigned)))
	if _, err := tokens.Verify(wrongAudienceToken, RuntimeTokenExpectation{}); !errors.Is(err, ErrInvalidRuntimeToken) {
		t.Fatalf("wrong audience token = %v", err)
	}
	wrongAudience.Issuer = "other-academy"
	wrongAudience.Audience = RuntimeAudience
	claimBytes, _ = json.Marshal(wrongAudience)
	unsigned = parts[0] + "." + base64.RawURLEncoding.EncodeToString(claimBytes)
	wrongIssuerToken := unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(tokens.private, []byte(unsigned)))
	if _, err := tokens.Verify(wrongIssuerToken, RuntimeTokenExpectation{}); !errors.Is(err, ErrInvalidRuntimeToken) {
		t.Fatalf("wrong issuer token = %v", err)
	}
}

func TestUnknownStrictPolicyCannotLaunch(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryRepository()
	registry := NewRegistryService(repository, Policy{})
	archive, _ := BuildPackageForTesting(testManifest("9.0.0"), map[string][]byte{"resources/timeline.js": []byte("unsigned")}, "", nil)
	release, _, err := registry.Register(ctx, readTestPackage(t, archive))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Enable(ctx, release.Release.PluginID, release.Release.Version); !errors.Is(err, ErrEnableDenied) {
		t.Fatalf("unknown enabled under strict policy: %v", err)
	}
	if _, err := registry.RuntimeRelease(ctx, release.Release.PluginID, release.Release.Version); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("unknown launched under strict policy: %v", err)
	}
}

func TestDisableBlocksLaunchAndRefreshButExistingTokenRemainsValid(t *testing.T) {
	registry, runtime, _, release := runtimeFixture(t, Policy{})
	launch, _ := runtime.PrepareWidgetRuntime(context.Background(), release.Release.PluginID, release.Release.Version, "timeline", TypeCourseWidget)
	if _, err := registry.Disable(context.Background(), release.Release.PluginID, release.Release.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.PrepareWidgetRuntime(context.Background(), release.Release.PluginID, release.Release.Version, "timeline", TypeCourseWidget); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("new launch = %v", err)
	}
	if _, err := runtime.Refresh(context.Background(), launch.Token); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("refresh = %v", err)
	}
	if _, err := runtime.VerifyContextToken(launch.Token); err != nil {
		t.Fatalf("existing short-lived token stopped immediately: %v", err)
	}
}

func TestApprovalRevocationBlocksNewLaunchAndRefresh(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryRepository()
	registry := NewRegistryService(repository, Policy{})
	public, private, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{41}, 64)))
	key := VerificationKey{ID: "approval-runtime-2026", PublicKey: public, Purpose: KeyPurposeApproval, Enabled: true}
	if err := repository.RegisterKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	archive, _ := BuildPackageForTesting(testManifest("4.0.0"), map[string][]byte{"resources/timeline.js": []byte("export const initialize=()=>{}")}, key.ID, private)
	release, _, err := registry.Register(ctx, readTestPackage(t, archive))
	if err != nil {
		t.Fatal(err)
	}
	approval, err := registry.Approve(ctx, release.Release, ApprovalBTGRelease, key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = registry.Enable(ctx, release.Release.PluginID, release.Release.Version); err != nil {
		t.Fatal(err)
	}
	tokens, _ := NewRuntimeTokenService("widget-runtime-2026", bytes.Repeat([]byte{42}, ed25519.SeedSize), DefaultTokenLifetime)
	runtime, _ := NewRuntimeService(registry, tokens, "https://plugins.academy.example", "https://academy.example")
	launch, err := runtime.PrepareWidgetRuntime(ctx, release.Release.PluginID, release.Release.Version, "timeline", TypeCourseWidget)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.RevokeApproval(ctx, approval.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.PrepareWidgetRuntime(ctx, release.Release.PluginID, release.Release.Version, "timeline", TypeCourseWidget); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("launch after revocation = %v", err)
	}
	if _, err := runtime.Refresh(ctx, launch.Token); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("refresh after revocation = %v", err)
	}
	if _, err := runtime.VerifyContextToken(launch.Token); err != nil {
		t.Fatalf("existing token did not retain bounded lifetime: %v", err)
	}
}

func TestTrustedKeyDisableBlocksNewLaunchAndRefresh(t *testing.T) {
	registry, runtime, _, release := runtimeFixture(t, Policy{})
	launch, _ := runtime.PrepareWidgetRuntime(context.Background(), release.Release.PluginID, release.Release.Version, "timeline", TypeCourseWidget)
	if err := registry.SetKeyEnabled(context.Background(), "owned-runtime-2026", false); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.PrepareWidgetRuntime(context.Background(), release.Release.PluginID, release.Release.Version, "timeline", TypeCourseWidget); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("launch after key disable = %v", err)
	}
	if _, err := runtime.Refresh(context.Background(), launch.Token); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("refresh after key disable = %v", err)
	}
	current, err := registry.Get(context.Background(), release.Release.PluginID, release.Release.Version)
	if err != nil || current.CurrentTrust != TrustUnknown || !current.Enabled {
		t.Fatalf("key disable collapsed trust and stored enablement: %#v %v", current, err)
	}
	if err := registry.SetKeyEnabled(context.Background(), "owned-runtime-2026", true); err != nil {
		t.Fatal(err)
	}
	restored, err := registry.Get(context.Background(), release.Release.PluginID, release.Release.Version)
	if err != nil || restored.CurrentTrust != TrustBTGOwned || !restored.Enabled {
		t.Fatalf("owned trust did not recompute after key re-enable: %#v %v", restored, err)
	}
	if _, err := runtime.PrepareWidgetRuntime(context.Background(), release.Release.PluginID, release.Release.Version, "timeline", TypeCourseWidget); err != nil {
		t.Fatalf("new launch after key re-enable = %v", err)
	}
}

func TestResourceServingDetectsMutationAndRejectsUndeclaredPath(t *testing.T) {
	registry, runtime, _, release := runtimeFixture(t, Policy{})
	resource, err := runtime.Resource(context.Background(), release.Release, "resources/timeline.js")
	if err != nil || len(resource.Content) == 0 {
		t.Fatalf("resource = %#v, %v", resource, err)
	}
	if _, err := runtime.Resource(context.Background(), release.Release, "resources/other.js"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("undeclared resource = %v", err)
	}
	repository := registry.repository.(*memoryRepository)
	key := releaseKey(release.Release.PluginID, release.Release.Version) + ":resources/timeline.js"
	mutated := repository.resources[key]
	mutated.Content = []byte("tampered")
	repository.resources[key] = mutated
	if _, err := runtime.Resource(context.Background(), release.Release, "resources/timeline.js"); !errors.Is(err, ErrResourceInvalid) {
		t.Fatalf("mutated resource = %v", err)
	}
}

func TestRuntimeRequiresDistinctOrigins(t *testing.T) {
	registry, _, tokens, _ := runtimeFixture(t, Policy{})
	if _, err := NewRuntimeService(registry, tokens, "https://academy.example", "https://academy.example"); !errors.Is(err, ErrInvalidRuntimeConfiguration) {
		t.Fatalf("same origin accepted: %v", err)
	}
}
