package courses

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("course record not found")
var ErrConflict = errors.New("course record conflicts with existing data")
var ErrInvalidStatusTransition = errors.New("invalid course version status transition")
var ErrInvalidLessonPrerequisite = errors.New("invalid lesson prerequisite")

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
	TransitionCourseVersionStatus(context.Context, CourseVersionID, CourseVersionStatus, CourseVersionStatus) (CourseVersion, error)

	CreateModule(context.Context, ModuleInput) (Module, error)
	GetModule(context.Context, ModuleID) (Module, error)
	GetModuleByCourseVersionAndKey(context.Context, CourseVersionID, string) (Module, error)
	ListModulesForCourseVersion(context.Context, CourseVersionID) ([]Module, error)

	CreateLesson(context.Context, LessonInput) (Lesson, error)
	GetLesson(context.Context, LessonID) (Lesson, error)
	GetLessonByCourseVersionAndKey(context.Context, CourseVersionID, string) (Lesson, error)
	ListLessonsForModule(context.Context, ModuleID) ([]Lesson, error)

	CreateLessonPrerequisite(context.Context, LessonPrerequisiteInput) (LessonPrerequisite, error)
	ListLessonPrerequisites(context.Context, LessonID) ([]LessonPrerequisite, error)
}
