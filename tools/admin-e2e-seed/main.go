package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	infrastructurepostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/portability"
	"github.com/jackc/pgx/v5"
)

const (
	fixtureEmail       = "admin-e2e@example.com"
	fixturePassword    = "admin integration password"
	fixturePluginID    = "com.example.academy.admin-e2e"
	fixturePluginVer   = "1.0.0"
	fixtureKeyID       = "admin-e2e-approval-key"
	fixtureCourseTitle = "Administration import integration"
	originCourseID     = "91000000-0000-4000-8000-000000000001"
	originVersionID    = "92000000-0000-4000-8000-000000000001"
)

type output struct {
	Email             string `json:"email"`
	Password          string `json:"password"`
	PluginPath        string `json:"pluginPath"`
	CoursePackagePath string `json:"coursePackagePath"`
	PluginID          string `json:"pluginId"`
	PluginVersion     string `json:"pluginVersion"`
	CourseTitle       string `json:"courseTitle"`
	PluginInstalled   bool   `json:"pluginInstalled"`
	PluginApproved    bool   `json:"pluginApproved"`
	PluginEnabled     bool   `json:"pluginEnabled"`
	ImportExists      bool   `json:"importExists"`
	ImportedCourseID  string `json:"importedCourseId,omitempty"`
}

func main() {
	if os.Getenv("BTG_ADMIN_E2E_ALLOW_SEED") != "1" {
		fatal(errors.New("refusing to manage the admin fixture without BTG_ADMIN_E2E_ALLOW_SEED=1"))
	}
	databaseURL := os.Getenv("BTG_LMS_DATABASE_URL")
	if databaseURL == "" {
		fatal(errors.New("BTG_LMS_DATABASE_URL is required"))
	}
	command := "seed"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	ctx := context.Background()
	if err := infrastructurepostgres.Migrate(ctx, databaseURL); err != nil {
		fatal(err)
	}
	pool, err := infrastructurepostgres.OpenPool(ctx, databaseURL)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()
	repository := identitypostgres.New(pool)
	userID, err := ensureAdmin(ctx, repository)
	if err != nil {
		fatal(err)
	}

	result := output{Email: fixtureEmail, Password: fixturePassword, PluginID: fixturePluginID, PluginVersion: fixturePluginVer, CourseTitle: fixtureCourseTitle}
	switch command {
	case "seed":
		if _, err = pool.Exec(ctx, `DELETE FROM identity.sessions WHERE user_id = $1`, userID); err != nil {
			fatal(err)
		}
		if _, err = pool.Exec(ctx, `DELETE FROM plugins.dashboard_widget_placement WHERE plugin_id = $1`, fixturePluginID); err != nil {
			fatal(err)
		}
		if _, err = pool.Exec(ctx, `DELETE FROM plugins.release_approval WHERE plugin_id = $1`, fixturePluginID); err != nil {
			fatal(err)
		}
		if _, err = pool.Exec(ctx, `DELETE FROM plugins.installed_release WHERE plugin_id = $1`, fixturePluginID); err != nil {
			fatal(err)
		}
		privateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{42}, ed25519.SeedSize))
		if _, err = pool.Exec(ctx, `INSERT INTO plugins.verification_key (key_id, public_key, purpose, allowed_plugin_ids, enabled)
			VALUES ($1, $2, 'BTG_APPROVAL_SIGNING', '{}', true)
			ON CONFLICT (key_id) DO UPDATE SET public_key=EXCLUDED.public_key, purpose=EXCLUDED.purpose, allowed_plugin_ids='{}', enabled=true`, fixtureKeyID, privateKey.Public().(ed25519.PublicKey)); err != nil {
			fatal(err)
		}
		artifactDir := filepath.Join("test-results", "admin-e2e")
		if err = os.MkdirAll(artifactDir, 0o755); err != nil {
			fatal(err)
		}
		result.PluginPath, err = writePluginPackage(artifactDir, privateKey)
		if err != nil {
			fatal(err)
		}
		result.CoursePackagePath, err = writeCoursePackage(artifactDir)
		if err != nil {
			fatal(err)
		}
	case "status":
		var approvalState string
		err = pool.QueryRow(ctx, `SELECT r.enabled, COALESCE(CASE WHEN a.revoked_at IS NULL THEN 'ACTIVE' ELSE 'REVOKED' END, 'NONE')
			FROM plugins.installed_release r LEFT JOIN LATERAL (
				SELECT revoked_at FROM plugins.release_approval a WHERE a.plugin_id=r.plugin_id AND a.version=r.version ORDER BY approved_at DESC LIMIT 1
			) a ON true WHERE r.plugin_id=$1 AND r.version=$2`, fixturePluginID, fixturePluginVer).Scan(&result.PluginEnabled, &approvalState)
		if err == nil {
			result.PluginInstalled = true
			result.PluginApproved = approvalState == "ACTIVE"
		} else if !errors.Is(err, pgx.ErrNoRows) {
			fatal(err)
		}
		err = pool.QueryRow(ctx, `SELECT target_course_id FROM portability.import_record WHERE portable_source_course_id=$1 AND portable_source_course_version_id=$2`, originCourseID, originVersionID).Scan(&result.ImportedCourseID)
		if err == nil {
			result.ImportExists = true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("unsupported command %q", command))
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fatal(err)
	}
}

