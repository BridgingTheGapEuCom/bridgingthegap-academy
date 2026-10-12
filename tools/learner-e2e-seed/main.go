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
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	fixtureEmail    = "learner-e2e@example.com"
	fixturePassword = "learner integration password"
	fixtureSlug     = "learner-reader-integration"
	fixtureTitle    = "Learner reader integration"
)

type output struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	CourseID      string `json:"courseId"`
	Version       string `json:"version"`
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	LessonKey     string `json:"lessonKey"`
	AssessmentKey string `json:"assessmentKey"`
}

func main() {
	if os.Getenv("BTG_LEARNER_E2E_ALLOW_SEED") != "1" {
		fatal(errors.New("refusing to seed without BTG_LEARNER_E2E_ALLOW_SEED=1"))
	}
	databaseURL := os.Getenv("BTG_LMS_DATABASE_URL")
	if databaseURL == "" {
		fatal(errors.New("BTG_LMS_DATABASE_URL is required"))
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

	userID, err := seedUser(ctx, identitypostgres.New(pool))
	if err != nil {
		fatal(err)
	}
	course, published, err := seedCourse(ctx, coursespostgres.New(pool), userID)
	if err != nil {
		fatal(err)
	}
	if err := resetLearnerState(ctx, pool, userID, published.ID); err != nil {
		fatal(err)
	}
	result := output{Email: fixtureEmail, Password: fixturePassword, CourseID: string(course.ID), Version: published.CourseVersion.Version.String(), Slug: fixtureSlug, Title: fixtureTitle, LessonKey: "introduction", AssessmentKey: published.AssessmentBindings[0].AssessmentKey}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fatal(err)
	}
}

func resetLearnerState(ctx context.Context, pool *pgxpool.Pool, userID identity.UserID, courseVersionID courses.CourseVersionID) error {
	statements := []string{
		`DELETE FROM open_badges.signed_credential WHERE certificate_id IN (SELECT id FROM credentials.certificate WHERE learner_user_id = $1 AND course_version_id = $2)`,
		`DELETE FROM open_badges.status_list_entry WHERE certificate_id IN (SELECT id FROM credentials.certificate WHERE learner_user_id = $1 AND course_version_id = $2)`,
		`DELETE FROM credentials.certificate WHERE learner_user_id = $1 AND course_version_id = $2`,
		`DELETE FROM assessments.assessment_attempt WHERE learner_user_id = $1 AND course_version_id = $2`,
		`DELETE FROM progress.lesson_completion WHERE learner_user_id = $1 AND course_version_id = $2`,
		`DELETE FROM progress.course_progress WHERE learner_user_id = $1 AND course_version_id = $2`,
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement, string(userID), string(courseVersionID)); err != nil {
			return err
		}
	}
	return nil
}

