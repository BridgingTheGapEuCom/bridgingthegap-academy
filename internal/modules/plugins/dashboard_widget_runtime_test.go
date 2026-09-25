package plugins

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type dashboardPlacementReaderFake struct {
	placements map[string]DashboardPlacement
	err        error
}

func (r *dashboardPlacementReaderFake) GetDashboardPlacement(_ context.Context, id string) (DashboardPlacement, error) {
	if r.err != nil {
		return DashboardPlacement{}, r.err
	}
	placement, ok := r.placements[id]
	if !ok {
		return DashboardPlacement{}, ErrDashboardPlacementNotFound
	}
	placement.Configuration = append(json.RawMessage(nil), placement.Configuration...)
	return placement, nil
}

func dashboardRuntimeFixture(t *testing.T) (*RegistryService, *RuntimeService, *RuntimeTokenService, *memoryRepository, *dashboardPlacementReaderFake, InstalledRelease, *DashboardWidgetLaunchService) {
	t.Helper()
	ctx := context.Background()
	repository := newMemoryRepository()
	registry := NewRegistryService(repository, Policy{})
	public, private, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{61}, 64)))
	pluginID := PluginID("com.example.academy.dashboard-runtime")
	key := VerificationKey{ID: "dashboard-runtime-owned-2026", PublicKey: public, Purpose: KeyPurposeOwned, AllowedPluginIDs: []PluginID{pluginID}, Enabled: true}
	if err := repository.RegisterKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{Format: PackageFormat, FormatVersion: 1, ID: pluginID, Version: "1.0.0", Name: "Dashboard runtime", Description: "Runtime test", Publisher: Publisher{Name: "Example"}, PluginTypes: []PluginType{TypeDashboardWidget, TypeCourseWidget}, Entrypoints: []Entrypoint{{ID: "summary", Type: TypeDashboardWidget, Name: "Summary", Resource: "resources/summary.js"}, {ID: "course", Type: TypeCourseWidget, Name: "Course", Resource: "resources/course.js"}}, Permissions: []Permission{PermissionNone}}
	archive, err := BuildPackageForTesting(manifest, map[string][]byte{"resources/summary.js": []byte("export function initialize() {}"), "resources/course.js": []byte("export default {}")}, key.ID, private)
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
	tokens, err := NewRuntimeTokenService("dashboard-runtime-token-2026", bytes.Repeat([]byte{62}, ed25519.SeedSize), DefaultTokenLifetime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntimeService(registry, tokens, "https://plugins.academy.example", "https://academy.example")
	if err != nil {
		t.Fatal(err)
	}
	placementID := "11111111-1111-4111-8111-111111111111"
	placements := &dashboardPlacementReaderFake{placements: map[string]DashboardPlacement{placementID: {ID: placementID, PluginID: release.Release.PluginID, PluginVersion: release.Release.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "summary", Configuration: json.RawMessage(`{"theme":"light"}`), Enabled: true, Revision: 1}}}
	launches := NewDashboardWidgetLaunchService(placements, runtime)
	return registry, runtime, tokens, repository, placements, release, launches
}

