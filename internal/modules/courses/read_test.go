package courses

import (
	"context"
	"errors"
	"testing"
)

// readRepository embeds the production-facing interface so this focused policy
// fake only implements operations the read service uses in each test.
type readRepository struct {
	Repository
	course        Course
	versions      []CourseVersion
	modules       []Module
	lessons       []Lesson
	prerequisites []LessonPrerequisite
}

func (r readRepository) GetCourseBySlug(context.Context, string) (Course, error) {
	return r.course, nil
}

func (r readRepository) ListCourseVersions(context.Context, CourseID) ([]CourseVersion, error) {
	return r.versions, nil
}

func (r readRepository) ListPublishedCourseVersions(context.Context) ([]CourseVersion, error) {
	result := make([]CourseVersion, 0, len(r.versions))
	for _, version := range r.versions {
		if version.Status == CourseVersionPublished {
			result = append(result, version)
		}
	}
	return result, nil
}

func (r readRepository) ListModulesForCourseVersion(context.Context, CourseVersionID) ([]Module, error) {
	return r.modules, nil
}

func (r readRepository) ListLessonsForCourseVersion(context.Context, CourseVersionID) ([]Lesson, error) {
	return r.lessons, nil
}

func (r readRepository) ListLessonPrerequisitesForCourseVersion(context.Context, CourseVersionID) ([]LessonPrerequisite, error) {
	return r.prerequisites, nil
}

func (r readRepository) GetCourseVersionByCourseAndVersion(_ context.Context, _ CourseID, version Version) (CourseVersion, error) {
	for _, item := range r.versions {
		if item.Version == version {
			return item, nil
		}
	}
	return CourseVersion{}, ErrNotFound
}

func TestReadServicePreferredUsesHighestPublishedNumericVersion(t *testing.T) {
	course := Course{ID: "course", Slug: "event-driven-architecture"}
	service := NewReadService(readRepository{course: course, versions: []CourseVersion{
		{ID: "deprecated", CourseID: course.ID, Version: Version{Major: 9}, Status: CourseVersionDeprecated},
		{ID: "two", CourseID: course.ID, Version: Version{Major: 2}, Status: CourseVersionPublished},
		{ID: "one-ten", CourseID: course.ID, Version: Version{Major: 1, Minor: 10}, Status: CourseVersionPublished},
		{ID: "one-nine", CourseID: course.ID, Version: Version{Major: 1, Minor: 9}, Status: CourseVersionPublished},
	}})

	got, err := service.Preferred(context.Background(), course.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version.ID != "two" {
		t.Fatalf("preferred version = %s, want highest PUBLISHED numeric SemVer", got.Version.ID)
	}
}

func TestReadServiceHistoricalServingPolicyAndStructure(t *testing.T) {
	version := Version{Major: 1}
	service := NewReadService(readRepository{
		course:        Course{ID: "course", Slug: "event-driven-architecture"},
		versions:      []CourseVersion{{ID: "archived", CourseID: "course", Version: version, Status: CourseVersionArchived}},
		modules:       []Module{{ID: "module", CourseVersionID: "archived", StableKey: "fundamentals", Position: 0}},
		lessons:       []Lesson{{ID: "lesson", CourseVersionID: "archived", ModuleID: "module", StableKey: "what-is-eai", Position: 0}},
		prerequisites: []LessonPrerequisite{{LessonID: "lesson", PrerequisiteStableKey: "intro", Position: 0}},
	})

	got, err := service.Explicit(context.Background(), "event-driven-architecture", version)
	if err != nil {
		t.Fatalf("archived explicit version should be historical-served: %v", err)
	}
	if len(got.Modules) != 1 || len(got.Modules[0].Lessons) != 1 || got.Modules[0].Lessons[0].RecommendedPrerequisiteKeys[0] != "intro" {
		t.Fatalf("ordered structure/prerequisites were not retained: %#v", got.Modules)
	}

	withdrawn := NewReadService(readRepository{course: Course{ID: "course", Slug: "event-driven-architecture"}, versions: []CourseVersion{{ID: "withdrawn", CourseID: "course", Version: version, Status: CourseVersionWithdrawn}}})
	if _, err := withdrawn.Explicit(context.Background(), "event-driven-architecture", version); !errors.Is(err, ErrNotServable) {
		t.Fatalf("withdrawn explicit version error = %v, want ErrNotServable", err)
	}
}

func TestReadServiceNoPublishedVersionIsNotCurrent(t *testing.T) {
	service := NewReadService(readRepository{course: Course{ID: "course", Slug: "event-driven-architecture"}, versions: []CourseVersion{{ID: "archived", CourseID: "course", Version: Version{Major: 1}, Status: CourseVersionArchived}}})
	if _, err := service.Preferred(context.Background(), "event-driven-architecture"); !errors.Is(err, ErrNotServable) {
		t.Fatalf("preferred no-published error = %v, want ErrNotServable", err)
	}
}
