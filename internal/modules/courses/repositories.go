package courses

import (
	"context"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("course record not found")
var ErrConflict = errors.New("course record conflicts with existing data")
var ErrInvalidStatusTransition = errors.New("invalid course version status transition")
var ErrInvalidLessonPrerequisite = errors.New("invalid lesson prerequisite")
var ErrCourseVersionAlreadyExists = fmt.Errorf("%w: course version already exists", ErrConflict)

// Repository deliberately has no content update or delete methods. Published
// CourseVersion, Module, and Lesson content is created once and retained for
// historical provenance.
type Repository interface {
	CreateCourse(context.Context, string) (Course, error)
	GetCourse(context.Context, CourseID) (Course, error)
	GetCourseBySlug(context.Context, string) (Course, error)
	ListCourses(context.Context) ([]Course, error)

	CreateCourseVersion(context.Context, CourseVersionInput) (CourseVersion, error)
	GetCourseVersion(context.Context, CourseVersionID) (CourseVersion, error)
	GetCourseVersionByCourseAndVersion(context.Context, CourseID, Version) (CourseVersion, error)
	ListCourseVersions(context.Context, CourseID) ([]CourseVersion, error)
	ListPublishedCourseVersions(context.Context) ([]CourseVersion, error)
	TransitionCourseVersionStatus(context.Context, CourseVersionID, CourseVersionStatus, CourseVersionStatus) (CourseVersion, error)

	CreateModule(context.Context, ModuleInput) (Module, error)
	GetModule(context.Context, ModuleID) (Module, error)
	GetModuleByCourseVersionAndKey(context.Context, CourseVersionID, string) (Module, error)
	ListModulesForCourseVersion(context.Context, CourseVersionID) ([]Module, error)

	CreateLesson(context.Context, LessonInput) (Lesson, error)
	GetLesson(context.Context, LessonID) (Lesson, error)
	GetLessonByCourseVersionAndKey(context.Context, CourseVersionID, string) (Lesson, error)
	ListLessonsForModule(context.Context, ModuleID) ([]Lesson, error)
	ListLessonSummariesForCourseVersion(context.Context, CourseVersionID) ([]LessonSummary, error)

	CreateLessonPrerequisite(context.Context, LessonPrerequisiteInput) (LessonPrerequisite, error)
	ListLessonPrerequisites(context.Context, LessonID) ([]LessonPrerequisite, error)
	ListLessonPrerequisitesForCourseVersion(context.Context, CourseVersionID) ([]LessonPrerequisite, error)
}

// ImmutableCourseVersionRepository is the atomic publication persistence
// boundary. It never reaches back into Authoring.
type ImmutableCourseVersionRepository interface {
	StoreImmutableCourseVersion(context.Context, ImmutableCourseVersion) (ImmutableCourseVersion, error)
	GetImmutableCourseVersion(context.Context, CourseVersionID) (ImmutableCourseVersion, error)
}

type CourseVersionStore struct {
	repository ImmutableCourseVersionRepository
}

func NewCourseVersionStore(repository ImmutableCourseVersionRepository) *CourseVersionStore {
	return &CourseVersionStore{repository: repository}
}

func (s *CourseVersionStore) Store(ctx context.Context, version ImmutableCourseVersion) (ImmutableCourseVersion, error) {
	if s == nil || s.repository == nil || version.ValidateForPersistence() != nil {
		return ImmutableCourseVersion{}, ErrInvalidImmutableCourseVersion
	}
	return s.repository.StoreImmutableCourseVersion(ctx, version)
}
