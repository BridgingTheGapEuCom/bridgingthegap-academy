package courses

import (
	"context"
	"errors"
	"time"
)

// PublishedCourseVersionRepository reads complete immutable publication
// aggregates from Courses-owned storage. It deliberately has no Authoring
// dependency: published content is reconstructed only from Courses records.
type PublishedCourseVersionRepository interface {
	GetPublishedImmutableCourseVersionByCourseAndVersion(context.Context, CourseID, Version) (ImmutableCourseVersion, error)
	GetLatestPublishedImmutableCourseVersion(context.Context, CourseID) (ImmutableCourseVersion, error)
}

// PublishedAssetBindingRepository resolves one frozen binary binding through
// its public CourseVersion coordinates. It intentionally does not expose an
// Asset repository: learner delivery must never consult mutable Authoring
// metadata.
type PublishedAssetBindingRepository interface {
	GetPublishedAssetBindingByCourseAndVersionAndAssetKey(context.Context, CourseID, Version, string) (PublishedAssetBinding, error)
}

// PublishedCourseVersion is the learner-facing immutable read model. It
// intentionally excludes Review, Draft, snapshot, and storage provenance.
type PublishedCourseVersion struct {
	CourseID           CourseID
	Version            Version
	Title              string
	Description        string
	LearningObjectives []string
	SourceLanguage     LanguageTag
	Changelog          string
	License            ContentLicense
	Contributors       []PublishedContributor
	PublishedAt        time.Time
	Modules            []PublishedCourseModule
	Assessments        []PublishedAssessmentLearnerView
}

// PublishedAssessmentLearnerView is the public, learner-safe representation
// of a frozen assessment binding. It deliberately has no answer definitions:
// grading remains server-side in a later slice.
type PublishedAssessmentLearnerView struct {
	AssessmentKey string
	Questions     []PublishedAssessmentLearnerQuestion
}

type PublishedAssessmentLearnerQuestion struct {
	StableKey  string
	Type       PublishedAssessmentQuestionType
	Prompt     string
	Position   int
	Options    []PublishedAssessmentLearnerOption
	LeftItems  []PublishedAssessmentLearnerItem
	RightItems []PublishedAssessmentLearnerItem
}

type PublishedAssessmentLearnerOption struct {
	StableKey string
	Text      string
	Position  int
}

type PublishedAssessmentLearnerItem struct {
	StableKey string
	Text      string
	Position  int
}

// PublishedContributor preserves public attribution without exposing an
// optional Identity-linked contributor identifier.
type PublishedContributor struct {
	DisplayName string
	Role        ContributorRole
	Order       int
}

type PublishedCourseModule struct {
	StableKey   string
	Title       string
	Description string
	Position    int
	Lessons     []PublishedCourseLesson
}

type PublishedCourseLesson struct {
	StableKey                string
	Title                    string
	Description              string
	LearningObjectives       []string
	EstimatedDurationMinutes *int
	Position                 int
	PrerequisiteStableKeys   []string
	Content                  LessonContent
}

// PublishedReadService retrieves only PUBLISHED immutable course versions.
// Exact lookup never falls back to another version; Latest selects the highest
// SemVer value in the Courses repository.
type PublishedReadService struct {
	repository PublishedCourseVersionRepository
}

// PublishedAssetReadService is the narrow Courses-owned boundary for public
// binary delivery. A successful lookup proves that the asset belongs to the
// requested exact PUBLISHED CourseVersion.
type PublishedAssetReadService struct {
	repository PublishedAssetBindingRepository
}

func NewPublishedAssetReadService(repository PublishedAssetBindingRepository) *PublishedAssetReadService {
	return &PublishedAssetReadService{repository: repository}
}

func (s *PublishedAssetReadService) Exact(ctx context.Context, courseID CourseID, version Version, assetKey string) (PublishedAssetBinding, error) {
	if s == nil || s.repository == nil || !uuidPattern.MatchString(string(courseID)) || !version.Valid() || !uuidPattern.MatchString(assetKey) {
		return PublishedAssetBinding{}, ErrNotFound
	}
	binding, err := s.repository.GetPublishedAssetBindingByCourseAndVersionAndAssetKey(ctx, courseID, version, assetKey)
	if err != nil {
		return PublishedAssetBinding{}, err
	}
	if binding.AssetKey != assetKey || binding.Validate() != nil {
		return PublishedAssetBinding{}, ErrInvalidImmutableCourseVersion
	}
	return binding, nil
}

func NewPublishedReadService(repository PublishedCourseVersionRepository) *PublishedReadService {
	return &PublishedReadService{repository: repository}
}

func (s *PublishedReadService) Exact(ctx context.Context, courseID CourseID, version Version) (PublishedCourseVersion, error) {
	if s == nil || s.repository == nil || courseID == "" || !version.Valid() {
		return PublishedCourseVersion{}, ErrNotFound
	}
	aggregate, err := s.repository.GetPublishedImmutableCourseVersionByCourseAndVersion(ctx, courseID, version)
	if err != nil {
		return PublishedCourseVersion{}, err
	}
	return publicPublishedCourseVersion(aggregate, courseID, version)
}