func seedUser(ctx context.Context, repository *identitypostgres.Repository) (identity.UserID, error) {
	email, err := repository.GetUserEmailByNormalized(ctx, fixtureEmail)
	if err == nil {
		hash, hashErr := identity.DefaultPasswordHasher().HashPassword([]byte(fixturePassword))
		if hashErr != nil {
			return "", hashErr
		}
		if _, replaceErr := repository.ReplaceLocalPasswordHash(ctx, email.UserID, hash); replaceErr != nil {
			return "", replaceErr
		}
		return email.UserID, nil
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

func seedCourse(ctx context.Context, repository *coursespostgres.Repository, userID identity.UserID) (courses.Course, courses.ImmutableCourseVersion, error) {
	version, _ := courses.ParseVersion("1.0.0")
	course, err := repository.GetCourseBySlug(ctx, fixtureSlug)
	if errors.Is(err, courses.ErrNotFound) {
		course, err = repository.CreateCourse(ctx, fixtureSlug)
	}
	if err != nil {
		return courses.Course{}, courses.ImmutableCourseVersion{}, err
	}
	published, err := repository.GetImmutableCourseVersionByCourseAndVersion(ctx, course.ID, version)
	if err == nil {
		if published.CourseVersion.Title != fixtureTitle {
			return courses.Course{}, courses.ImmutableCourseVersion{}, errors.New("existing learner E2E fixture has unexpected content; use a clean test database")
		}
		return course, published, nil
	}
	if !errors.Is(err, courses.ErrNotFound) {
		return courses.Course{}, courses.ImmutableCourseVersion{}, err
	}

	now := time.Now().UTC().Truncate(time.Second)
	approved := now.Add(-time.Minute)
	reviewID, draftID, moduleID, firstLessonID, secondLessonID, assessmentID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	provenance := courses.CourseVersionProvenance{ReviewID: reviewID, ReviewRevision: 1, DraftID: draftID, DraftRevision: 1, SnapshotSchemaVersion: 1, SubmittedByUserID: string(userID), SubmittedAt: now.Add(-2 * time.Minute), ApprovedByUserID: string(userID), ApprovedAt: &approved, PublishedByUserID: string(userID)}
	aggregate := courses.ImmutableCourseVersion{
		CourseVersion: courses.CourseVersionInput{CourseID: course.ID, Version: version, Status: courses.CourseVersionPublished, Title: fixtureTitle, Description: "A deterministic real-stack learner reader course.", LearningObjectives: []string{"Persist learner progress and assessment results"}, SourceLanguage: "en", Changelog: "Integration fixture.", License: courses.ContentLicense{Kind: courses.ContentLicenseStandard, Identifier: "CC-BY-4.0", DisplayName: "Creative Commons Attribution 4.0", URL: "https://creativecommons.org/licenses/by/4.0/"}, Attribution: []courses.ContributorSnapshot{{UserID: string(userID), DisplayName: "Learner integration", Role: courses.ContributorAuthor, Order: 0}}, PublishedAt: now},
		Provenance:    provenance, Publication: courses.PublicationProvenance{Origin: courses.PublicationOriginNative, Native: &provenance},
		Modules: []courses.ImmutableCourseVersionModule{{SourceID: moduleID, StableKey: "foundations", Title: "Foundations", Description: "Reader foundations.", Position: 0, Lessons: []courses.ImmutableCourseVersionLesson{
			{SourceID: firstLessonID, StableKey: "introduction", Title: "Introduction", Description: "Complete the knowledge check.", LearningObjectives: []string{"Verify durable learning state"}, Position: 0, PrerequisiteStableKeys: []string{}, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "intro-text", Type: courses.BlockText, Payload: courses.TextBlockPayload{Content: courses.RichText{Nodes: []courses.RichTextNode{{Type: "paragraph", Content: []courses.RichTextInline{{Type: "text", Text: "Real-stack learner content.", Marks: []courses.RichTextMark{}}}}}}}}, {Key: "knowledge-check", Type: courses.BlockKnowledgeCheck, Payload: courses.KnowledgeCheckBlockPayload{AssessmentKey: assessmentID}}}}},
			{SourceID: secondLessonID, StableKey: "next-steps", Title: "Next steps", Description: "Continue learning.", LearningObjectives: []string{"Continue the course"}, Position: 1, PrerequisiteStableKeys: []string{"introduction"}, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{}}},
		}}},
		AssessmentBindings: []courses.PublishedAssessmentBinding{{AssessmentKey: assessmentID, Questions: []courses.PublishedAssessmentQuestion{{StableKey: "publication-question", Type: courses.PublishedQuestionSingleChoice, Prompt: "What persists learner state?", Position: 0, Options: []courses.PublishedAssessmentOption{{StableKey: "backend", Text: "The Academy backend", Position: 0}, {StableKey: "memory", Text: "Browser memory only", Position: 1}}, CorrectOptionKeys: []string{"backend"}}}}},
		AssetBindings:      []courses.PublishedAssetBinding{},
	}
	published, err = courses.NewCourseVersionStore(repository).Store(ctx, aggregate)
	return course, published, err
}

func fatal(err error) { _, _ = fmt.Fprintln(os.Stderr, err); os.Exit(1) }
