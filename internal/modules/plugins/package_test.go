package plugins

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"testing"
	"time"
)

func testManifest(version string) Manifest {
	return Manifest{Format: PackageFormat, FormatVersion: 1, ID: "com.example.academy.timeline", Version: version, Name: "Timeline", Description: "A timeline widget", Publisher: Publisher{Name: "Example"}, PluginTypes: []PluginType{TypeCourseWidget}, Entrypoints: []Entrypoint{{ID: "timeline", Type: TypeCourseWidget, Name: "Timeline", Resource: "resources/timeline.js"}}, Permissions: []Permission{PermissionNone}}
}
func readTestPackage(t *testing.T, b []byte) *ValidatedPluginPackage {
	t.Helper()
	p, err := NewPackageReader(Limits{}).Read(context.Background(), bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPackageManifestAndSignature(t *testing.T) {
	pub, private, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{7}, 64)))
	_ = pub
	b, err := BuildPackageForTesting(testManifest("1.4.2"), map[string][]byte{"resources/timeline.js": []byte("export default 1")}, "owned-2026", private)
	if err != nil {
		t.Fatal(err)
	}
	p := readTestPackage(t, b)
	if p.Manifest().ID != "com.example.academy.timeline" || p.ArtifactDigest() == "" {
		t.Fatalf("bad package: %#v", p.Manifest())
	}
	sig, ok := p.Signature()
	if !ok || sig.KeyID != "owned-2026" || !ed25519.Verify(pub, p.SigningPayload(), mustSignature(t, sig.Value)) {
		t.Fatal("signature did not bind canonical payload")
	}
}

func TestManifestSupportsBothWidgetTypes(t *testing.T) {
	m := testManifest("1.0.0")
	m.PluginTypes = []PluginType{TypeCourseWidget, TypeDashboardWidget}
	m.Entrypoints = append(m.Entrypoints, Entrypoint{ID: "dashboard-timeline", Type: TypeDashboardWidget, Name: "Dashboard timeline", Resource: "resources/dashboard.js"})
	b, err := BuildPackageForTesting(m, map[string][]byte{"resources/timeline.js": []byte("course"), "resources/dashboard.js": []byte("dashboard")}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	validated := readTestPackage(t, b)
	if len(validated.Manifest().PluginTypes) != 2 || len(validated.Manifest().Entrypoints) != 2 {
		t.Fatalf("both widget types were not preserved: %#v", validated.Manifest())
	}
}

func TestManifestRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{{"id", func(m *Manifest) { m.ID = "Timeline" }}, {"version", func(m *Manifest) { m.Version = "1.0" }}, {"type", func(m *Manifest) { m.PluginTypes = []PluginType{"SERVER_PLUGIN"} }}, {"duplicate widget", func(m *Manifest) { m.Entrypoints = append(m.Entrypoints, m.Entrypoints[0]) }}, {"format version", func(m *Manifest) { m.FormatVersion = 2 }}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := testManifest("1.0.0")
			tc.mutate(&m)
			_, err := BuildPackageForTesting(m, map[string][]byte{"resources/timeline.js": []byte("x")}, "", nil)
			if err == nil {
				t.Fatal("accepted invalid manifest")
			}
		})
	}
}

