package translations

import (
	"context"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

// ApplicationService is the server-side authorization boundary used by future
// HTTP handlers. It loads an existing Translation before authorizing so callers
// cannot select a more favorable source Course in a route or request body.
type ApplicationService struct {
	translations *Service
	authorizer   Authorizer
}

func NewApplicationService(translations *Service, authorizer Authorizer) (*ApplicationService, error) {
	if translations == nil || authorizer == nil {
		return nil, ErrInvalidTranslation
	}
	return &ApplicationService{translations: translations, authorizer: authorizer}, nil
}

func (s *ApplicationService) Create(ctx context.Context, actorID string, sourceID courses.CourseVersionID, target courses.LanguageTag) (CourseTranslation, error) {
	if s == nil || s.translations == nil {
		return CourseTranslation{}, ErrInvalidTranslation
	}
	source, err := s.translations.source.GetImmutableCourseVersion(ctx, sourceID)
	if err != nil {
		return CourseTranslation{}, err
	}
	binding, err := sourceFromImmutable(source)
	if err != nil || source.CourseVersion.Status != courses.CourseVersionPublished {
		return CourseTranslation{}, ErrInvalidTranslation
	}
	if err := s.authorizer.Authorize(ctx, actorID, CapabilityCreate, SourceCourseResource(binding)); err != nil {
		return CourseTranslation{}, err
	}
	return s.translations.Create(ctx, sourceID, target, actorID)
}
func (s *ApplicationService) Workspace(ctx context.Context, actorID string, id TranslationID) (CourseTranslation, error) {
	translation, err := s.loadAuthorized(ctx, actorID, id, CapabilityRead)
	if err != nil {
		return CourseTranslation{}, err
	}
	return translation, nil
}
func (s *ApplicationService) Update(ctx context.Context, actorID string, id TranslationID, expected int64, tree TranslationTree) (CourseTranslation, error) {
	if _, err := s.loadAuthorized(ctx, actorID, id, CapabilityEdit); err != nil {
		return CourseTranslation{}, err
	}
	return s.translations.Update(ctx, id, expected, tree)
}
func (s *ApplicationService) Publish(ctx context.Context, actorID string, id TranslationID) (TranslationPublication, error) {
	if _, err := s.loadAuthorized(ctx, actorID, id, CapabilityPublish); err != nil {
		return TranslationPublication{}, err
	}
	return s.translations.Publish(ctx, id)
}
func (s *ApplicationService) loadAuthorized(ctx context.Context, actorID string, id TranslationID, capability Capability) (CourseTranslation, error) {
	if s == nil || s.translations == nil || s.authorizer == nil {
		return CourseTranslation{}, ErrInvalidTranslation
	}
	translation, err := s.translations.Get(ctx, id)
	if err != nil {
		return CourseTranslation{}, err
	}
	if err := s.authorizer.Authorize(ctx, actorID, capability, SourceCourseResource(translation.Source)); err != nil {
		return CourseTranslation{}, err
	}
	return translation, nil
}
