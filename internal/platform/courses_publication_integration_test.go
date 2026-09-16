//go:build integration

package platform

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testImmutableCourseVersionPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	repository := coursespostgres.New(pool)
	store := courses.NewCourseVersionStore(repository)
	course, err := repository.CreateCourse(ctx, "immutable-publication-persistence")
	if err != nil {
		t.Fatal(err)
	}

	input := immutableCourseVersionFixture(t, course.ID, "1.2.3", "71000000-0000-4000-8000-000000000001")
	stored, err := store.Store(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	assertImmutableCourseVersionRoundTrip(t, input, stored)
	reloaded, err := repository.GetImmutableCourseVersion(ctx, stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertImmutableCourseVersionRoundTrip(t, input, reloaded)

	versionCount, moduleCount, lessonCount := publicationCounts(t, ctx, pool, course.ID)
	if versionCount != 1 || moduleCount != 2 || lessonCount != 3 {
		t.Fatalf("unexpected publication counts: versions=%d modules=%d lessons=%d", versionCount, moduleCount, lessonCount)
	}
	if _, err := store.Store(ctx, input); !errors.Is(err, courses.ErrCourseVersionAlreadyExists) {
		t.Fatalf("duplicate version error = %v", err)
	}
	if counts := publicationCountsTuple(t, ctx, pool, course.ID); counts != [3]int{1, 2, 3} {
		t.Fatalf("duplicate attempt created children: %v", counts)
	}

	replay := immutableCourseVersionFixture(t, course.ID, "1.2.4", input.Provenance.ReviewID)
	if _, err := store.Store(ctx, replay); !errors.Is(err, courses.ErrCourseVersionAlreadyExists) {
		t.Fatalf("Review replay error = %v", err)
	}
	if counts := publicationCountsTuple(t, ctx, pool, course.ID); counts != [3]int{1, 2, 3} {
		t.Fatalf("Review replay left partial data: %v", counts)
	}

	rollback := immutableCourseVersionFixture(t, course.ID, "2.0.0", "71000000-0000-4000-8000-000000000002")
	// The database source-provenance uniqueness constraint fails on a later
	// Lesson insert, after the parent, provenance, Modules, and first Lesson.
	rollback.Modules[1].Lessons[0].SourceID = rollback.Modules[0].Lessons[0].SourceID
	if _, err := store.Store(ctx, rollback); !errors.Is(err, courses.ErrInvalidImmutableCourseVersion) {
		t.Fatalf("late child failure = %v", err)
	}
	if counts := publicationCountsTuple(t, ctx, pool, course.ID); counts != [3]int{1, 2, 3} {
		t.Fatalf("late child failure did not roll back: %v", counts)
	}

	concurrent := immutableCourseVersionFixture(t, course.ID, "3.0.0", "71000000-0000-4000-8000-000000000003")
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, writeErr := store.Store(ctx, concurrent)
			results <- writeErr
		}()
	}
	wait.Wait()
	close(results)
	succeeded, duplicated := 0, 0
	for result := range results {
		switch {
		case result == nil:
			succeeded++
		case errors.Is(result, courses.ErrCourseVersionAlreadyExists):
			duplicated++
		default:
			t.Fatalf("concurrent publication error = %v", result)
		}
	}
	if succeeded != 1 || duplicated != 1 {
		t.Fatalf("concurrent results: success=%d duplicate=%d", succeeded, duplicated)
	}
	if counts := publicationCountsTuple(t, ctx, pool, course.ID); counts != [3]int{2, 4, 6} {
		t.Fatalf("concurrent publication was incomplete or duplicated: %v", counts)
	}
	concurrentStored, err := repository.GetCourseVersionByCourseAndVersion(ctx, course.ID, concurrent.CourseVersion.Version)
	if err != nil {
		t.Fatal(err)
	}
	concurrentReloaded, err := repository.GetImmutableCourseVersion(ctx, concurrentStored.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertImmutableCourseVersionRoundTrip(t, concurrent, concurrentReloaded)
}

