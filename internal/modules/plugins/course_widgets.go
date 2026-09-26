package plugins

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

// CourseWidgetContentValidator is a neutral adapter for Authoring's canonical
// LessonContent replacement boundary. It owns no Authoring state and only
// attests that every pinned widget placement is launchable right now.
type CourseWidgetContentValidator struct{ registry *RegistryService }

func NewCourseWidgetContentValidator(registry *RegistryService) *CourseWidgetContentValidator {
	return &CourseWidgetContentValidator{registry: registry}
}

func (v *CourseWidgetContentValidator) ValidateLessonContent(ctx context.Context, content courses.LessonContent) error {
	if v == nil || v.registry == nil {
		return ErrLaunchDenied
	}
	for _, block := range content.Blocks {
		placement, ok := block.Payload.(courses.PluginWidgetBlockPayload)
		if !ok {
			continue
		}
		if err := v.registry.ValidateCourseWidgetPlacement(ctx, placement); err != nil {
			return err
		}
	}
	return nil
}

// ValidateReviewSnapshot is intentionally structural: it receives only the
// frozen content supplied by Authoring and does not consult mutable Draft rows.
func (v *CourseWidgetContentValidator) ValidateReviewSnapshot(ctx context.Context, contents []courses.LessonContent) error {
	for _, content := range contents {
		if err := v.ValidateLessonContent(ctx, content); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCourseWidgetPlacement verifies the exact installed immutable
// release, not just a plugin name. Disabled or no-longer-trusted releases are
// intentionally unavailable for new Draft placement and publication.
func (s *RegistryService) ValidateCourseWidgetPlacement(ctx context.Context, placement courses.PluginWidgetBlockPayload) error {
	if s == nil || placement.Validate() != nil {
		return ErrLaunchDenied
	}
	release, err := s.RuntimeRelease(ctx, PluginID(placement.PluginID), placement.PluginVersion)
	if err != nil {
		return err
	}
	if release.Release.ArtifactDigest != placement.ArtifactDigest {
		return ErrLaunchDenied
	}
	for _, entry := range release.Manifest.Entrypoints {
		if entry.ID != placement.WidgetID {
			continue
		}
		if entry.Type != TypeCourseWidget || placement.WidgetType != string(TypeCourseWidget) {
			return ErrLaunchDenied
		}
		resource, err := s.Resource(ctx, release.Release, entry.Resource)
		if err != nil || resource.SHA256 != declaredDigest(release.Manifest, entry.Resource) {
			return ErrResourceInvalid
		}
		return nil
	}
	return ErrLaunchDenied
}

// CourseWidgetEntrypoints is the authoring-safe discovery view. It exposes no
// signing keys, storage paths, or package bytes.
type CourseWidgetEntrypoint struct {
	PluginID       PluginID
	PluginName     string
	PluginVersion  string
	ArtifactDigest string
	WidgetID       string
	WidgetName     string
	Description    string
	Trust          TrustLevel
}

func (s *RegistryService) DiscoverCourseWidgets(ctx context.Context) ([]CourseWidgetEntrypoint, error) {
	releases, err := s.CourseWidgets(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]CourseWidgetEntrypoint, 0)
	for _, release := range releases {
		for _, entry := range release.Manifest.Entrypoints {
			if entry.Type != TypeCourseWidget {
				continue
			}
			result = append(result, CourseWidgetEntrypoint{
				PluginID: release.Release.PluginID, PluginName: release.Manifest.Name,
				PluginVersion: release.Release.Version, ArtifactDigest: release.Release.ArtifactDigest,
				WidgetID: entry.ID, WidgetName: entry.Name, Description: release.Manifest.Description,
				Trust: release.CurrentTrust,
			})
		}
	}
	return result, nil
}

func IsCourseWidgetPlacementUnavailable(err error) bool {
	return errors.Is(err, ErrLaunchDenied) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrResourceInvalid)
}
