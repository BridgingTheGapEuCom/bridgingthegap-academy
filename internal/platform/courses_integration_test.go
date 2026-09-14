//go:build integration

package platform

import (
	"context"
	"errors"
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
