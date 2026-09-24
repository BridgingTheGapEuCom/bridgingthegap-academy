package translations

import (
	"context"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

// ApplicationService is the server-side authorization boundary. Existing
// resources are always loaded before authorization, so route/body metadata
// cannot select a more favorable source Course.
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
func (s *ApplicationService) CreateForCourseVersion(ctx context.Context, actorID string, courseID courses.CourseID, version courses.Version, target courses.LanguageTag) (CourseTranslation, error) {
	if s == nil || s.translations == nil {
		return CourseTranslation{}, ErrInvalidTranslation
	}
	source, err := s.translations.PublishedSource(ctx, courseID, version)
	if err != nil {
		return CourseTranslation{}, err
	}
	return s.Create(ctx, actorID, source.ID, target)
}
func (s *ApplicationService) Workspace(ctx context.Context, actorID string, id TranslationID) (CourseTranslation, error) {
	return s.loadAuthorized(ctx, actorID, id, CapabilityRead)
}
func (s *ApplicationService) Update(ctx context.Context, actorID string, id TranslationID, expected int64, tree TranslationTree) (CourseTranslation, error) {
	if _, err := s.loadAuthorized(ctx, actorID, id, CapabilityEdit); err != nil {
		return CourseTranslation{}, err
	}
	return s.translations.Update(ctx, id, expected, tree)
}
func (s *ApplicationService) Patch(ctx context.Context, actorID string, id TranslationID, expected int64, changes []TextChange) (CourseTranslation, error) {
	translation, err := s.loadAuthorized(ctx, actorID, id, CapabilityEdit)
	if err != nil {
		return CourseTranslation{}, err
	}
	if translation.Revision != expected {
		return CourseTranslation{}, ErrRevisionMismatch
	}
	tree, err := ApplyTextChanges(translation.Tree, changes)
	if err != nil {
		return CourseTranslation{}, err
	}
	return s.translations.Update(ctx, id, expected, tree)
}
func (s *ApplicationService) Publish(ctx context.Context, actorID string, id TranslationID) (TranslationPublication, error) {
	translation, err := s.loadAuthorized(ctx, actorID, id, CapabilityPublish)
	if err != nil {
		return TranslationPublication{}, err
	}
	return s.PublishExpected(ctx, actorID, id, translation.Revision)
}
func (s *ApplicationService) PublishExpected(ctx context.Context, actorID string, id TranslationID, expected int64) (TranslationPublication, error) {
	translation, err := s.loadAuthorized(ctx, actorID, id, CapabilityPublish)
	if err != nil {
		return TranslationPublication{}, err
	}
	if translation.Revision != expected {
		return TranslationPublication{}, ErrRevisionMismatch
	}
	source, err := s.translations.source.GetImmutableCourseVersion(ctx, translation.Source.CourseVersionID)
	if err != nil || translation.Tree.ValidateAgainstSource(source) != nil {
		return TranslationPublication{}, ErrSourceIntegrity
	}
	view, err := buildWorkspaceView(translation, source)
	if err != nil {
		return TranslationPublication{}, ErrSourceIntegrity
	}
	if !view.Completeness.Complete {
		return TranslationPublication{}, ErrTranslationIncomplete
	}
	return s.translations.PublishExpected(ctx, id, expected)
}
func (s *ApplicationService) ListForCourseVersion(ctx context.Context, actorID string, courseID courses.CourseID, version courses.Version) ([]CourseTranslation, error) {
	if s == nil || s.translations == nil {
		return nil, ErrInvalidTranslation
	}
	source, err := s.translations.PublishedSource(ctx, courseID, version)
	if err != nil {
		return nil, err
	}
	binding, err := sourceFromImmutable(source)
	if err != nil {
		return nil, ErrInvalidTranslation
	}
	if err := s.authorizer.Authorize(ctx, actorID, CapabilityRead, SourceCourseResource(binding)); err != nil {
		return nil, err
	}
	return s.translations.ListBySourceVersion(ctx, source.ID)
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
