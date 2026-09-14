//go:build integration

package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testCoursesPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	r := coursespostgres.New(pool)
	course, err := r.CreateCourse(ctx, "event-driven-architecture")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := r.GetCourseBySlug(ctx, "event-driven-architecture")
	if err != nil || loaded.ID != course.ID {
		t.Fatalf("course lookup failed: %v", err)
	}
	if _, err := r.CreateCourse(ctx, "event-driven-architecture"); !errors.Is(err, courses.ErrConflict) {
		t.Fatalf("duplicate course slug was accepted: %v", err)
	}
	all, err := r.ListCourses(ctx)
	if err != nil || len(all) == 0 {
		t.Fatalf("course list failed: %v", err)
	}

	first := createCourseVersionInput(t, course.ID, "1.0.0")
	firstVersion, err := r.CreateCourseVersion(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if firstVersion.SourceLanguage != "en-GB" || len(firstVersion.LearningObjectives) != 2 || firstVersion.LearningObjectives[0] != "Explain event ownership" {
		t.Fatal("course version metadata did not round trip")
	}
	if len(firstVersion.Attribution) != 2 || firstVersion.Attribution[0].Role != courses.ContributorAuthor || firstVersion.License.Identifier != "CC-BY-4.0" {
		t.Fatal("course version attribution or license did not round trip")
	}
	if _, err := r.CreateCourseVersion(ctx, first); !errors.Is(err, courses.ErrConflict) {
		t.Fatalf("duplicate course version accepted: %v", err)
	}
	second := createCourseVersionInput(t, course.ID, "1.10.0")
	if _, err := r.CreateCourseVersion(ctx, second); err != nil {
		t.Fatal(err)
	}
	versions, err := r.ListCourseVersions(ctx, course.ID)
	if err != nil || len(versions) != 2 || versions[0].Version.String() != "1.10.0" {
		t.Fatalf("version ordering failed: %#v, %v", versions, err)
	}

	other, err := r.CreateCourse(ctx, "distributed-systems")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateCourseVersion(ctx, createCourseVersionInput(t, other.ID, "1.0.0")); err != nil {
		t.Fatalf("same version on another course was rejected: %v", err)
	}

	changed, err := r.TransitionCourseVersionStatus(ctx, firstVersion.ID, courses.CourseVersionPublished, courses.CourseVersionDeprecated)
	if err != nil || changed.Status != courses.CourseVersionDeprecated || changed.Title != first.Title || changed.Description != first.Description {
		t.Fatalf("narrow status transition failed or changed content: %v", err)
	}
	if _, err := r.TransitionCourseVersionStatus(ctx, firstVersion.ID, courses.CourseVersionDeprecated, courses.CourseVersionPublished); !errors.Is(err, courses.ErrInvalidStatusTransition) {
		t.Fatalf("invalid status transition accepted: %v", err)
	}
	if _, err := r.CreateCourseVersion(ctx, createCourseVersionInput(t, courses.CourseID("00000000-0000-0000-0000-000000000001"), "2.0.0")); !errors.Is(err, courses.ErrNotFound) {
		t.Fatalf("course foreign key was not enforced: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO courses.course_version (course_id, version, status, title, description, learning_objectives, source_language, changelog, license_kind, license_display_name, attribution, published_at) VALUES ($1, '3.0.0', 'DRAFT', 'invalid', 'invalid', '[\"objective\"]', 'en', 'invalid', 'ALL_RIGHTS_RESERVED', 'All Rights Reserved', '[{\"display_name\":\"Ada\",\"role\":\"AUTHOR\",\"order\":0}]', now())", course.ID); err == nil {
		t.Fatal("database accepted invalid course version status")
	}
}

func createCourseVersionInput(t *testing.T, courseID courses.CourseID, versionText string) courses.CourseVersionInput {
	t.Helper()
	version, err := courses.ParseVersion(versionText)
	if err != nil {
		t.Fatal(err)
	}
	language, err := courses.NormalizeLanguageTag("en-gb")
	if err != nil {
		t.Fatal(err)
	}
	return courses.CourseVersionInput{
		CourseID:           courseID,
		Version:            version,
		Status:             courses.CourseVersionPublished,
		Title:              "Event-driven architecture",
		Description:        "A stable, published learning artifact.",
		LearningObjectives: []string{"Explain event ownership", "Describe asynchronous boundaries"},
		SourceLanguage:     language,
		Changelog:          "Initial publication.",
		License: courses.ContentLicense{
			Kind:        courses.ContentLicenseStandard,
			Identifier:  "CC-BY-4.0",
			DisplayName: "Creative Commons Attribution 4.0",
			URL:         "https://creativecommons.org/licenses/by/4.0/",
		},
		Attribution: []courses.ContributorSnapshot{
			{DisplayName: "Ada Author", Role: courses.ContributorAuthor, Order: 0},
			{DisplayName: "Mina Maintainer", Role: courses.ContributorMaintainer, Order: 1},
		},
		PublishedAt: time.Now().UTC(),
	}
}

func testCourseStructurePersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	r := coursespostgres.New(pool)
	course, err := r.CreateCourse(ctx, "integration-structure")
	if err != nil {
		t.Fatal(err)
	}
	version, err := r.CreateCourseVersion(ctx, createCourseVersionInput(t, course.ID, "1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	advanced, err := r.CreateModule(ctx, createModuleInput(version.ID, "advanced", "Advanced", 1))
	if err != nil {
		t.Fatal(err)
	}
	fundamentals, err := r.CreateModule(ctx, createModuleInput(version.ID, "fundamentals", "Fundamentals", 0))
	if err != nil {
		t.Fatal(err)
	}
	modules, err := r.ListModulesForCourseVersion(ctx, version.ID)
	if err != nil || len(modules) != 2 || modules[0].ID != fundamentals.ID || modules[1].ID != advanced.ID {
		t.Fatalf("module ordering did not use positions: %#v, %v", modules, err)
	}
	if _, err := r.CreateModule(ctx, createModuleInput(version.ID, "fundamentals", "Duplicate", 2)); !errors.Is(err, courses.ErrConflict) {
		t.Fatalf("duplicate module stable key accepted: %v", err)
	}
	if _, err := r.CreateModule(ctx, createModuleInput(version.ID, "other", "Duplicate position", 0)); !errors.Is(err, courses.ErrConflict) {
		t.Fatalf("duplicate module position accepted: %v", err)
	}

	duration := 25
	synchronous, err := r.CreateLesson(ctx, createLessonInput(version.ID, fundamentals.ID, "sync-vs-async", "Synchronous and asynchronous", 1, &duration))
	if err != nil {
		t.Fatal(err)
	}
	whatIsEAI, err := r.CreateLesson(ctx, createLessonInput(version.ID, fundamentals.ID, "what-is-eai", "What is EAI?", 0, nil))
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.CreateLesson(ctx, createLessonInput(version.ID, advanced.ID, "message-brokers", "Message brokers", 0, nil))
	if err != nil {
		t.Fatal(err)
	}
	lessons, err := r.ListLessonsForModule(ctx, fundamentals.ID)
	if err != nil || len(lessons) != 2 || lessons[0].ID != whatIsEAI.ID || lessons[1].ID != synchronous.ID {
		t.Fatalf("lesson ordering did not use positions: %#v, %v", lessons, err)
	}
	if lessons[1].EstimatedDurationMinutes == nil || *lessons[1].EstimatedDurationMinutes != duration || lessons[1].LearningObjectives[0] != "Explain the lesson concept" {
		t.Fatal("lesson metadata did not round trip")
	}
	if len(lessons[1].Content.Blocks) != 1 || lessons[1].Content.Blocks[0].Key != "intro" {
		t.Fatal("lesson content JSONB did not round trip")
	}
	invalidContent := createLessonInput(version.ID, fundamentals.ID, "invalid-content", "Invalid content", 3, nil)
	invalidContent.Content.SchemaVersion = 2
	if _, err := r.CreateLesson(ctx, invalidContent); err == nil {
		t.Fatal("repository accepted invalid semantic lesson content")
	}
	if _, err := pool.Exec(ctx, "UPDATE courses.lesson SET content = '{\"schemaVersion\": \"1\", \"blocks\": []}'::jsonb WHERE id = $1", synchronous.ID); err == nil {
		t.Fatal("database accepted invalid top-level lesson content shape")
	}
	if _, err := r.CreateLesson(ctx, createLessonInput(version.ID, fundamentals.ID, "sync-vs-async", "Duplicate key", 2, nil)); !errors.Is(err, courses.ErrConflict) {
		t.Fatalf("duplicate version-level lesson key accepted: %v", err)
	}
	if _, err := r.CreateLesson(ctx, createLessonInput(version.ID, fundamentals.ID, "another-lesson", "Duplicate position", 0, nil)); !errors.Is(err, courses.ErrConflict) {
		t.Fatalf("duplicate module lesson position accepted: %v", err)
	}

	prerequisite, err := r.CreateLessonPrerequisite(ctx, courses.LessonPrerequisiteInput{CourseVersionID: version.ID, LessonID: synchronous.ID, PrerequisiteStableKey: "what-is-eai", Position: 0})
	if err != nil || prerequisite.PrerequisiteLessonID != whatIsEAI.ID || prerequisite.PrerequisiteStableKey != "what-is-eai" {
		t.Fatalf("lesson prerequisite failed: %#v, %v", prerequisite, err)
	}
	if _, err := r.CreateLessonPrerequisite(ctx, courses.LessonPrerequisiteInput{CourseVersionID: version.ID, LessonID: synchronous.ID, PrerequisiteStableKey: "message-brokers", Position: 1}); err != nil {
		t.Fatal(err)
	}
	prerequisites, err := r.ListLessonPrerequisites(ctx, synchronous.ID)
	if err != nil || len(prerequisites) != 2 || prerequisites[0].PrerequisiteStableKey != "what-is-eai" || prerequisites[1].PrerequisiteStableKey != "message-brokers" {
		t.Fatalf("lesson prerequisite listing failed: %#v, %v", prerequisites, err)
	}
	if _, err := r.CreateLessonPrerequisite(ctx, courses.LessonPrerequisiteInput{CourseVersionID: version.ID, LessonID: synchronous.ID, PrerequisiteStableKey: "what-is-eai", Position: 1}); !errors.Is(err, courses.ErrConflict) {
		t.Fatalf("duplicate prerequisite accepted: %v", err)
	}
	if _, err := r.CreateLessonPrerequisite(ctx, courses.LessonPrerequisiteInput{CourseVersionID: version.ID, LessonID: whatIsEAI.ID, PrerequisiteStableKey: "what-is-eai", Position: 0}); !errors.Is(err, courses.ErrInvalidLessonPrerequisite) {
		t.Fatalf("self prerequisite accepted: %v", err)
	}

	secondVersion, err := r.CreateCourseVersion(ctx, createCourseVersionInput(t, course.ID, "1.1.0"))
	if err != nil {
		t.Fatal(err)
	}
	secondModule, err := r.CreateModule(ctx, createModuleInput(secondVersion.ID, "fundamentals", "Fundamentals", 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateLesson(ctx, createLessonInput(secondVersion.ID, secondModule.ID, "sync-vs-async", "Synchronous and asynchronous", 0, nil)); err != nil {
		t.Fatalf("same logical lesson key in another CourseVersion was rejected: %v", err)
	}
	if _, err := r.CreateLessonPrerequisite(ctx, courses.LessonPrerequisiteInput{CourseVersionID: version.ID, LessonID: whatIsEAI.ID, PrerequisiteStableKey: "sync-vs-async", Position: 0}); err != nil {
		t.Fatal(err)
	}
	secondLesson, err := r.GetLessonByCourseVersionAndKey(ctx, secondVersion.ID, "sync-vs-async")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateLessonPrerequisite(ctx, courses.LessonPrerequisiteInput{CourseVersionID: version.ID, LessonID: secondLesson.ID, PrerequisiteStableKey: "what-is-eai", Position: 0}); !errors.Is(err, courses.ErrInvalidLessonPrerequisite) {
		t.Fatalf("cross-version prerequisite accepted: %v", err)
	}
	if _, err := r.CreateModule(ctx, createModuleInput(courses.CourseVersionID("00000000-0000-0000-0000-000000000001"), "missing", "Missing", 0)); !errors.Is(err, courses.ErrNotFound) {
		t.Fatalf("module course-version foreign key was not enforced: %v", err)
	}
	if _, err := r.CreateLesson(ctx, createLessonInput(secondVersion.ID, fundamentals.ID, "invalid-module-version", "Invalid", 5, nil)); !errors.Is(err, courses.ErrNotFound) {
		t.Fatalf("lesson module/course-version consistency was not enforced: %v", err)
	}
}

func testCourseReadService(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	r := coursespostgres.New(pool)
	course, err := r.CreateCourse(ctx, "public-course-read")
	if err != nil {
		t.Fatal(err)
	}
	deprecatedInput := createCourseVersionInput(t, course.ID, "1.9.0")
	deprecatedInput.Status = courses.CourseVersionDeprecated
	if _, err := r.CreateCourseVersion(ctx, deprecatedInput); err != nil {
		t.Fatal(err)
	}
	publishedInput := createCourseVersionInput(t, course.ID, "1.10.0")
	if _, err := r.CreateCourseVersion(ctx, publishedInput); err != nil {
		t.Fatal(err)
	}
	latestInput := createCourseVersionInput(t, course.ID, "2.0.0")
	latest, err := r.CreateCourseVersion(ctx, latestInput)
	if err != nil {
		t.Fatal(err)
	}
	module, err := r.CreateModule(ctx, createModuleInput(latest.ID, "fundamentals", "Fundamentals", 0))
	if err != nil {
		t.Fatal(err)
	}
	lesson, err := r.CreateLesson(ctx, createLessonInput(latest.ID, module.ID, "what-is-eai", "What is EAI?", 0, nil))
	if err != nil {
		t.Fatal(err)
	}
	service := courses.NewReadService(r)
	preferred, err := service.Preferred(ctx, course.Slug)
	if err != nil || preferred.Version.ID != latest.ID || len(preferred.Modules) != 1 || preferred.Modules[0].Lessons[0].Lesson.ID != lesson.ID {
		t.Fatalf("preferred public read did not use highest numeric published version: %#v, %v", preferred, err)
	}
	if _, err := service.Explicit(ctx, course.Slug, deprecatedInput.Version); err != nil {
		t.Fatalf("deprecated version was not explicitly servable: %v", err)
	}
	withdrawnInput := createCourseVersionInput(t, course.ID, "3.0.0")
	withdrawnInput.Status = courses.CourseVersionWithdrawn
	withdrawn, err := r.CreateCourseVersion(ctx, withdrawnInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Explicit(ctx, course.Slug, withdrawn.Version); !errors.Is(err, courses.ErrNotServable) {
		t.Fatalf("withdrawn version was served: %v", err)
	}
	if _, _, readLesson, _, err := service.Lesson(ctx, course.Slug, latest.Version, lesson.StableKey); err != nil || readLesson.Content.SchemaVersion != courses.LessonContentSchemaVersion {
		t.Fatalf("lesson content did not round trip through read service: %#v, %v", readLesson, err)
	}
	router := authTestRouter(&authHTTP{courses: service})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/courses/public-course-read/versions/2.0.0/lessons/what-is-eai", nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("PostgreSQL-backed lesson API failed: status=%d cache=%q", response.Code, response.Header().Get("Cache-Control"))
	}
	if !strings.Contains(response.Body.String(), `"payload":{"content":{"nodes":`) {
		t.Fatalf("TEXT payload does not match the published block contract: %s", response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/courses/public-course-read/versions/3.0.0", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("withdrawn PostgreSQL-backed version was public: status=%d", response.Code)
	}
	if _, err := r.TransitionCourseVersionStatus(ctx, latest.ID, courses.CourseVersionPublished, courses.CourseVersionWithdrawn); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/courses/public-course-read/versions/2.0.0",
		"/api/courses/public-course-read/versions/2.0.0/lessons/what-is-eai",
	} {
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("withdrawn content remained public at %s: status=%d cache=%q", path, response.Code, response.Header().Get("Cache-Control"))
		}
	}
}

func createModuleInput(courseVersionID courses.CourseVersionID, stableKey, title string, position int) courses.ModuleInput {
	return courses.ModuleInput{CourseVersionID: courseVersionID, StableKey: stableKey, Title: title, Description: "Module metadata.", Position: position}
}

func createLessonInput(courseVersionID courses.CourseVersionID, moduleID courses.ModuleID, stableKey, title string, position int, duration *int) courses.LessonInput {
	return courses.LessonInput{
		CourseVersionID:          courseVersionID,
		ModuleID:                 moduleID,
		StableKey:                stableKey,
		Title:                    title,
		Description:              "Brief lesson metadata.",
		LearningObjectives:       []string{"Explain the lesson concept", "Apply the lesson concept"},
		EstimatedDurationMinutes: duration,
		Position:                 position,
		Content: courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{{
			Key: "intro", Type: courses.BlockText, Payload: courses.TextBlockPayload{Content: courses.RichText{Nodes: []courses.RichTextNode{{
				Type: "paragraph", Content: []courses.RichTextInline{{Type: "text", Text: "Published lesson content."}},
			}}}},
		}}},
	}
}
