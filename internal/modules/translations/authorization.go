package translations

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

// Capability names the Translation-specific operation, independently of raw
// Course attribution roles. Future translator membership can extend
// roleAllows without changing callers or transport boundaries.
type Capability string

const (
	CapabilityRead    Capability = "translation.read"
	CapabilityCreate  Capability = "translation.create"
	CapabilityEdit    Capability = "translation.edit"
	CapabilityPublish Capability = "translation.publish"
)

var (
	ErrAuthorizationDenied      = errors.New("translation authorization denied")
	ErrAuthorizationUnavailable = errors.New("translation authorization unavailable")
)

// Resource is bound to the immutable source CourseVersion. Its Course ID,
// rather than a client assertion or workspace creator, is the policy scope.
type Resource struct{ source SourceCourseVersion }

func SourceCourseResource(source SourceCourseVersion) Resource { return Resource{source: source} }

type Authorizer interface {
	Authorize(context.Context, string, Capability, Resource) error
}

// PublishedCourseAuthorityReader is the existing durable published-attribution
// source. It intentionally receives no global roles, so administrator status
// cannot become an implicit Translation bypass.
type PublishedCourseAuthorityReader interface {
	ListPublishedCourseVersions(context.Context) ([]courses.CourseVersion, error)
}

type AuthorizationService struct {
	courses PublishedCourseAuthorityReader
}

func NewAuthorizationService(courses PublishedCourseAuthorityReader) AuthorizationService {
	return AuthorizationService{courses: courses}
}

func (s AuthorizationService) Authorize(ctx context.Context, actorID string, capability Capability, resource Resource) error {
	if !validUUID(actorID) || resource.source.Validate() != nil || !knownCapability(capability) {
		return ErrAuthorizationDenied
	}
	if s.courses == nil {
		return ErrAuthorizationUnavailable
	}
	versions, err := s.courses.ListPublishedCourseVersions(ctx)
	if err != nil {
		return ErrAuthorizationUnavailable
	}
	for _, version := range versions {
		if version.CourseID != resource.source.CourseID || version.Status != courses.CourseVersionPublished {
			continue
		}
		for _, contributor := range version.Attribution {
			if contributor.UserID == actorID && roleAllows(contributor.Role, capability) {
				return nil
			}
		}
	}
	return ErrAuthorizationDenied
}
func knownCapability(capability Capability) bool {
	switch capability {
	case CapabilityRead, CapabilityCreate, CapabilityEdit, CapabilityPublish:
		return true
	default:
		return false
	}
}
func roleAllows(role courses.ContributorRole, capability Capability) bool {
	return knownCapability(capability) && (role == courses.ContributorAuthor || role == courses.ContributorMaintainer)
}