func ensureAdmin(ctx context.Context, repository *identitypostgres.Repository) (identity.UserID, error) {
	email, err := repository.GetUserEmailByNormalized(ctx, fixtureEmail)
	if errors.Is(err, identity.ErrNotFound) {
		user, createErr := repository.CreateUser(ctx, identity.UserActive)
		if createErr != nil {
			return "", createErr
		}
		email, err = repository.CreateUserEmail(ctx, user.ID, fixtureEmail, true)
		if err != nil {
			return "", err
		}
		if _, err = repository.MarkUserEmailVerified(ctx, email.ID, time.Now().UTC()); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	hash, err := identity.DefaultPasswordHasher().HashPassword([]byte(fixturePassword))
	if err != nil {
		return "", err
	}
	if _, err = repository.ReplaceLocalPasswordHash(ctx, email.UserID, hash); errors.Is(err, identity.ErrNotFound) {
		_, err = repository.CreateLocalPasswordCredential(ctx, email.UserID, hash)
	}
	if err != nil {
		return "", err
	}
	if _, err = repository.AssignGlobalRole(ctx, email.UserID, identity.RoleAdministrator, nil); err != nil && !errors.Is(err, identity.ErrConflict) {
		return "", err
	}
	return email.UserID, nil
}

func writePluginPackage(dir string, privateKey ed25519.PrivateKey) (string, error) {
	manifest := plugins.Manifest{Format: plugins.PackageFormat, FormatVersion: plugins.PackageFormatVersion, ID: fixturePluginID, Version: fixturePluginVer, Name: "Admin integration plugin", Description: "Deterministic real-stack plugin lifecycle fixture.", Publisher: plugins.Publisher{Name: "Academy integration"}, PluginTypes: []plugins.PluginType{plugins.TypeCourseWidget}, Entrypoints: []plugins.Entrypoint{{ID: "course-widget", Type: plugins.TypeCourseWidget, Name: "Course widget", Resource: "resources/course.js"}}, Permissions: []plugins.Permission{plugins.PermissionNone}}
	body, err := plugins.BuildPackageForTesting(manifest, map[string][]byte{"resources/course.js": []byte("export default {}")}, fixtureKeyID, privateKey)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "admin-integration-plugin.zip")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", err
	}
	return absolute(path)
}

func writeCoursePackage(dir string) (string, error) {
	course := portability.CoursePayload{Title: fixtureCourseTitle, Description: "Deterministic real-stack course import fixture.", LearningObjectives: []string{"Verify course import persistence"}, Language: "en", Version: "1.0.0", Changelog: "Integration fixture.", Modules: []portability.Module{{StableKey: "imported-module", Title: "Imported module", Description: "Imported module description.", Position: 0, Lessons: []portability.Lesson{{StableKey: "imported-lesson", Title: "Imported lesson", Description: "Imported lesson description.", LearningObjectives: []string{"Verify imported content"}, Position: 0, PrerequisiteStableKeys: []string{}, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{}}}}}}}
	assessments := portability.AssessmentsPayload{Assessments: []portability.Assessment{}}
	files := map[string][]byte{}
	var err error
	if files["course.json"], err = json.Marshal(course); err != nil {
		return "", err
	}
	if files["assessments.json"], err = json.Marshal(assessments); err != nil {
		return "", err
	}
	paths := []string{"assessments.json", "course.json"}
	entries := make([]portability.ContentEntry, 0, len(paths))
	for _, path := range paths {
		sum := sha256.Sum256(files[path])
		entries = append(entries, portability.ContentEntry{Path: path, SHA256: hex.EncodeToString(sum[:])})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	manifest := portability.Manifest{Format: portability.Format, FormatVersion: portability.FormatVersion, ExportedAt: time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), Course: portability.ManifestCourse{OriginCourseID: originCourseID, OriginCourseVersionID: originVersionID, Version: "1.0.0", Language: "en", License: portability.License{Kind: string(courses.ContentLicenseStandard), Identifier: "CC-BY-4.0", DisplayName: "Creative Commons Attribution 4.0", URL: "https://creativecommons.org/licenses/by/4.0/"}, Attribution: []portability.Attribution{{DisplayName: "Integration author", Role: string(courses.ContributorAuthor), Order: 0}}}, Contents: entries, Assets: []portability.AssetEntry{}, Translations: []portability.TranslationEntry{}, ChecksumsPath: "checksums.json"}
	manifestBody, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	checksums, err := json.Marshal(portability.Checksums{Entries: entries})
	if err != nil {
		return "", err
	}
	files["manifest.json"] = manifestBody
	files["checksums.json"] = checksums
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, path := range []string{"manifest.json", "course.json", "assessments.json", "checksums.json"} {
		entry, createErr := writer.Create(path)
		if createErr != nil {
			return "", createErr
		}
		if _, writeErr := entry.Write(files[path]); writeErr != nil {
			return "", writeErr
		}
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	if _, err := portability.NewReader(portability.DefaultLimits()).Read(context.Background(), bytes.NewReader(archive.Bytes())); err != nil {
		return "", fmt.Errorf("validate generated course package: %w", err)
	}
	path := filepath.Join(dir, "admin-integration-course.zip")
	if err := os.WriteFile(path, archive.Bytes(), 0o600); err != nil {
		return "", err
	}
	return absolute(path)
}

func absolute(path string) (string, error) { return filepath.Abs(path) }
func fatal(err error)                      { _, _ = fmt.Fprintln(os.Stderr, err); os.Exit(1) }
