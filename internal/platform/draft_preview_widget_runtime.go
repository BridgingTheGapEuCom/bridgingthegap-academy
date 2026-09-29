package platform

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
)

type authoringDraftPreviewPlacementSource struct {
	reads      *authoring.ReadService
	repository authoring.ReadRepository
}

func (s *authoringDraftPreviewPlacementSource) authorizedPlacement(ctx context.Context, actor identity.AuthenticatedActor, draftID authoring.DraftID, lessonID authoring.LessonID, placementKey string) (courses.PluginWidgetBlockPayload, error) {
	if s == nil || s.reads == nil {
		return courses.PluginWidgetBlockPayload{}, authoring.ErrAuthorizationUnavailable
	}
	lesson, _, err := s.reads.Lesson(ctx, actor, draftID, lessonID)
	if err != nil {
		return courses.PluginWidgetBlockPayload{}, err
	}
	return exactDraftPreviewPlacement(lesson, placementKey)
}

// DraftPreviewPlacement is used only after a preview runtime UUID and bearer
// have already been established. It rechecks the authoritative placement for
// refresh without exposing any Draft data to the widget.
func (s *authoringDraftPreviewPlacementSource) DraftPreviewPlacement(ctx context.Context, draftID, lessonID, placementKey string) (courses.PluginWidgetBlockPayload, error) {
	if s == nil || s.repository == nil {
		return courses.PluginWidgetBlockPayload{}, plugins.ErrLaunchDenied
	}
	lesson, _, err := s.repository.ReadLesson(ctx, authoring.DraftID(draftID), authoring.LessonID(lessonID))
	if err != nil {
		return courses.PluginWidgetBlockPayload{}, plugins.ErrLaunchDenied
	}
	placement, err := exactDraftPreviewPlacement(lesson, placementKey)
	if err != nil {
		return courses.PluginWidgetBlockPayload{}, plugins.ErrLaunchDenied
	}
	return placement, nil
}

func exactDraftPreviewPlacement(lesson authoring.DraftLesson, placementKey string) (courses.PluginWidgetBlockPayload, error) {
	normalized, err := courses.NormalizeStructureKey(placementKey)
	if err != nil || normalized != placementKey {
		return courses.PluginWidgetBlockPayload{}, plugins.ErrLaunchDenied
	}
	for _, block := range lesson.Content.Blocks {
		if block.Key != placementKey {
			continue
		}
		placement, ok := block.Payload.(courses.PluginWidgetBlockPayload)
		if !ok || block.Type != courses.BlockPluginWidget || placement.Validate() != nil {
			return courses.PluginWidgetBlockPayload{}, plugins.ErrLaunchDenied
		}
		return placement, nil
	}
	return courses.PluginWidgetBlockPayload{}, plugins.ErrLaunchDenied
}

type draftPreviewWidgetLaunchService struct {
	source  *authoringDraftPreviewPlacementSource
	runtime *plugins.RuntimeService
}

func (s *draftPreviewWidgetLaunchService) Prepare(ctx context.Context, actor identity.AuthenticatedActor, draftID authoring.DraftID, lessonID authoring.LessonID, placementKey string) (plugins.RuntimeLaunch, error) {
	if s == nil || s.runtime == nil || s.source == nil {
		return plugins.RuntimeLaunch{}, plugins.ErrLaunchDenied
	}
	placement, err := s.source.authorizedPlacement(ctx, actor, draftID, lessonID, placementKey)
	if err != nil {
		return plugins.RuntimeLaunch{}, err
	}
	previewContext := plugins.DraftPreviewWidgetRuntimeContext{ContextType: plugins.RuntimeModeDraftPreview, PlacementKey: placementKey, Configuration: append([]byte(nil), placement.Configuration...)}
	return s.runtime.PrepareDraftPreviewWidgetRuntime(ctx, plugins.DraftPreviewWidgetRuntimePlacement{DraftID: string(draftID), LessonID: string(lessonID), Context: previewContext, Placement: placement})
}

func isDraftPreviewWidgetUnavailable(err error) bool {
	return errors.Is(err, authoring.ErrNotFound) || errors.Is(err, authoring.ErrAuthorizationDenied) || plugins.IsCourseWidgetLaunchUnavailable(err)
}
