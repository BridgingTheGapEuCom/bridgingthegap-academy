package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

// CourseWidgetDescriptor is the safe authoring discovery contract. Plugin
// trust infrastructure is intentionally hidden behind this neutral value.
type CourseWidgetDescriptor struct {
	PluginID       string
	PluginName     string
	PluginVersion  string
	ArtifactDigest string
	WidgetID       string
	WidgetName     string
	Description    string
	Trust          string
}

type CourseWidgetDiscoveryProvider interface {
	DiscoverCourseWidgets(context.Context) ([]CourseWidgetDescriptor, error)
}

type CourseWidgetDiscoveryService struct {
	provider   CourseWidgetDiscoveryProvider
	authorizer Authorizer
}

func NewCourseWidgetDiscoveryService(provider CourseWidgetDiscoveryProvider, authorizer Authorizer) *CourseWidgetDiscoveryService {
	return &CourseWidgetDiscoveryService{provider: provider, authorizer: authorizer}
}

func (s *CourseWidgetDiscoveryService) List(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID) ([]CourseWidgetDescriptor, error) {
	if s == nil || s.provider == nil || s.authorizer == nil {
		return nil, ErrAuthorizationUnavailable
	}
	if err := s.authorizer.Authorize(ctx, actor, CapabilityContentEdit, DraftResource(draftID)); err != nil {
		if errors.Is(err, ErrAuthorizationDenied) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.provider.DiscoverCourseWidgets(ctx)
}
