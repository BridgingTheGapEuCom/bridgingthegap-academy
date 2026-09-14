package courses

import (
	"context"
	"errors"
)

var ErrNotServable = errors.New("course version is not learner-servable")

type ModuleStructure struct {
	Module  Module
	Lessons []LessonStructure
}

// LessonSummary omits the potentially large block document from course outlines.
type LessonSummary struct {
	ID                       LessonID
	ModuleID                 ModuleID
	StableKey                string
	Title                    string
	Description              string
	LearningObjectives       []string
	EstimatedDurationMinutes *int
	Position                 int
}

// LessonStructure is the public-read projection of version-owned lesson
// metadata. Recommended prerequisites are advisory stable keys, never locks.
type LessonStructure struct {
	Lesson                      LessonSummary
	RecommendedPrerequisiteKeys []string
}
type CourseRead struct {
	Course  Course
	Version CourseVersion
	Modules []ModuleStructure
}

// ReadService applies learner serving policy without depending on HTTP. The
// highest numeric PUBLISHED version is preferred; DEPRECATED and ARCHIVED are
// explicit-history only, while WITHDRAWN is deliberately never served.
type ReadService struct{ repository Repository }

func NewReadService(repository Repository) *ReadService { return &ReadService{repository: repository} }

func (s *ReadService) Preferred(ctx context.Context, slug string) (CourseRead, error) {
	course, err := s.repository.GetCourseBySlug(ctx, slug)
	if err != nil {
		return CourseRead{}, err
	}
	versions, err := s.repository.ListCourseVersions(ctx, course.ID)
	if err != nil {
		return CourseRead{}, err
	}
	var preferred *CourseVersion
	for i := range versions {
		version := &versions[i]
		if version.Status == CourseVersionPublished && (preferred == nil || version.Version.Compare(preferred.Version) > 0) {
			preferred = version
		}
	}
	if preferred == nil {
		return CourseRead{}, ErrNotServable
	}
	return s.load(ctx, course, *preferred)
}
func (s *ReadService) Explicit(ctx context.Context, slug string, version Version) (CourseRead, error) {
	course, v, err := s.explicitVersion(ctx, slug, version)
	if err != nil {
		return CourseRead{}, err
	}
	return s.load(ctx, course, v)
}
func (s *ReadService) explicitVersion(ctx context.Context, slug string, version Version) (Course, CourseVersion, error) {
	course, err := s.repository.GetCourseBySlug(ctx, slug)
	if err != nil {
		return Course{}, CourseVersion{}, err
	}
	v, err := s.repository.GetCourseVersionByCourseAndVersion(ctx, course.ID, version)
	if err != nil {
		return Course{}, CourseVersion{}, err
	}
	if v.Status != CourseVersionPublished && v.Status != CourseVersionDeprecated && v.Status != CourseVersionArchived {
		return Course{}, CourseVersion{}, ErrNotServable
	}
	return course, v, nil
}
func (s *ReadService) Discover(ctx context.Context) ([]CourseRead, error) {
	courses, err := s.repository.ListCourses(ctx)
	if err != nil {
		return nil, err
	}
	published, err := s.repository.ListPublishedCourseVersions(ctx)
	if err != nil {
		return nil, err
	}
	preferred := make(map[CourseID]CourseVersion, len(courses))
	for _, version := range published {
		current, exists := preferred[version.CourseID]
		if version.Status == CourseVersionPublished && (!exists || version.Version.Compare(current.Version) > 0) {
			preferred[version.CourseID] = version
		}
	}
	result := make([]CourseRead, 0, len(courses))
	for _, course := range courses {
		if version, exists := preferred[course.ID]; exists {
			result = append(result, CourseRead{Course: course, Version: version})
		}
	}
	return result, nil
}
func (s *ReadService) Lesson(ctx context.Context, slug string, version Version, key string) (CourseRead, Module, Lesson, []LessonPrerequisite, error) {
	course, courseVersion, err := s.explicitVersion(ctx, slug, version)
	if err != nil {
		return CourseRead{}, Module{}, Lesson{}, nil, err
	}
	lesson, err := s.repository.GetLessonByCourseVersionAndKey(ctx, courseVersion.ID, key)
	if err != nil {
		return CourseRead{}, Module{}, Lesson{}, nil, err
	}
	module, err := s.repository.GetModule(ctx, lesson.ModuleID)
	if err != nil {
		return CourseRead{}, Module{}, Lesson{}, nil, err
	}
	prerequisites, err := s.repository.ListLessonPrerequisites(ctx, lesson.ID)
	if err != nil {
		return CourseRead{}, Module{}, Lesson{}, nil, err
	}
	return CourseRead{Course: course, Version: courseVersion}, module, lesson, prerequisites, nil
}
func (s *ReadService) load(ctx context.Context, course Course, version CourseVersion) (CourseRead, error) {
	modules, err := s.repository.ListModulesForCourseVersion(ctx, version.ID)
	if err != nil {
		return CourseRead{}, err
	}
	lessons, err := s.repository.ListLessonSummariesForCourseVersion(ctx, version.ID)
	if err != nil {
		return CourseRead{}, err
	}
	prerequisites, err := s.repository.ListLessonPrerequisitesForCourseVersion(ctx, version.ID)
	if err != nil {
		return CourseRead{}, err
	}
	prerequisiteKeys := make(map[LessonID][]string, len(lessons))
	for _, prerequisite := range prerequisites {
		prerequisiteKeys[prerequisite.LessonID] = append(prerequisiteKeys[prerequisite.LessonID], prerequisite.PrerequisiteStableKey)
	}
	byModule := make(map[ModuleID][]LessonStructure, len(modules))
	for _, lesson := range lessons {
		byModule[lesson.ModuleID] = append(byModule[lesson.ModuleID], LessonStructure{Lesson: lesson, RecommendedPrerequisiteKeys: prerequisiteKeys[lesson.ID]})
	}
	structure := make([]ModuleStructure, 0, len(modules))
	for _, module := range modules {
		structure = append(structure, ModuleStructure{Module: module, Lessons: byModule[module.ID]})
	}
	return CourseRead{Course: course, Version: version, Modules: structure}, nil
}
