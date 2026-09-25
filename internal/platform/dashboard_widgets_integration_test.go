//go:build integration

package platform

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
	pluginspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"testing"
	"time"
)

func testDashboardWidgetPlacements(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	repo := pluginspostgres.New(pool)
	registry := plugins.NewRegistryService(repo, plugins.Policy{})
	pub, priv, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{91}, 64)))
	manifest := plugins.Manifest{Format: plugins.PackageFormat, FormatVersion: 1, ID: "eu.com.bridgingthegap.widgets.dashboard", Version: "1.0.0", Name: "Dashboard", Description: "test", Publisher: plugins.Publisher{Name: "BTG"}, PluginTypes: []plugins.PluginType{plugins.TypeDashboardWidget}, Entrypoints: []plugins.Entrypoint{{ID: "dashboard", Type: plugins.TypeDashboardWidget, Name: "Dashboard", Resource: "resources/widget.js"}}, Permissions: []plugins.Permission{plugins.PermissionNone}}
	if err := repo.RegisterKey(ctx, plugins.VerificationKey{ID: "dashboard-owned-2026", PublicKey: pub, Purpose: plugins.KeyPurposeOwned, AllowedPluginIDs: []plugins.PluginID{manifest.ID}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	archive, err := plugins.BuildPackageForTesting(manifest, map[string][]byte{"resources/widget.js": []byte("export default {}")}, "dashboard-owned-2026", priv)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := plugins.NewPackageReader(plugins.Limits{}).Read(ctx, bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	release, _, err := registry.Register(ctx, pkg)
	if err != nil {
		t.Fatal(err)
	}
	release, err = registry.Enable(ctx, manifest.ID, manifest.Version)
	if err != nil {
		t.Fatal(err)
	}
	service := plugins.NewDashboardPlacementService(registry, repo, func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	create := func(config string) plugins.DashboardPlacement {
		value, err := service.Create(ctx, plugins.DashboardPlacement{PluginID: manifest.ID, PluginVersion: manifest.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "dashboard", Configuration: []byte(config)})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	first := create(`{"one":1}`)
	second := create(`{"two":2}`)
	if first.Position != 0 || second.Position != 1 || first.Revision != 1 || !bytes.Contains(first.Configuration, []byte(`"one"`)) {
		t.Fatalf("creation=%#v %#v", first, second)
	}
	second.Configuration = []byte(`{"updated":true}`)
	updated, err := service.Update(ctx, second, second.Revision)
	if err != nil || updated.Revision != 2 {
		t.Fatalf("update=%#v %v", updated, err)
	}
	if _, err := service.Update(ctx, second, second.Revision); !errors.Is(err, plugins.ErrDashboardPlacementConflict) {
		t.Fatalf("stale update=%v", err)
	}
	values, _ := service.List(ctx)
	revisions := map[string]int64{values[0].ID: values[0].Revision, values[1].ID: values[1].Revision}
	ordered, err := service.Reorder(ctx, []string{values[1].ID, values[0].ID}, revisions)
	if err != nil || ordered[0].ID != second.ID || ordered[0].Position != 0 || ordered[1].Position != 1 {
		t.Fatalf("reorder=%#v %v", ordered, err)
	}
	stale := map[string]int64{ordered[0].ID: ordered[0].Revision, ordered[1].ID: 1}
	if _, err := service.Reorder(ctx, []string{ordered[1].ID, ordered[0].ID}, stale); !errors.Is(err, plugins.ErrDashboardPlacementConflict) {
		t.Fatalf("stale reorder=%v", err)
	}
	if err := service.Delete(ctx, ordered[0].ID, ordered[0].Revision); err != nil {
		t.Fatal(err)
	}
	left, _ := service.List(ctx)
	if len(left) != 1 || left[0].ID != first.ID {
		t.Fatalf("delete=%#v", left)
	}
	// Exact release pinning is placement eligibility, not client metadata.
	for _, invalid := range []plugins.DashboardPlacement{
		{PluginID: manifest.ID, PluginVersion: manifest.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "missing", Configuration: []byte(`{}`)},
		{PluginID: manifest.ID, PluginVersion: manifest.Version, ArtifactDigest: strings.Repeat("0", 64), WidgetID: "dashboard", Configuration: []byte(`{}`)},
	} {
		if _, err := service.Create(ctx, invalid); !errors.Is(err, plugins.ErrLaunchDenied) {
			t.Fatalf("invalid placement accepted: %v", err)
		}
	}
	if _, err := registry.Disable(ctx, manifest.ID, manifest.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, plugins.DashboardPlacement{PluginID: manifest.ID, PluginVersion: manifest.Version, ArtifactDigest: release.Release.ArtifactDigest, WidgetID: "dashboard", Configuration: []byte(`{}`)}); !errors.Is(err, plugins.ErrLaunchDenied) {
		t.Fatalf("disabled release placed: %v", err)
	}
}
