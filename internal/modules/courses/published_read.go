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
	return result, nil
}

func clonePublishedLessonContent(content LessonContent) (LessonContent, error) {
	encoded, err := MarshalLessonContent(content)
	if err != nil {
		return LessonContent{}, err
	}
	return ParseLessonContent(encoded)
}
