package translations

import (
	"context"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

// SourceRepository is deliberately exact and immutable. Courses remains the
// source-of-truth owner; Translations cannot write it or select latest.
type SourceRepository interface {
	GetImmutableCourseVersion(context.Context, courses.CourseVersionID) (courses.ImmutableCourseVersion, error)
}
type Repository interface {
	Create(context.Context, CreateInput) (CourseTranslation, error)
	Get(context.Context, TranslationID) (CourseTranslation, error)
	GetBySourceVersionAndLanguage(context.Context, courses.CourseVersionID, courses.LanguageTag) (CourseTranslation, error)
	UpdateTree(context.Context, TranslationID, int64, TranslationTree, time.Time) (CourseTranslation, error)
	Publish(context.Context, TranslationID, TranslationPublication, time.Time) (TranslationPublication, error)
	GetLatestPublication(context.Context, courses.CourseVersionID, courses.LanguageTag) (TranslationPublication, error)
	ListLanguages(context.Context, courses.CourseVersionID) ([]courses.LanguageTag, error)
}
type CreateInput struct {
	Source         SourceCourseVersion
	TargetLanguage courses.LanguageTag
	CreatorUserID  string
	Tree           TranslationTree
	CreatedAt      time.Time
}

type Service struct {
	source     SourceRepository
	repository Repository
	now        func() time.Time
}

func NewService(source SourceRepository, repository Repository, now func() time.Time) (*Service, error) {
	if source == nil || repository == nil {
		return nil, ErrInvalidTranslation
	}
	if now == nil {
		now = time.Now
	}
	return &Service{source: source, repository: repository, now: now}, nil
}

func (s *Service) Create(ctx context.Context, sourceID courses.CourseVersionID, target courses.LanguageTag, creator string) (CourseTranslation, error) {
	if s == nil || s.source == nil || s.repository == nil || !validUUID(creator) {
		return CourseTranslation{}, ErrInvalidTranslation
	}
	source, err := s.source.GetImmutableCourseVersion(ctx, sourceID)
	if err != nil {
		return CourseTranslation{}, err
	}
	binding, err := sourceFromImmutable(source)
	if err != nil || source.CourseVersion.Status != courses.CourseVersionPublished || binding.CourseVersionID != sourceID || target == "" || target == binding.Language {
		return CourseTranslation{}, ErrInvalidTranslation
	}
	normalized, err := courses.NormalizeLanguageTag(string(target))
	if err != nil || normalized != target {
		return CourseTranslation{}, ErrInvalidTranslation
	}
	tree, err := SeedTranslationTree(source)
	if err != nil {
		return CourseTranslation{}, err
	}
	return s.repository.Create(ctx, CreateInput{Source: binding, TargetLanguage: target, CreatorUserID: creator, Tree: tree, CreatedAt: s.now().UTC()})
}
func (s *Service) Update(ctx context.Context, id TranslationID, expected int64, tree TranslationTree) (CourseTranslation, error) {
	if s == nil || expected < 1 {
		return CourseTranslation{}, ErrInvalidTranslation
	}
	translation, err := s.repository.Get(ctx, id)
	if err != nil {
		return CourseTranslation{}, err
	}
	if translation.Revision != expected {
		return CourseTranslation{}, ErrRevisionMismatch
	}
	source, err := s.source.GetImmutableCourseVersion(ctx, translation.Source.CourseVersionID)
	if err != nil {
		return CourseTranslation{}, err
	}
	if binding, err := sourceFromImmutable(source); err != nil || binding != translation.Source || tree.ValidateAgainstSource(source) != nil {
		return CourseTranslation{}, ErrInvalidTranslation
	}
	return s.repository.UpdateTree(ctx, id, expected, tree, s.now().UTC())
}
func (s *Service) Publish(ctx context.Context, id TranslationID) (TranslationPublication, error) {
	translation, err := s.Get(ctx, id)
	if err != nil {
		return TranslationPublication{}, err
	}
	return s.PublishExpected(ctx, id, translation.Revision)
}
func (s *Service) PublishExpected(ctx context.Context, id TranslationID, expected int64) (TranslationPublication, error) {
	if s == nil || expected < 1 {
		return TranslationPublication{}, ErrInvalidTranslation
	}
	translation, err := s.repository.Get(ctx, id)
	if err != nil {
		return TranslationPublication{}, err
	}
	if translation.Revision != expected {
		return TranslationPublication{}, ErrRevisionMismatch
	}
	source, err := s.source.GetImmutableCourseVersion(ctx, translation.Source.CourseVersionID)
	if err != nil {
		return TranslationPublication{}, err
	}
	if binding, err := sourceFromImmutable(source); err != nil || binding != translation.Source || translation.Tree.ValidateAgainstSource(source) != nil {
		return TranslationPublication{}, ErrInvalidTranslation
	}
	tree, err := cloneTree(translation.Tree)
	if err != nil {
		return TranslationPublication{}, err
	}
	now := s.now().UTC()
	publication := TranslationPublication{TranslationID: id, Source: translation.Source, TargetLanguage: translation.TargetLanguage, Revision: translation.Revision, Tree: tree, PublishedAt: now}
	return s.repository.Publish(ctx, id, publication, now)
}
func (s *Service) Get(ctx context.Context, id TranslationID) (CourseTranslation, error) {
	if s == nil {
		return CourseTranslation{}, ErrInvalidTranslation
	}
	return s.repository.Get(ctx, id)
}
func (s *Service) GetBySourceVersionAndLanguage(ctx context.Context, id courses.CourseVersionID, language courses.LanguageTag) (CourseTranslation, error) {
	if s == nil {
		return CourseTranslation{}, ErrInvalidTranslation
	}
	return s.repository.GetBySourceVersionAndLanguage(ctx, id, language)
}

type sourceVersionReader interface {
	GetPublishedImmutableCourseVersionByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error)
}
type sourceTranslationLister interface {
	ListBySourceVersion(context.Context, courses.CourseVersionID) ([]CourseTranslation, error)
}

func (s *Service) PublishedSource(ctx context.Context, courseID courses.CourseID, version courses.Version) (courses.ImmutableCourseVersion, error) {
	if s == nil {
		return courses.ImmutableCourseVersion{}, ErrInvalidTranslation
	}
	reader, ok := s.source.(sourceVersionReader)
	if !ok {
		return courses.ImmutableCourseVersion{}, ErrInvalidTranslation
	}
	return reader.GetPublishedImmutableCourseVersionByCourseAndVersion(ctx, courseID, version)
}
func (s *Service) ListBySourceVersion(ctx context.Context, id courses.CourseVersionID) ([]CourseTranslation, error) {
	if s == nil {
		return nil, ErrInvalidTranslation
	}
	lister, ok := s.repository.(sourceTranslationLister)
	if !ok {
		return nil, ErrInvalidTranslation
	}
	return lister.ListBySourceVersion(ctx, id)
}
func (s *Service) LatestPublication(ctx context.Context, id courses.CourseVersionID, language courses.LanguageTag) (TranslationPublication, error) {
	if s == nil {
		return TranslationPublication{}, ErrInvalidTranslation
	}
	return s.repository.GetLatestPublication(ctx, id, language)
}
func (s *Service) Languages(ctx context.Context, id courses.CourseVersionID) ([]courses.LanguageTag, error) {
	if s == nil {
		return nil, ErrInvalidTranslation
	}
	return s.repository.ListLanguages(ctx, id)
}

func IsNotFound(err error) bool { return errors.Is(err, ErrTranslationNotFound) }