func immutableCourseVersionFixture(t *testing.T, courseID courses.CourseID, versionText, reviewID string) courses.ImmutableCourseVersion {
	t.Helper()
	version, err := courses.ParseVersion(versionText)
	if err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	approvedAt := time.Date(2026, time.September, 14, 11, 0, 0, 0, time.UTC)
	duration := 35
	return courses.ImmutableCourseVersion{
		CourseVersion: courses.CourseVersionInput{
			CourseID:           courseID,
			Version:            version,
			Status:             courses.CourseVersionPublished,
			Title:              "Immutable publication",
			Description:        "A complete immutable Courses aggregate.",
			LearningObjectives: []string{"Explain atomic publication", "Preserve canonical content"},
			SourceLanguage:     "en-GB",
			Changelog:          "Initial immutable publication.",
			License: courses.ContentLicense{
				Kind: courses.ContentLicenseStandard, Identifier: "CC-BY-4.0",
				DisplayName: "Creative Commons Attribution 4.0", URL: "https://creativecommons.org/licenses/by/4.0/",
			},
			Attribution: []courses.ContributorSnapshot{
				{UserID: "72000000-0000-4000-8000-000000000001", DisplayName: "Ada Author", Role: courses.ContributorAuthor, Order: 0},
				{UserID: "72000000-0000-4000-8000-000000000002", DisplayName: "Mina Maintainer", Role: courses.ContributorMaintainer, Order: 1},
			},
			PublishedAt: publishedAt,
		},
		Provenance: courses.CourseVersionProvenance{
			ReviewID: reviewID, ReviewRevision: 2,
			DraftID: "73000000-0000-4000-8000-000000000001", DraftRevision: 11, SnapshotSchemaVersion: 1,
			SubmittedByUserID: "72000000-0000-4000-8000-000000000001", SubmittedAt: time.Date(2026, time.September, 13, 10, 0, 0, 0, time.UTC),
			ApprovedByUserID: "72000000-0000-4000-8000-000000000002", ApprovedAt: &approvedAt,
			PublishedByUserID: "72000000-0000-4000-8000-000000000003",
		},
		Modules: []courses.ImmutableCourseVersionModule{
			{
				SourceID: "74000000-0000-4000-8000-000000000001", StableKey: "foundations", Title: "Foundations", Description: "Core concepts.", Position: 0,
				Lessons: []courses.ImmutableCourseVersionLesson{
					{
						SourceID: "75000000-0000-4000-8000-000000000001", StableKey: "introduction", Title: "Introduction", Description: "Introduce publication.", LearningObjectives: []string{"Describe publication"}, Position: 0, PrerequisiteStableKeys: []string{},
						Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{
							{Key: "architecture", Type: courses.BlockImage, Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: "architecture-diagram"}, AltText: "Publication architecture diagram", Caption: "Atomic publication flow"}},
							{Key: "check", Type: courses.BlockKnowledgeCheck, Payload: courses.KnowledgeCheckBlockPayload{AssessmentKey: "publication-check"}},
						}},
					},
					{
						SourceID: "75000000-0000-4000-8000-000000000002", StableKey: "transactions", Title: "Transactions", Description: "Apply transactional storage.", LearningObjectives: []string{"Explain rollback"}, EstimatedDurationMinutes: &duration, Position: 1, PrerequisiteStableKeys: []string{"introduction"},
						Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "divider", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}}}},
					},
				},
			},
			{
				SourceID: "74000000-0000-4000-8000-000000000002", StableKey: "practice", Title: "Practice", Description: "Practice publication.", Position: 1,
				Lessons: []courses.ImmutableCourseVersionLesson{{
					SourceID: "75000000-0000-4000-8000-000000000003", StableKey: "exercise", Title: "Exercise", Description: "Persist a complete version.", LearningObjectives: []string{"Persist an aggregate"}, Position: 0, PrerequisiteStableKeys: []string{"transactions", "introduction"},
					Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "exercise-divider", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}}}},
				}},
			},
		},
	}
}

func assertImmutableCourseVersionRoundTrip(t *testing.T, expected, actual courses.ImmutableCourseVersion) {
	t.Helper()
	if _, err := uuid.Parse(string(actual.ID)); err != nil || actual.ID == courses.CourseVersionID(expected.Provenance.ReviewID) {
		t.Fatalf("invalid generated CourseVersion ID: %q", actual.ID)
	}
	if !reflect.DeepEqual(actual.CourseVersion, expected.CourseVersion) || !reflect.DeepEqual(actual.Provenance, expected.Provenance) || len(actual.Modules) != len(expected.Modules) {
		t.Fatalf("CourseVersion metadata/provenance mismatch:\nexpected=%#v\nactual=%#v", expected, actual)
	}
	for moduleIndex := range expected.Modules {
		expectedModule, actualModule := expected.Modules[moduleIndex], actual.Modules[moduleIndex]
		expectedModule.Lessons = append([]courses.ImmutableCourseVersionLesson{}, expectedModule.Lessons...)
		if _, err := uuid.Parse(string(actualModule.ID)); err != nil || string(actualModule.ID) == expectedModule.SourceID {
			t.Fatalf("invalid generated Module ID: %#v", actualModule)
		}
		expectedModule.ID = actualModule.ID
		for lessonIndex := range actualModule.Lessons {
			actualLesson := actualModule.Lessons[lessonIndex]
			if _, err := uuid.Parse(string(actualLesson.ID)); err != nil || string(actualLesson.ID) == actualLesson.SourceID {
				t.Fatalf("invalid generated Lesson ID: %#v", actualLesson)
			}
			expectedModule.Lessons[lessonIndex].ID = actualLesson.ID
		}
		if !reflect.DeepEqual(actualModule, expectedModule) {
			t.Fatalf("Module round trip mismatch:\nexpected=%#v\nactual=%#v", expectedModule, actualModule)
		}
	}
}

func publicationCounts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, courseID courses.CourseID) (int, int, int) {
	t.Helper()
	var versions, modules, lessons int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM courses.course_version WHERE course_id=$1", courseID).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM courses.module WHERE course_version_id IN (SELECT id FROM courses.course_version WHERE course_id=$1)", courseID).Scan(&modules); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM courses.lesson WHERE course_version_id IN (SELECT id FROM courses.course_version WHERE course_id=$1)", courseID).Scan(&lessons); err != nil {
		t.Fatal(err)
	}
	return versions, modules, lessons
}

func publicationCountsTuple(t *testing.T, ctx context.Context, pool *pgxpool.Pool, courseID courses.CourseID) [3]int {
	t.Helper()
	versions, modules, lessons := publicationCounts(t, ctx, pool, courseID)
	return [3]int{versions, modules, lessons}
}