func TestDashboardRuntimeDerivesExactPlacementAndNarrowContext(t *testing.T) {
	_, runtime, tokens, _, placements, release, launches := dashboardRuntimeFixture(t)
	ctx := context.Background()
	placementID := "11111111-1111-4111-8111-111111111111"
	launch, err := launches.Prepare(ctx, placementID)
	if err != nil {
		t.Fatal(err)
	}
	if launch.Context.PluginID != release.Release.PluginID || launch.Context.PluginVersion != release.Release.Version || launch.Context.ArtifactDigest != release.Release.ArtifactDigest || launch.Context.WidgetID != "summary" || launch.Context.WidgetType != TypeDashboardWidget {
		t.Fatalf("launch was not placement-derived: %#v", launch.Context)
	}
	if launch.DashboardContext == nil || launch.DashboardContext.PlacementID != placementID || string(launch.DashboardContext.Configuration) != `{"theme":"light"}` {
		t.Fatalf("dashboard context = %#v", launch.DashboardContext)
	}
	for _, capability := range []string{CapabilityDashboardContextRead} {
		if !hasCapability(launch.Capabilities, capability) {
			t.Fatalf("missing capability %q: %v", capability, launch.Capabilities)
		}
	}
	for _, forbidden := range []string{CapabilityCourseContextRead, "dashboard.widgets.manage", "plugins.manage"} {
		if hasCapability(launch.Capabilities, forbidden) {
			t.Fatalf("unexpected capability %q: %v", forbidden, launch.Capabilities)
		}
	}
	if _, err := tokens.Verify(launch.Token, RuntimeTokenExpectation{RuntimeInstanceID: launch.Context.RuntimeInstanceID, PluginID: release.Release.PluginID, PluginVersion: release.Release.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "summary", WidgetType: TypeDashboardWidget, Capability: CapabilityDashboardContextRead}); err != nil {
		t.Fatal(err)
	}
	contextValue, err := runtime.DashboardContext(launch.Token)
	if err != nil || contextValue.PlacementID != placementID {
		t.Fatalf("context=%#v err=%v", contextValue, err)
	}
	encoded, _ := json.Marshal(contextValue)
	for _, forbidden := range []string{"user", "email", "role", "session", "course", "progress", "assessment", "certificate", "community", "capabilities"} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Fatalf("Dashboard context leaked %q: %s", forbidden, encoded)
		}
	}

	// Context is a launch-time snapshot, matching Course runtime semantics. A
	// placement edit affects the next launch, while this runtime retains its
	// original bounded context until its token expires.
	changed := placements.placements[placementID]
	changed.Configuration = json.RawMessage(`{"theme":"dark"}`)
	placements.placements[placementID] = changed
	refreshed, err := runtime.Refresh(ctx, launch.Token)
	if err != nil || refreshed.DashboardContext == nil || string(refreshed.DashboardContext.Configuration) != `{"theme":"light"}` {
		t.Fatalf("refresh changed launch snapshot: %#v %v", refreshed, err)
	}
	next, err := launches.Prepare(ctx, placementID)
	if err != nil || next.DashboardContext == nil || string(next.DashboardContext.Configuration) != `{"theme":"dark"}` {
		t.Fatalf("new launch missed authoritative config: %#v %v", next, err)
	}
	otherID := "22222222-2222-4222-8222-222222222222"
	other := changed
	other.ID = otherID
	other.Configuration = json.RawMessage(`{"scope":"other"}`)
	placements.placements[otherID] = other
	otherLaunch, err := launches.Prepare(ctx, otherID)
	if err != nil {
		t.Fatal(err)
	}
	if contextValue, err := runtime.DashboardContext(launch.Token); err != nil || contextValue.PlacementID != placementID || string(contextValue.Configuration) == string(other.Configuration) {
		t.Fatalf("runtime A crossed into placement B: %#v %v", contextValue, err)
	}
	if _, err := tokens.Verify(launch.Token, RuntimeTokenExpectation{RuntimeInstanceID: otherLaunch.Context.RuntimeInstanceID}); !errors.Is(err, ErrInvalidRuntimeToken) {
		t.Fatalf("runtime A token matched runtime B: %v", err)
	}

	now := tokens.now()
	tokens.now = func() time.Time { return now.Add(DefaultTokenLifetime) }
	if _, err := runtime.DashboardContext(launch.Token); !errors.Is(err, ErrRuntimeTokenExpired) {
		t.Fatalf("Dashboard token did not expire normally: %v", err)
	}
}

func TestDashboardRuntimePlacementAndReleaseLifecycle(t *testing.T) {
	registry, runtime, _, repository, placements, release, launches := dashboardRuntimeFixture(t)
	ctx := context.Background()
	id := "11111111-1111-4111-8111-111111111111"
	launch, err := launches.Prepare(ctx, id)
	if err != nil {
		t.Fatal(err)
	}

	placement := placements.placements[id]
	placement.Enabled = false
	placements.placements[id] = placement
	if _, err := launches.Prepare(ctx, id); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("disabled placement launch=%v", err)
	}
	if _, err := runtime.Refresh(ctx, launch.Token); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("disabled placement refresh=%v", err)
	}
	if _, err := runtime.DashboardContext(launch.Token); err != nil {
		t.Fatalf("existing bounded token stopped early: %v", err)
	}

	placement.Enabled = true
	placements.placements[id] = placement
	replacement, _ := launches.Prepare(ctx, id)
	delete(placements.placements, id)
	if _, err := runtime.Refresh(ctx, replacement.Token); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("deleted placement refresh=%v", err)
	}
	if context, err := runtime.DashboardContext(replacement.Token); err != nil || context.PlacementID != id {
		t.Fatalf("deleted placement changed existing bounded context: %#v %v", context, err)
	}
	if _, err := launches.Prepare(ctx, id); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("deleted placement launch=%v", err)
	}

	placements.placements[id] = placement
	pluginLaunch, _ := launches.Prepare(ctx, id)
	if _, err := registry.Disable(ctx, release.Release.PluginID, release.Release.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Refresh(ctx, pluginLaunch.Token); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("disabled plugin refresh=%v", err)
	}

	// Restore the enabled release, then mutate the exact declared entrypoint.
	if _, err := registry.Enable(ctx, release.Release.PluginID, release.Release.Version); err != nil {
		t.Fatal(err)
	}
	integrityLaunch, _ := launches.Prepare(ctx, id)
	resourceKey := releaseKey(release.Release.PluginID, release.Release.Version) + ":resources/summary.js"
	resource := repository.resources[resourceKey]
	resource.Content = []byte("tampered")
	repository.resources[resourceKey] = resource
	if _, err := runtime.Refresh(ctx, integrityLaunch.Token); !errors.Is(err, ErrResourceInvalid) {
		t.Fatalf("mutated artifact refresh=%v", err)
	}
}

