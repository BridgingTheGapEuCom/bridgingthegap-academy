//go:build integration

package platform

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
	pluginspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPluginRegistry(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	repo := pluginspostgres.New(pool)
	service := plugins.NewRegistryService(repo, plugins.Policy{AllowUnknown: true, RequireManualApprovalForUnknown: true})
	pub, private, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{12}, 64)))
	key := plugins.VerificationKey{ID: "btg-owned-2026", PublicKey: pub, Purpose: plugins.KeyPurposeOwned, AllowedPluginIDs: []plugins.PluginID{"eu.com.bridgingthegap.widgets.diagram"}, Enabled: true}
	if err := repo.RegisterKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	conflict := key
	conflict.PublicKey = append(ed25519.PublicKey(nil), pub...)
	conflict.PublicKey[0] ^= 1
	if err := repo.RegisterKey(ctx, conflict); !errors.Is(err, plugins.ErrKeyIDConflict) {
		t.Fatalf("key conflict got %v", err)
	}
	manifest := plugins.Manifest{Format: plugins.PackageFormat, FormatVersion: 1, ID: "eu.com.bridgingthegap.widgets.diagram", Version: "1.0.0", Name: "Diagram", Description: "Diagram widget", Publisher: plugins.Publisher{Name: "Bridging the Gap"}, PluginTypes: []plugins.PluginType{plugins.TypeCourseWidget}, Entrypoints: []plugins.Entrypoint{{ID: "diagram", Type: plugins.TypeCourseWidget, Name: "Diagram", Resource: "resources/diagram.js"}}, Permissions: []plugins.Permission{plugins.PermissionNone}}
	archive, err := plugins.BuildPackageForTesting(manifest, map[string][]byte{"resources/diagram.js": []byte("export default {}")}, key.ID, private)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := plugins.NewPackageReader(plugins.Limits{}).Read(ctx, bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	registered, created, err := service.Register(ctx, pkg)
	if err != nil || !created || registered.CurrentTrust != plugins.TrustBTGOwned {
		t.Fatalf("registered=%#v created=%v err=%v", registered, created, err)
	}
	// M10.1a releases predate persisted runtime bytes. Exact package replay is
	// the safe, digest-bound backfill path after migration 00032.
	if _, err := pool.Exec(ctx, "DELETE FROM plugins.installed_resource WHERE installation_id = $1", registered.InstallationID); err != nil {
		t.Fatal(err)
	}
	replayed, created, err := service.Register(ctx, pkg)
	if err != nil || created || replayed.InstallationID != registered.InstallationID {
		t.Fatalf("replay=%#v created=%v err=%v", replayed, created, err)
	}
	enabled, err := service.Enable(ctx, manifest.ID, manifest.Version)
	if err != nil || !enabled.Enabled || enabled.State != plugins.StateInstalled {
		t.Fatalf("enabled=%#v err=%v", enabled, err)
	}
	resource, err := repo.GetResource(ctx, enabled.Release, "resources/diagram.js")
	if err != nil || string(resource.Content) != "export default {}" {
		t.Fatalf("persisted resource=%#v err=%v", resource, err)
	}
	tokens, err := plugins.NewRuntimeTokenService("integration-runtime-key", bytes.Repeat([]byte{14}, ed25519.SeedSize), plugins.DefaultTokenLifetime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := plugins.NewRuntimeService(service, tokens, "https://plugins.academy.example", "https://academy.example")
	if err != nil {
		t.Fatal(err)
	}
	launch, err := runtime.PrepareWidgetRuntime(ctx, manifest.ID, manifest.Version, "diagram", plugins.TypeCourseWidget)
	if err != nil || !strings.Contains(launch.RuntimeURL, registered.Release.ArtifactDigest) {
		t.Fatalf("runtime launch=%#v err=%v", launch, err)
	}
	changed, _ := plugins.BuildPackageForTesting(manifest, map[string][]byte{"resources/diagram.js": []byte("changed")}, key.ID, private)
	changedPkg, _ := plugins.NewPackageReader(plugins.Limits{}).Read(ctx, bytes.NewReader(changed))
	if _, _, err := service.Register(ctx, changedPkg); !errors.Is(err, plugins.ErrReleaseConflict) {
		t.Fatalf("mutable release got %v", err)
	}
	if err := service.SetKeyEnabled(ctx, key.ID, false); err != nil {
		t.Fatal(err)
	}
	reevaluated, err := service.Get(ctx, manifest.ID, manifest.Version)
	if err != nil || reevaluated.CurrentTrust != plugins.TrustUnknown {
		t.Fatalf("reevaluated=%#v err=%v", reevaluated, err)
	}

	approvalPublic, approvalPrivate, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{13}, 64)))
	approvalKey := plugins.VerificationKey{ID: "btg-approval-2026", PublicKey: approvalPublic, Purpose: plugins.KeyPurposeApproval, Enabled: true}
	if err := repo.RegisterKey(ctx, approvalKey); err != nil {
		t.Fatal(err)
	}
	manifest.Version = "2.0.0"
	approvedArchive, _ := plugins.BuildPackageForTesting(manifest, map[string][]byte{"resources/diagram.js": []byte("external")}, approvalKey.ID, approvalPrivate)
	approvedPackage, _ := plugins.NewPackageReader(plugins.Limits{}).Read(ctx, bytes.NewReader(approvedArchive))
	external, _, err := service.Register(ctx, approvedPackage)
	if err != nil || external.CurrentTrust != plugins.TrustUnknown {
		t.Fatalf("pre-approval=%#v err=%v", external, err)
	}
	approval, err := service.Approve(ctx, external.Release, plugins.ApprovalBTGRelease, approvalKey.ID)
	if err != nil {
		t.Fatal(err)
	}
	external, err = service.Get(ctx, manifest.ID, manifest.Version)
	if err != nil || external.CurrentTrust != plugins.TrustBTGApproved {
		t.Fatalf("approved=%#v err=%v", external, err)
	}
	if err := service.RevokeApproval(ctx, approval.ID); err != nil {
		t.Fatal(err)
	}
	external, err = service.Get(ctx, manifest.ID, manifest.Version)
	if err != nil || external.CurrentTrust != plugins.TrustUnknown {
		t.Fatalf("revoked=%#v err=%v", external, err)
	}
}
