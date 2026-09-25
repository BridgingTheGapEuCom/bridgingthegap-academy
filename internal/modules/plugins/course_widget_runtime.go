package plugins

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

// CourseWidgetVersionRepository is deliberately Course-owned. The launch
// service receives immutable content and derives all release/configuration
// facts from it; callers never submit plugin coordinates or configuration.
type CourseWidgetVersionRepository interface {
	GetPublishedImmutableCourseVersionByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error)
}

type CourseWidgetLaunchService struct {
	versions CourseWidgetVersionRepository
	runtime  *RuntimeService
}

func NewCourseWidgetLaunchService(versions CourseWidgetVersionRepository, runtime *RuntimeService) *CourseWidgetLaunchService {
	return &CourseWidgetLaunchService{versions: versions, runtime: runtime}
}

func (s *CourseWidgetLaunchService) Prepare(ctx context.Context, courseID courses.CourseID, version courses.Version, lessonKey, placementKey string) (RuntimeLaunch, error) {
	if s == nil || s.versions == nil || s.runtime == nil || courseID == "" || !version.Valid() {
		return RuntimeLaunch{}, ErrLaunchDenied
	}
	lessonKeyNormalized, lessonErr := courses.NormalizeStructureKey(lessonKey)
	placementKeyNormalized, placementErr := courses.NormalizeStructureKey(placementKey)
	if lessonErr != nil || placementErr != nil || lessonKeyNormalized != lessonKey || placementKeyNormalized != placementKey {
		return RuntimeLaunch{}, ErrLaunchDenied
	}
	published, err := s.versions.GetPublishedImmutableCourseVersionByCourseAndVersion(ctx, courseID, version)
	if err != nil || published.ID == "" || published.CourseVersion.CourseID != courseID || published.CourseVersion.Version != version || published.CourseVersion.Status != courses.CourseVersionPublished {
		return RuntimeLaunch{}, ErrLaunchDenied
	}
	for _, module := range published.Modules {
		for _, lesson := range module.Lessons {
			if lesson.StableKey != lessonKey {
				continue
			}
			for _, block := range lesson.Content.Blocks {
				if block.Key != placementKey {
					continue
				}
				placement, ok := block.Payload.(courses.PluginWidgetBlockPayload)
				if !ok || block.Type != courses.BlockPluginWidget {
					return RuntimeLaunch{}, ErrLaunchDenied
				}
				context := CourseWidgetRuntimeContext{
					CourseID: courseID, CourseVersionID: published.ID, CourseVersion: version.String(),
					LessonKey: lessonKey, PlacementKey: placementKey,
					PresentationLanguage: published.CourseVersion.SourceLanguage,
					Configuration:        append([]byte(nil), placement.Configuration...),
				}
				return s.runtime.PrepareCourseWidgetRuntime(ctx, CourseWidgetRuntimePlacement{Context: context, Placement: placement})
			}
			return RuntimeLaunch{}, ErrLaunchDenied
		}
	}
	return RuntimeLaunch{}, ErrLaunchDenied
}

func IsCourseWidgetLaunchUnavailable(err error) bool {
	return errors.Is(err, ErrLaunchDenied) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrResourceInvalid)
}