func (s *PublishedReadService) Latest(ctx context.Context, courseID CourseID) (PublishedCourseVersion, error) {
	if s == nil || s.repository == nil || courseID == "" {
		return PublishedCourseVersion{}, ErrNotFound
	}
	aggregate, err := s.repository.GetLatestPublishedImmutableCourseVersion(ctx, courseID)
	if err != nil {
		return PublishedCourseVersion{}, err
	}
	return publicPublishedCourseVersion(aggregate, courseID, aggregate.CourseVersion.Version)
}

func publicPublishedCourseVersion(aggregate ImmutableCourseVersion, courseID CourseID, expected Version) (PublishedCourseVersion, error) {
	metadata := aggregate.CourseVersion
	if aggregate.ID == "" || metadata.Status != CourseVersionPublished || metadata.CourseID != courseID || metadata.Version != expected {
		return PublishedCourseVersion{}, ErrNotFound
	}

	result := PublishedCourseVersion{
		CourseID:           metadata.CourseID,
		Version:            metadata.Version,
		Title:              metadata.Title,
		Description:        metadata.Description,
		LearningObjectives: append([]string(nil), metadata.LearningObjectives...),
		SourceLanguage:     metadata.SourceLanguage,
		Changelog:          metadata.Changelog,
		License:            metadata.License,
		Contributors:       make([]PublishedContributor, 0, len(metadata.Attribution)),
		PublishedAt:        metadata.PublishedAt.UTC(),
		Modules:            make([]PublishedCourseModule, 0, len(aggregate.Modules)),
		Assessments:        make([]PublishedAssessmentLearnerView, 0, len(aggregate.AssessmentBindings)),
	}
	for _, contributor := range metadata.Attribution {
		result.Contributors = append(result.Contributors, PublishedContributor{
			DisplayName: contributor.DisplayName, Role: contributor.Role, Order: contributor.Order,
		})
	}
	for _, module := range aggregate.Modules {
		publicModule := PublishedCourseModule{
			StableKey: module.StableKey, Title: module.Title, Description: module.Description,
			Position: module.Position, Lessons: make([]PublishedCourseLesson, 0, len(module.Lessons)),
		}
		for _, lesson := range module.Lessons {
			content, err := clonePublishedLessonContent(lesson.Content)
			if err != nil {
				return PublishedCourseVersion{}, errors.New("invalid stored published lesson content")
			}
			publicLesson := PublishedCourseLesson{
				StableKey: lesson.StableKey, Title: lesson.Title, Description: lesson.Description,
				LearningObjectives:     append([]string(nil), lesson.LearningObjectives...),
				Position:               lesson.Position,
				PrerequisiteStableKeys: append([]string(nil), lesson.PrerequisiteStableKeys...),
				Content:                content,
			}
			if lesson.EstimatedDurationMinutes != nil {
				duration := *lesson.EstimatedDurationMinutes
				publicLesson.EstimatedDurationMinutes = &duration
			}
			publicModule.Lessons = append(publicModule.Lessons, publicLesson)
		}
		result.Modules = append(result.Modules, publicModule)
	}
	for _, binding := range aggregate.AssessmentBindings {
		learnerView, err := publishedAssessmentLearnerView(binding)
		if err != nil {
			return PublishedCourseVersion{}, errors.New("invalid stored published assessment")
		}
		result.Assessments = append(result.Assessments, learnerView)
	}
	return result, nil
}

// publishedAssessmentLearnerView copies only the presentation fields from a
// Courses-owned immutable binding. Correct options and matching pairs remain
// deliberately absent from the public read model.
func publishedAssessmentLearnerView(binding PublishedAssessmentBinding) (PublishedAssessmentLearnerView, error) {
	if err := binding.Validate(); err != nil {
		return PublishedAssessmentLearnerView{}, err
	}
	result := PublishedAssessmentLearnerView{
		AssessmentKey: binding.AssessmentKey,
		Questions:     make([]PublishedAssessmentLearnerQuestion, 0, len(binding.Questions)),
	}
	for _, question := range binding.Questions {
		learnerQuestion := PublishedAssessmentLearnerQuestion{
			StableKey:  question.StableKey,
			Type:       question.Type,
			Prompt:     question.Prompt,
			Position:   question.Position,
			Options:    make([]PublishedAssessmentLearnerOption, 0, len(question.Options)),
			LeftItems:  make([]PublishedAssessmentLearnerItem, 0, len(question.LeftItems)),
			RightItems: make([]PublishedAssessmentLearnerItem, 0, len(question.RightItems)),
		}
		for _, option := range question.Options {
			learnerQuestion.Options = append(learnerQuestion.Options, PublishedAssessmentLearnerOption(option))
		}
		for _, item := range question.LeftItems {
			learnerQuestion.LeftItems = append(learnerQuestion.LeftItems, PublishedAssessmentLearnerItem(item))
		}
		for _, item := range question.RightItems {
			learnerQuestion.RightItems = append(learnerQuestion.RightItems, PublishedAssessmentLearnerItem(item))
		}
		result.Questions = append(result.Questions, learnerQuestion)
	}
	return result, nil
}

func clonePublishedLessonContent(content LessonContent) (LessonContent, error) {
	encoded, err := MarshalLessonContent(content)
	if err != nil {
		return LessonContent{}, err
	}
	return ParseLessonContent(encoded)
}
