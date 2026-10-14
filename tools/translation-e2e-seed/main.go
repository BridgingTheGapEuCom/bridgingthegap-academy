package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	infrastructurepostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	fixtureEmail           = "translation-e2e@example.com"
	fixturePassword        = "translation integration password"
	fixtureSlug            = "translation-workspace-integration"
	fixtureTitle           = "Immutable translation source"
	fixtureTranslatedTitle = "Título de integración persistido"
	fixtureTargetLanguage  = "es"
)

type output struct {
	Email                 string `json:"email"`
	Password              string `json:"password"`
	CourseID              string `json:"courseId"`
	CourseVersionID       string `json:"courseVersionId"`
	Version               string `json:"version"`
	SourceLanguage        string `json:"sourceLanguage"`
	TargetLanguage        string `json:"targetLanguage"`
	SourceTitle           string `json:"sourceTitle"`
	ExpectedTranslated    string `json:"expectedTranslatedTitle"`
	TranslationID         string `json:"translationId,omitempty"`
	TranslationRevision   int64  `json:"translationRevision,omitempty"`
	StoredTranslatedTitle string `json:"storedTranslatedTitle,omitempty"`
	SourceUnchanged       bool   `json:"sourceUnchanged"`
	SourceBindingValid    bool   `json:"sourceBindingValid"`
}

func main() {
	if os.Getenv("BTG_TRANSLATION_E2E_ALLOW_SEED") != "1" {
		fatal(errors.New("refusing to manage the translation fixture without BTG_TRANSLATION_E2E_ALLOW_SEED=1"))
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
	userID, err := ensureUser(ctx, identitypostgres.New(pool))
	if err != nil {
		fatal(err)
	}
	course, source, err := ensureSource(ctx, coursespostgres.New(pool), userID)
	if err != nil {
		fatal(err)
	}
	result := output{Email: fixtureEmail, Password: fixturePassword, CourseID: string(course.ID), CourseVersionID: string(source.ID), Version: source.CourseVersion.Version.String(), SourceLanguage: string(source.CourseVersion.SourceLanguage), TargetLanguage: fixtureTargetLanguage, SourceTitle: fixtureTitle, ExpectedTranslated: fixtureTranslatedTitle}

	switch command {
	case "seed":
		if err := reset(ctx, pool, userID, source.ID); err != nil {
			fatal(err)
		}
	case "status":
		if err := inspect(ctx, pool, &result); err != nil {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("unknown command %q", command))
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fatal(err)
	}
}

func ensureUser(ctx context.Context, repository *identitypostgres.Repository) (identity.UserID, error) {
	email, err := repository.GetUserEmailByNormalized(ctx, fixtureEmail)
	if err == nil {
		hash, hashErr := identity.DefaultPasswordHasher().HashPassword([]byte(fixturePassword))
		if hashErr != nil {
			return "", hashErr
		}
		_, replaceErr := repository.ReplaceLocalPasswordHash(ctx, email.UserID, hash)
		return email.UserID, replaceErr
	}
	if !errors.Is(err, identity.ErrNotFound) {
		return "", err
	}
	user, err := repository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		return "", err
	}
	email, err = repository.CreateUserEmail(ctx, user.ID, fixtureEmail, true)
	if err != nil {
		return "", err
	}
	if _, err = repository.MarkUserEmailVerified(ctx, email.ID, time.Now().UTC()); err != nil {
		return "", err
	}
	hash, err := identity.DefaultPasswordHasher().HashPassword([]byte(fixturePassword))
	if err != nil {
		return "", err
	}
	if _, err = repository.CreateLocalPasswordCredential(ctx, user.ID, hash); err != nil {
		return "", err
	}
	return user.ID, nil
}