func TestPackageArchiveSecurity(t *testing.T) {
	valid, _ := BuildPackageForTesting(testManifest("1.0.0"), map[string][]byte{"resources/timeline.js": []byte("x")}, "", nil)
	t.Run("traversal", func(t *testing.T) {
		b := zipEntries(t, map[string][]byte{"../manifest.json": []byte("{}")})
		_, err := NewPackageReader(Limits{}).Read(context.Background(), bytes.NewReader(b))
		if !IsPackageError(err, ErrInvalidArchive) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("absolute path", func(t *testing.T) {
		for _, name := range []string{"/manifest.json", "C:/manifest.json", `C:\manifest.json`} {
			_, err := NewPackageReader(Limits{}).Read(context.Background(), bytes.NewReader(zipEntries(t, map[string][]byte{name: []byte("{}")})))
			if !IsPackageError(err, ErrInvalidArchive) {
				t.Fatalf("accepted absolute path %q: %v", name, err)
			}
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		var out bytes.Buffer
		z := zip.NewWriter(&out)
		for range 2 {
			w, _ := z.Create("manifest.json")
			_, _ = w.Write([]byte("{}"))
		}
		_ = z.Close()
		_, err := NewPackageReader(Limits{}).Read(context.Background(), bytes.NewReader(out.Bytes()))
		if !IsPackageError(err, ErrInvalidArchive) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("symlink", func(t *testing.T) {
		var out bytes.Buffer
		z := zip.NewWriter(&out)
		header := &zip.FileHeader{Name: "resources/link"}
		header.SetMode(0o777 | os.ModeSymlink)
		w, _ := z.CreateHeader(header)
		_, _ = w.Write([]byte("/etc/passwd"))
		_ = z.Close()
		_, err := NewPackageReader(Limits{}).Read(context.Background(), bytes.NewReader(out.Bytes()))
		if !IsPackageError(err, ErrInvalidArchive) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("oversized", func(t *testing.T) {
		_, err := NewPackageReader(Limits{MaxCompressedBytes: 8}).Read(context.Background(), bytes.NewReader(valid))
		if !IsPackageError(err, ErrPackageTooLarge) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("tampered resource", func(t *testing.T) {
		entries := unzipEntries(t, valid)
		entries["resources/timeline.js"] = []byte("changed")
		_, err := NewPackageReader(Limits{}).Read(context.Background(), bytes.NewReader(zipEntries(t, entries)))
		if !IsPackageError(err, ErrChecksumMismatch) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("unknown manifest field", func(t *testing.T) {
		entries := unzipEntries(t, valid)
		var raw map[string]any
		_ = json.Unmarshal(entries[manifestPath], &raw)
		raw["trust"] = "BTG_OWNED"
		entries[manifestPath], _ = json.Marshal(raw)
		_, err := NewPackageReader(Limits{}).Read(context.Background(), bytes.NewReader(zipEntries(t, entries)))
		if !IsPackageError(err, ErrInvalidManifest) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("duplicate manifest key", func(t *testing.T) {
		entries := unzipEntries(t, valid)
		entries[manifestPath] = []byte(`{"format":"bridging-the-gap-plugin","format":"bridging-the-gap-plugin"}`)
		_, err := NewPackageReader(Limits{}).Read(context.Background(), bytes.NewReader(zipEntries(t, entries)))
		if !IsPackageError(err, ErrInvalidManifest) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("malformed manifest", func(t *testing.T) {
		entries := unzipEntries(t, valid)
		entries[manifestPath] = []byte(`{"format":`)
		_, err := NewPackageReader(Limits{}).Read(context.Background(), bytes.NewReader(zipEntries(t, entries)))
		if !IsPackageError(err, ErrInvalidManifest) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("trailing manifest value", func(t *testing.T) {
		entries := unzipEntries(t, valid)
		entries[manifestPath] = append(entries[manifestPath], []byte(` {}`)...)
		_, err := NewPackageReader(Limits{}).Read(context.Background(), bytes.NewReader(zipEntries(t, entries)))
		if !IsPackageError(err, ErrInvalidManifest) {
			t.Fatalf("got %v", err)
		}
	})
}

func TestRecognizedSignatureRejectsManifestTamperingAndWrongKey(t *testing.T) {
	ctx := context.Background()
	publicKey, privateKey, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{21}, 64)))
	archive, err := BuildPackageForTesting(testManifest("3.0.0"), map[string][]byte{"resources/timeline.js": []byte("signed")}, "owned-2026", privateKey)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("tampered manifest", func(t *testing.T) {
		entries := unzipEntries(t, archive)
		var manifest Manifest
		if err := json.Unmarshal(entries[manifestPath], &manifest); err != nil {
			t.Fatal(err)
		}
		manifest.Name = "Tampered"
		entries[manifestPath], _ = json.Marshal(manifest)
		pkg := readTestPackage(t, zipEntries(t, entries))
		repo := newMemoryRepository()
		_ = repo.RegisterKey(ctx, VerificationKey{ID: "owned-2026", PublicKey: publicKey, Purpose: KeyPurposeOwned, AllowedPluginIDs: []PluginID{manifest.ID}, Enabled: true})
		_, _, err := NewRegistryService(repo, Policy{}).Register(ctx, pkg)
		if !IsPackageError(err, ErrInvalidSignature) {
			t.Fatalf("tampered manifest was not rejected: %v", err)
		}
	})

	t.Run("wrong recognized key", func(t *testing.T) {
		wrongPublicKey, _, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{22}, 64)))
		repo := newMemoryRepository()
		_ = repo.RegisterKey(ctx, VerificationKey{ID: "owned-2026", PublicKey: wrongPublicKey, Purpose: KeyPurposeOwned, AllowedPluginIDs: []PluginID{"com.example.academy.timeline"}, Enabled: true})
		_, _, err := NewRegistryService(repo, Policy{}).Register(ctx, readTestPackage(t, archive))
		if !IsPackageError(err, ErrInvalidSignature) {
			t.Fatalf("wrong recognized key was not rejected: %v", err)
		}
	})
}

func TestRegistryTrustReplayMutationAndRevocation(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	svc := NewRegistryService(repo, Policy{AllowUnknown: true, RequireManualApprovalForUnknown: true})
	pub, private, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{9}, 64)))
	key := VerificationKey{ID: "owned-2026", PublicKey: pub, Purpose: KeyPurposeOwned, AllowedPluginIDs: []PluginID{"com.example.academy.timeline"}, Enabled: true}
	if err := repo.RegisterKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	b, _ := BuildPackageForTesting(testManifest("1.0.0"), map[string][]byte{"resources/timeline.js": []byte("v1")}, key.ID, private)
	p := readTestPackage(t, b)
	release, created, err := svc.Register(ctx, p)
	if err != nil || !created || release.CurrentTrust != TrustBTGOwned {
		t.Fatalf("register: %#v %v", release, err)
	}
	replay, created, err := svc.Register(ctx, p)
	if err != nil || created || replay.InstallationID != release.InstallationID {
		t.Fatalf("replay: %#v %v", replay, err)
	}
	enabled, err := svc.Enable(ctx, p.manifest.ID, p.manifest.Version)
	if err != nil || !enabled.Enabled {
		t.Fatalf("enable: %v", err)
	}
	mutated, _ := BuildPackageForTesting(testManifest("1.0.0"), map[string][]byte{"resources/timeline.js": []byte("different")}, key.ID, private)
	_, _, err = svc.Register(ctx, readTestPackage(t, mutated))
	if !errors.Is(err, ErrReleaseConflict) {
		t.Fatalf("mutation got %v", err)
	}
	if err := svc.SetKeyEnabled(ctx, key.ID, false); err != nil {
		t.Fatal(err)
	}
	current, err := svc.Get(ctx, p.manifest.ID, p.manifest.Version)
	if err != nil || current.CurrentTrust != TrustUnknown {
		t.Fatalf("trust not re-evaluated: %#v %v", current, err)
	}
}

func TestApprovedIsExactReleaseAndInvalidSignatureIsNotUnknown(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	svc := NewRegistryService(repo, Policy{})
	pub, private, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{4}, 64)))
	key := VerificationKey{ID: "approval-2026", PublicKey: pub, Purpose: KeyPurposeApproval, Enabled: true}
	_ = repo.RegisterKey(ctx, key)
	b, _ := BuildPackageForTesting(testManifest("1.0.0"), map[string][]byte{"resources/timeline.js": []byte("v1")}, key.ID, private)
	p := readTestPackage(t, b)
	unknown, _, err := svc.Register(ctx, p)
	if err != nil || unknown.CurrentTrust != TrustUnknown {
		t.Fatalf("before approval: %#v %v", unknown, err)
	}
	approval, err := svc.Approve(ctx, unknown.Release, ApprovalBTGRelease, key.ID)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := svc.Get(ctx, p.manifest.ID, p.manifest.Version)
	if err != nil || approved.CurrentTrust != TrustBTGApproved {
		t.Fatalf("approved: %#v %v", approved, err)
	}
	enabled, err := svc.Enable(ctx, p.manifest.ID, p.manifest.Version)
	if err != nil || !enabled.Enabled || enabled.CurrentTrust != TrustBTGApproved {
		t.Fatalf("approved release did not enable: %#v %v", enabled, err)
	}
	b2, _ := BuildPackageForTesting(testManifest("1.1.0"), map[string][]byte{"resources/timeline.js": []byte("v2")}, key.ID, private)
	newVersion, _, err := svc.Register(ctx, readTestPackage(t, b2))
	if err != nil || newVersion.CurrentTrust != TrustUnknown {
		t.Fatalf("approval floated: %#v %v", newVersion, err)
	}
	if err := svc.RevokeApproval(ctx, approval.ID); err != nil {
		t.Fatal(err)
	}
	revoked, err := svc.Get(ctx, p.manifest.ID, p.manifest.Version)
	if err != nil || revoked.CurrentTrust != TrustUnknown {
		t.Fatalf("revocation ignored: %#v %v", revoked, err)
	}
	tampered := *p
	tampered.signature = &Signature{Algorithm: "Ed25519", KeyID: key.ID, Value: p.signature.Value[:len(p.signature.Value)-2] + "aa"}
	_, _, err = svc.Register(ctx, &tampered)
	if !IsPackageError(err, ErrInvalidSignature) {
		t.Fatalf("invalid signature degraded to unknown: %v", err)
	}
}

func TestUnknownEnablementRequiresExplicitLocalApproval(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	service := NewRegistryService(repo, Policy{AllowUnknown: true, RequireManualApprovalForUnknown: true})
	b, _ := BuildPackageForTesting(testManifest("2.0.0"), map[string][]byte{"resources/timeline.js": []byte("unsigned")}, "", nil)
	pkg := readTestPackage(t, b)
	release, _, err := service.Register(ctx, pkg)
	if err != nil || release.CurrentTrust != TrustUnknown {
		t.Fatalf("register: %#v %v", release, err)
	}
	if _, err := service.Enable(ctx, release.Release.PluginID, release.Release.Version); !errors.Is(err, ErrEnableDenied) {
		t.Fatalf("enabled without approval: %v", err)
	}
	if _, err := service.Approve(ctx, release.Release, ApprovalLocalUnknown, ""); err != nil {
		t.Fatal(err)
	}
	enabled, err := service.Enable(ctx, release.Release.PluginID, release.Release.Version)
	if err != nil || !enabled.Enabled || enabled.CurrentTrust != TrustUnknown {
		t.Fatalf("local approval changed trust or failed: %#v %v", enabled, err)
	}
}

func TestManagementReleasesKeepValidationTrustApprovalAndEnablementSeparate(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	service := NewRegistryService(repo, Policy{})
	ownedPublic, ownedPrivate, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{31}, 64)))
	approvalPublic, approvalPrivate, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{32}, 64)))
	if err := repo.RegisterKey(ctx, VerificationKey{ID: "management-owned-2026", PublicKey: ownedPublic, Purpose: KeyPurposeOwned, AllowedPluginIDs: []PluginID{"com.example.academy.alpha"}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := repo.RegisterKey(ctx, VerificationKey{ID: "management-approval-2026", PublicKey: approvalPublic, Purpose: KeyPurposeApproval, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	register := func(id PluginID, version, keyID string, key ed25519.PrivateKey) InstalledRelease {
		manifest := testManifest(version)
		manifest.ID = id
		archive, err := BuildPackageForTesting(manifest, map[string][]byte{"resources/timeline.js": []byte(version)}, keyID, key)
		if err != nil {
			t.Fatal(err)
		}
		release, _, err := service.Register(ctx, readTestPackage(t, archive))
		if err != nil {
			t.Fatal(err)
		}
		return release
	}
	owned := register("com.example.academy.alpha", "1.0.0", "management-owned-2026", ownedPrivate)
	approved := register("com.example.academy.alpha", "2.0.0", "management-approval-2026", approvalPrivate)
	unknown := register("com.example.academy.beta", "1.0.0", "", nil)
	approval, err := service.Approve(ctx, approved.Release, ApprovalBTGRelease, "management-approval-2026")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Enable(ctx, owned.Release.PluginID, owned.Release.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Enable(ctx, approved.Release.PluginID, approved.Release.Version); err != nil {
		t.Fatal(err)
	}

	values, err := service.ManagementReleases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 3 || values[0].Release.Release.Version != "2.0.0" || values[1].Release.Release.Version != "1.0.0" || values[2].Release.Release.PluginID != unknown.Release.PluginID {
		t.Fatalf("management ordering=%#v", values)
	}
	if values[0].Release.CurrentTrust != TrustBTGApproved || !values[0].Release.Enabled || len(values[0].Approvals) != 1 || !values[0].Approvals[0].Active() {
		t.Fatalf("approved projection=%#v", values[0])
	}
	if values[1].Release.CurrentTrust != TrustBTGOwned || !values[1].Release.Enabled || len(values[1].Approvals) != 0 {
		t.Fatalf("owned projection=%#v", values[1])
	}
	if values[2].Release.CurrentTrust != TrustUnknown || values[2].Release.Enabled || values[2].ValidationStatus != ValidationStatusValidatedAtRegistration {
		t.Fatalf("unknown projection=%#v", values[2])
	}
	if err := service.RevokeApproval(ctx, approval.ID); err != nil {
		t.Fatal(err)
	}
	values, err = service.ManagementReleases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if values[0].Release.CurrentTrust != TrustUnknown || len(values[0].Approvals) != 1 || values[0].Approvals[0].Active() {
		t.Fatalf("revoked approval projection=%#v", values[0])
	}

	stored := repo.releases[releaseKey(owned.Release.PluginID, owned.Release.Version)]
	stored.Signature = &SignatureEvidence{KeyID: stored.Signature.KeyID, Value: "invalid", SigningPayload: stored.Signature.SigningPayload}
	repo.releases[releaseKey(owned.Release.PluginID, owned.Release.Version)] = stored
	values, err = service.ManagementReleases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if values[1].ValidationStatus != ValidationStatusSignatureInvalid || values[1].Release.CurrentTrust != "" {
		t.Fatalf("invalid signature was treated as trust=%#v", values[1])
	}
}

func TestExactReleaseApprovalLifecycleIsIdempotentAndVersionScoped(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryRepository()
	registry := NewRegistryService(repository, Policy{})
	publicKey, privateKey, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{33}, 64)))
	key, err := registry.AddKey(ctx, VerificationKey{ID: "lifecycle-approval-2026", PublicKey: publicKey, Purpose: KeyPurposeApproval, Enabled: true})
	if err != nil || key.ID != "lifecycle-approval-2026" {
		t.Fatalf("add key=%#v err=%v", key, err)
	}
	register := func(version string) InstalledRelease {
		archive, err := BuildPackageForTesting(testManifest(version), map[string][]byte{"resources/timeline.js": []byte(version)}, key.ID, privateKey)
		if err != nil {
			t.Fatal(err)
		}
		release, _, err := registry.Register(ctx, readTestPackage(t, archive))
		if err != nil {
			t.Fatal(err)
		}
		return release
	}
	v1 := register("6.0.0")
	v2 := register("6.1.0")
	first, err := registry.ApproveRelease(ctx, v1.Release.PluginID, v1.Release.Version)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := registry.ApproveRelease(ctx, v1.Release.PluginID, v1.Release.Version)
	if err != nil || replay.ID != first.ID {
		t.Fatalf("approval replay=%#v err=%v", replay, err)
	}
	if current, err := registry.Get(ctx, v1.Release.PluginID, v1.Release.Version); err != nil || current.CurrentTrust != TrustBTGApproved {
		t.Fatalf("approved release=%#v err=%v", current, err)
	}
	if current, err := registry.Get(ctx, v2.Release.PluginID, v2.Release.Version); err != nil || current.CurrentTrust != TrustUnknown {
		t.Fatalf("approval floated to new version=%#v err=%v", current, err)
	}
	if err := registry.RevokeReleaseApproval(ctx, v1.Release.PluginID, v1.Release.Version); err != nil {
		t.Fatal(err)
	}
	if err := registry.RevokeReleaseApproval(ctx, v1.Release.PluginID, v1.Release.Version); err != nil {
		t.Fatalf("revocation replay=%v", err)
	}
	if current, err := registry.Get(ctx, v1.Release.PluginID, v1.Release.Version); err != nil || current.CurrentTrust != TrustUnknown {
		t.Fatalf("revoked release=%#v err=%v", current, err)
	}
	views, err := registry.ManagementReleases(ctx)
	if err != nil || len(views) != 2 || len(views[1].Approvals) != 1 || views[1].Approvals[0].Active() {
		t.Fatalf("revoked history=%#v err=%v", views, err)
	}
	keys, err := registry.Keys(ctx)
	if err != nil || len(keys) != 1 || keys[0].ID != key.ID {
		t.Fatalf("keys=%#v err=%v", keys, err)
	}
}

func TestOwnedKeyAllowListDoesNotTrustAnotherPlugin(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryRepository()
	registry := NewRegistryService(repository, Policy{})
	publicKey, privateKey, _ := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{34}, 64)))
	if _, err := registry.AddKey(ctx, VerificationKey{ID: "owned-allow-list-2026", PublicKey: publicKey, Purpose: KeyPurposeOwned, AllowedPluginIDs: []PluginID{"com.example.academy.allowed"}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	manifest := testManifest("7.0.0")
	manifest.ID = "com.example.academy.not-allowed"
	archive, err := BuildPackageForTesting(manifest, map[string][]byte{"resources/timeline.js": []byte("not owned")}, "owned-allow-list-2026", privateKey)
	if err != nil {
		t.Fatal(err)
	}
	release, _, err := registry.Register(ctx, readTestPackage(t, archive))
	if err != nil || release.CurrentTrust != TrustUnknown {
		t.Fatalf("allow-list bypass release=%#v err=%v", release, err)
	}
	if _, err := registry.Enable(ctx, release.Release.PluginID, release.Release.Version); !errors.Is(err, ErrEnableDenied) {
		t.Fatalf("allow-list bypass enabled release: %v", err)
	}
}

func mustSignature(t *testing.T, v string) []byte {
	t.Helper()
	b, err := decodeSignature(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func zipEntries(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for n, b := range entries {
		w, err := z.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func unzipEntries(t *testing.T, b []byte) map[string][]byte {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range z.File {
		r, _ := f.Open()
		out[f.Name], _ = io.ReadAll(r)
		_ = r.Close()
	}
	return out
}

type memoryRepository struct {
	keys      map[string]VerificationKey
	releases  map[string]InstalledRelease
	approvals map[string]Approval
	resources map[string]InstalledResource
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{map[string]VerificationKey{}, map[string]InstalledRelease{}, map[string]Approval{}, map[string]InstalledResource{}}
}
func (r *memoryRepository) RegisterKey(_ context.Context, k VerificationKey) error {
	if old, ok := r.keys[k.ID]; ok && !bytes.Equal(old.PublicKey, k.PublicKey) {
		return ErrKeyIDConflict
	}
	r.keys[k.ID] = k
	return nil
}
func (r *memoryRepository) GetKey(_ context.Context, id string) (VerificationKey, error) {
	v, ok := r.keys[id]
	if !ok {
		return VerificationKey{}, ErrNotFound
	}
	return v, nil
}
func (r *memoryRepository) ListKeys(_ context.Context) ([]VerificationKey, error) {
	values := make([]VerificationKey, 0, len(r.keys))
	for _, key := range r.keys {
		values = append(values, key)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}
func (r *memoryRepository) SetKeyEnabled(_ context.Context, id string, e bool) error {
	v, ok := r.keys[id]
	if !ok {
		return ErrNotFound
	}
	v.Enabled = e
	r.keys[id] = v
	return nil
}
func releaseKey(id PluginID, v string) string { return string(id) + "@" + v }
func (r *memoryRepository) RegisterRelease(_ context.Context, v InstalledRelease, resources []InstalledResource) (InstalledRelease, bool, error) {
	k := releaseKey(v.Release.PluginID, v.Release.Version)
	if old, ok := r.releases[k]; ok {
		if old.Release.ArtifactDigest != v.Release.ArtifactDigest {
			return InstalledRelease{}, false, ErrReleaseConflict
		}
		for _, resource := range resources {
			resource.InstallationID = old.InstallationID
			resource.Content = append([]byte(nil), resource.Content...)
			r.resources[k+":"+resource.Path] = resource
		}
		return old, false, nil
	}
	r.releases[k] = v
	for _, resource := range resources {
		resource.Content = append([]byte(nil), resource.Content...)
		r.resources[k+":"+resource.Path] = resource
	}
	return v, true, nil
}
func (r *memoryRepository) GetRelease(_ context.Context, id PluginID, v string) (InstalledRelease, error) {
	x, ok := r.releases[releaseKey(id, v)]
	if !ok {
		return InstalledRelease{}, ErrNotFound
	}
	return x, nil
}
func (r *memoryRepository) ListReleases(_ context.Context) ([]InstalledRelease, error) {
	result := make([]InstalledRelease, 0, len(r.releases))
	for _, release := range r.releases {
		result = append(result, release)
	}
	return result, nil
}
func (r *memoryRepository) GetResource(_ context.Context, id ReleaseIdentity, path string) (InstalledResource, error) {
	v, ok := r.resources[releaseKey(id.PluginID, id.Version)+":"+path]
	if !ok || v.SHA256 != idResourceDigest(r, id, path) {
		return InstalledResource{}, ErrNotFound
	}
	v.Content = append([]byte(nil), v.Content...)
	return v, nil
}

func idResourceDigest(r *memoryRepository, id ReleaseIdentity, path string) string {
	v, ok := r.releases[releaseKey(id.PluginID, id.Version)]
	if !ok || v.Release.ArtifactDigest != id.ArtifactDigest {
		return ""
	}
	for _, resource := range v.Manifest.Resources {
		if resource.Path == path {
			return resource.SHA256
		}
	}
	return ""
}
func (r *memoryRepository) SetReleaseEnabled(_ context.Context, id string, e bool) (InstalledRelease, error) {
	for k, v := range r.releases {
		if v.InstallationID == id {
			v.Enabled = e
			if e {
				v.State = StateInstalled
			} else {
				v.State = StateDisabled
			}
			r.releases[k] = v
			return v, nil
		}
	}
	return InstalledRelease{}, ErrNotFound
}
func (r *memoryRepository) CreateApproval(_ context.Context, a Approval) error {
	r.approvals[a.ID] = a
	return nil
}
func (r *memoryRepository) RevokeApproval(_ context.Context, id string, at time.Time) error {
	a, ok := r.approvals[id]
	if !ok {
		return ErrNotFound
	}
	a.RevokedAt = &at
	r.approvals[id] = a
	return nil
}
func (r *memoryRepository) FindActiveApproval(_ context.Context, id ReleaseIdentity, k ApprovalKind, authority string) (Approval, error) {
	for _, a := range r.approvals {
		if a.Release == id && a.Kind == k && a.AuthorityKeyID == authority && a.Active() {
			return a, nil
		}
	}
	return Approval{}, ErrNotFound
}
func (r *memoryRepository) ListApprovals(_ context.Context, id ReleaseIdentity) ([]Approval, error) {
	values := make([]Approval, 0)
	for _, approval := range r.approvals {
		if approval.Release == id {
			values = append(values, approval)
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].ApprovedAt.Equal(values[j].ApprovedAt) {
			return values[i].ID < values[j].ID
		}
		return values[i].ApprovedAt.Before(values[j].ApprovedAt)
	})
	return values, nil
}