func TestDashboardRuntimeRejectsCorruptOrArbitraryPlacementIdentity(t *testing.T) {
	_, _, _, _, placements, release, launches := dashboardRuntimeFixture(t)
	ctx := context.Background()
	id := "11111111-1111-4111-8111-111111111111"
	original := placements.placements[id]
	for name, mutate := range map[string]func(*DashboardPlacement){
		"wrong digest":   func(value *DashboardPlacement) { value.ArtifactDigest = strings.Repeat("0", 64) },
		"missing widget": func(value *DashboardPlacement) { value.WidgetID = "missing" },
		"course widget":  func(value *DashboardPlacement) { value.WidgetID = "course" },
		"wrong plugin":   func(value *DashboardPlacement) { value.PluginID = "com.example.academy.other" },
	} {
		t.Run(name, func(t *testing.T) {
			value := original
			mutate(&value)
			placements.placements[id] = value
			if _, err := launches.Prepare(ctx, id); err == nil {
				t.Fatal("corrupt placement launched")
			}
			placements.placements[id] = original
		})
	}
	if _, err := launches.Prepare(ctx, "22222222-2222-4222-8222-222222222222"); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("arbitrary placement launch=%v", err)
	}
	if _, err := launches.Prepare(ctx, "not-a-placement-id"); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("malformed placement launch=%v", err)
	}
	if original.PluginID != release.Release.PluginID {
		t.Fatal("fixture did not use authoritative release")
	}
}

func TestDashboardRuntimeApprovalRevocationBlocksRefresh(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryRepository()
	registry := NewRegistryService(repository, Policy{})
	public, private, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{71}, 64)))
	pluginID := PluginID("com.example.academy.approved-dashboard")
	key := VerificationKey{ID: "dashboard-approval-2026", PublicKey: public, Purpose: KeyPurposeApproval, Enabled: true}
	if err := repository.RegisterKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{Format: PackageFormat, FormatVersion: 1, ID: pluginID, Version: "1.0.0", Name: "Approved dashboard", Publisher: Publisher{Name: "Community"}, PluginTypes: []PluginType{TypeDashboardWidget}, Entrypoints: []Entrypoint{{ID: "approved", Type: TypeDashboardWidget, Name: "Approved", Resource: "resources/approved.js"}}, Permissions: []Permission{PermissionNone}}
	archive, _ := BuildPackageForTesting(manifest, map[string][]byte{"resources/approved.js": []byte("export default {}")}, key.ID, private)
	release, _, err := registry.Register(ctx, readTestPackage(t, archive))
	if err != nil {
		t.Fatal(err)
	}
	approval, err := registry.Approve(ctx, release.Release, ApprovalBTGRelease, key.ID)
	if err != nil {
		t.Fatal(err)
	}
	release, err = registry.Enable(ctx, pluginID, manifest.Version)
	if err != nil {
		t.Fatal(err)
	}
	tokens, _ := NewRuntimeTokenService("dashboard-approved-token-2026", bytes.Repeat([]byte{72}, ed25519.SeedSize), DefaultTokenLifetime)
	runtime, _ := NewRuntimeService(registry, tokens, "https://plugins.academy.example", "https://academy.example")
	id := "44444444-4444-4444-8444-444444444444"
	placements := &dashboardPlacementReaderFake{placements: map[string]DashboardPlacement{id: {ID: id, PluginID: pluginID, PluginVersion: manifest.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "approved", Configuration: json.RawMessage(`{}`), Enabled: true}}}
	launches := NewDashboardWidgetLaunchService(placements, runtime)
	launch, err := launches.Prepare(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.RevokeApproval(ctx, approval.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := launches.Prepare(ctx, id); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("launch after approval revocation=%v", err)
	}
	if _, err := runtime.Refresh(ctx, launch.Token); !errors.Is(err, ErrLaunchDenied) {
		t.Fatalf("refresh after approval revocation=%v", err)
	}
	if _, err := runtime.DashboardContext(launch.Token); err != nil {
		t.Fatalf("existing short-lived token stopped immediately: %v", err)
	}
}