func ensureSource(ctx context.Context, repository *coursespostgres.Repository, userID identity.UserID) (courses.Course, courses.ImmutableCourseVersion, error) {
	version, _ := courses.ParseVersion("1.0.0")
	course, err := repository.GetCourseBySlug(ctx, fixtureSlug)
	if errors.Is(err, courses.ErrNotFound) {
		course, err = repository.CreateCourse(ctx, fixtureSlug)
	}
	if err != nil {
		return courses.Course{}, courses.ImmutableCourseVersion{}, err
	}
	source, err := repository.GetImmutableCourseVersionByCourseAndVersion(ctx, course.ID, version)
	if err == nil {
		if source.CourseVersion.Title != fixtureTitle {
			return courses.Course{}, courses.ImmutableCourseVersion{}, errors.New("existing translation fixture has unexpected source content; use a clean test database")
		}
		return course, source, nil
	}
	if !errors.Is(err, courses.ErrNotFound) {
		return courses.Course{}, courses.ImmutableCourseVersion{}, err
	}

	now := time.Now().UTC().Truncate(time.Second)
	approved := now.Add(-time.Minute)
	reviewID, draftID, moduleID, lessonID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	provenance := courses.CourseVersionProvenance{ReviewID: reviewID, ReviewRevision: 1, DraftID: draftID, DraftRevision: 1, SnapshotSchemaVersion: 1, SubmittedByUserID: string(userID), SubmittedAt: now.Add(-2 * time.Minute), ApprovedByUserID: string(userID), ApprovedAt: &approved, PublishedByUserID: string(userID)}
	aggregate := courses.ImmutableCourseVersion{
		CourseVersion: courses.CourseVersionInput{CourseID: course.ID, Version: version, Status: courses.CourseVersionPublished, Title: fixtureTitle, Description: "An exact immutable source for real translation integration.", LearningObjectives: []string{"Preserve source and target language identity"}, SourceLanguage: "en", Changelog: "Translation fixture.", License: courses.ContentLicense{Kind: courses.ContentLicenseStandard, Identifier: "CC-BY-4.0", DisplayName: "Creative Commons Attribution 4.0", URL: "https://creativecommons.org/licenses/by/4.0/"}, Attribution: []courses.ContributorSnapshot{{UserID: string(userID), DisplayName: "Translation integration author", Role: courses.ContributorAuthor, Order: 0}}, PublishedAt: now},
		Provenance:    provenance, Publication: courses.PublicationProvenance{Origin: courses.PublicationOriginNative, Native: &provenance},
		Modules:            []courses.ImmutableCourseVersionModule{{SourceID: moduleID, StableKey: "translation-module", Title: "Source module", Description: "Source module description.", Position: 0, Lessons: []courses.ImmutableCourseVersionLesson{{SourceID: lessonID, StableKey: "translation-lesson", Title: "Source lesson", Description: "Source lesson description.", LearningObjectives: []string{"Translate source prose"}, Position: 0, PrerequisiteStableKeys: []string{}, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "source-text", Type: courses.BlockText, Payload: courses.TextBlockPayload{Content: courses.RichText{Nodes: []courses.RichTextNode{{Type: "paragraph", Content: []courses.RichTextInline{{Type: "text", Text: "Immutable source prose.", Marks: []courses.RichTextMark{}}}}}}}}}}}}}},
		AssessmentBindings: []courses.PublishedAssessmentBinding{}, AssetBindings: []courses.PublishedAssetBinding{},
	}
	source, err = courses.NewCourseVersionStore(repository).Store(ctx, aggregate)
	return course, source, err
}

func reset(ctx context.Context, pool *pgxpool.Pool, userID identity.UserID, sourceID courses.CourseVersionID) error {
	if _, err := pool.Exec(ctx, `DELETE FROM identity.sessions WHERE user_id=$1`, userID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `DELETE FROM preferences.user_preferences WHERE user_id=$1`, userID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `DELETE FROM translations.translation_publication WHERE source_course_version_id=$1 AND target_language=$2`, sourceID, fixtureTargetLanguage); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, `DELETE FROM translations.course_translation WHERE source_course_version_id=$1 AND target_language=$2`, sourceID, fixtureTargetLanguage)
	return err
}

func inspect(ctx context.Context, pool *pgxpool.Pool, result *output) error {
	var storedSourceID, storedCourseID, sourceLanguage, targetLanguage, sourceTitle string
	err := pool.QueryRow(ctx, `
		SELECT t.id,t.revision,t.source_course_id,t.source_course_version_id,t.source_language,t.target_language,t.translated_tree->>'Title',v.title
		FROM translations.course_translation t
		JOIN courses.course_version v ON v.id=t.source_course_version_id
		WHERE t.source_course_version_id=$1 AND t.target_language=$2`, result.CourseVersionID, fixtureTargetLanguage).
		Scan(&result.TranslationID, &result.TranslationRevision, &storedCourseID, &storedSourceID, &sourceLanguage, &targetLanguage, &result.StoredTranslatedTitle, &sourceTitle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("translation fixture was not persisted")
		}
		return err
	}
	result.SourceUnchanged = sourceTitle == fixtureTitle
	result.SourceBindingValid = storedCourseID == result.CourseID && storedSourceID == result.CourseVersionID && sourceLanguage == result.SourceLanguage && targetLanguage == result.TargetLanguage
	return nil
}

func fatal(err error) { _, _ = fmt.Fprintln(os.Stderr, err); os.Exit(1) }
