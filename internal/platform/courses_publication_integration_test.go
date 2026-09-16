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

func testImmutablePublishedCourseVersionReads(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	repository := coursespostgres.New(pool)
	store := courses.NewCourseVersionStore(repository)
	service := courses.NewPublishedReadService(repository)
	course, err := repository.CreateCourse(ctx, "immutable-published-read")
	if err != nil {
		t.Fatal(err)
	}

	older := immutableCourseVersionFixture(t, course.ID, "1.9.0", "81000000-0000-4000-8000-000000000001")
	older.Modules[0].Title = "Older foundations"
	if _, err := store.Store(ctx, older); err != nil {
		t.Fatal(err)
	}
	latest := immutableCourseVersionFixture(t, course.ID, "1.10.0", "81000000-0000-4000-8000-000000000002")
	latest.Modules[0].Title = "Latest foundations"
	latest.Modules[0].Lessons[0].Title = "Latest introduction"
	if _, err := store.Store(ctx, latest); err != nil {
		t.Fatal(err)
	}

	exact, err := service.Exact(ctx, course.ID, older.CourseVersion.Version)
	if err != nil {
		t.Fatal(err)
	}
	if exact.Version.String() != "1.9.0" || exact.Modules[0].Title != "Older foundations" || exact.Modules[0].Lessons[0].Title != "Introduction" {
		t.Fatalf("exact read mixed immutable versions: %#v", exact)
	}
	selected, err := service.Latest(ctx, course.ID)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Version.String() != "1.10.0" || selected.Modules[0].Title != "Latest foundations" || selected.Modules[0].Lessons[0].Title != "Latest introduction" {
		t.Fatalf("latest read did not use numeric SemVer and one aggregate: %#v", selected)
	}
	if len(selected.Modules) != 2 || selected.Modules[0].Position != 0 || selected.Modules[1].Position != 1 || len(selected.Modules[0].Lessons) != 2 || selected.Modules[0].Lessons[1].PrerequisiteStableKeys[0] != "introduction" {
		t.Fatalf("published structure ordering or prerequisites were not preserved: %#v", selected.Modules)
	}
	if len(selected.Modules[0].Lessons[0].Content.Blocks) != 2 || selected.Modules[0].Lessons[0].Content.Blocks[0].Type != courses.BlockImage || selected.Modules[0].Lessons[0].Content.Blocks[1].Type != courses.BlockKnowledgeCheck {
		t.Fatalf("canonical accessibility, asset, or assessment content was not preserved: %#v", selected.Modules[0].Lessons[0].Content)
	}
	missingVersion, err := courses.ParseVersion("1.11.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Exact(ctx, course.ID, missingVersion); !errors.Is(err, courses.ErrNotFound) {
		t.Fatalf("unknown exact version error = %v, want not found", err)
	}
	if _, err := service.Latest(ctx, courses.CourseID("82000000-0000-4000-8000-000000000001")); !errors.Is(err, courses.ErrNotFound) {
		t.Fatalf("no published version error = %v, want not found", err)
	}
}

func testPublishedCourseCatalog(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	repository := coursespostgres.New(pool)
	store := courses.NewCourseVersionStore(repository)
	service := courses.NewPublishedCatalogService(repository)
	baseline, err := service.List(ctx, courses.PublishedCatalogQuery{Limit: courses.MaxPublishedCatalogLimit})
	if err != nil {
		t.Fatal(err)
	}
	first, err := repository.CreateCourse(ctx, "published-catalog-first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.CreateCourse(ctx, "published-catalog-second")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateCourse(ctx, "published-catalog-unpublished"); err != nil {
		t.Fatal(err)
	}

	older := immutableCourseVersionFixture(t, first.ID, "1.9.0", "84000000-0000-4000-8000-000000000001")
	older.CourseVersion.Title = "Older Esperanto catalog entry"
	older.CourseVersion.SourceLanguage = "eo"
	if _, err := store.Store(ctx, older); err != nil {
		t.Fatal(err)
	}
	latest := immutableCourseVersionFixture(t, first.ID, "1.10.0", "84000000-0000-4000-8000-000000000002")
	latest.CourseVersion.Title = "Shared catalog title"
	latest.CourseVersion.SourceLanguage = "fr"
	if _, err := store.Store(ctx, latest); err != nil {
		t.Fatal(err)
	}
	other := immutableCourseVersionFixture(t, second.ID, "2.0.0", "84000000-0000-4000-8000-000000000003")
	other.CourseVersion.Title = "Shared catalog title"
	other.CourseVersion.SourceLanguage = "eo"
	if _, err := store.Store(ctx, other); err != nil {
		t.Fatal(err)
	}

	page, err := service.List(ctx, courses.PublishedCatalogQuery{Limit: courses.MaxPublishedCatalogLimit})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != baseline.Total+2 || len(page.Items) != baseline.Total+2 {
		t.Fatalf("catalog cardinality = %#v", page)
	}
	for index := 1; index < len(page.Items); index++ {
		previous, current := page.Items[index-1], page.Items[index]
		if previous.Title > current.Title || (previous.Title == current.Title && previous.CourseID > current.CourseID) {
			t.Fatalf("catalog ordering is not title then course ID: %#v", page.Items)
		}
	}
	var selected, selectedOther courses.PublishedCatalogItem
	for _, item := range page.Items {
		if item.CourseID == first.ID {
			selected = item
		}
		if item.CourseID == second.ID {
			selectedOther = item
		}
	}
	if selected.Version.String() != "1.10.0" || selected.SourceLanguage != "fr" {
		t.Fatalf("catalog did not select latest numeric SemVer: %#v", selected)
	}
	if selectedOther.Version.String() != "2.0.0" || selectedOther.SourceLanguage != "eo" {
		t.Fatalf("catalog omitted the other published course: %#v", selectedOther)
	}
	if len(selected.Contributors) != 2 || selected.Contributors[0].DisplayName != "Ada Author" {
		t.Fatalf("catalog did not preserve public attribution: %#v", selected.Contributors)
	}

	language, err := courses.NormalizeLanguageTag("eo")
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := service.List(ctx, courses.PublishedCatalogQuery{Language: &language})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].CourseID != second.ID {
		t.Fatalf("language filter used an older version instead of the selected latest version: %#v", filtered)
	}

	paged, err := service.List(ctx, courses.PublishedCatalogQuery{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if paged.Total != page.Total || len(paged.Items) != 1 || paged.Items[0].CourseID != page.Items[1].CourseID {
		t.Fatalf("catalog pagination was not deterministic: %#v", paged)
	}
	emptyLanguage, err := courses.NormalizeLanguageTag("zu")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := service.List(ctx, courses.PublishedCatalogQuery{Language: &emptyLanguage})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Total != 0 || len(empty.Items) != 0 || empty.Items == nil {
		t.Fatalf("empty catalog page = %#v", empty)
	}
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
