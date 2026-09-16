package authoring

import (
	"errors"
	"fmt"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

var (
	ErrPublicationValidationFailed = errors.New("publication validation failed")
	ErrPublicationConversionInput  = errors.New("invalid publication conversion input")
)

// PublicationConversionMetadata contains immutable values required by the
// Courses model that are deliberately not invented by the frozen Review
// snapshot. The publishing orchestrator will provide them explicitly.
type PublicationConversionMetadata struct {
	PublishedAt       time.Time
	PublishedByUserID string
	Attribution       []courses.ContributorSnapshot
}

// PublicationValidationFailure preserves every deterministic M4.5a issue for
// later application/API presentation while returning no partial CourseVersion.
type PublicationValidationFailure struct {
	Result PublicationValidationResult
}

func (e *PublicationValidationFailure) Error() string {
	return fmt.Sprintf("%s: %d issue(s)", ErrPublicationValidationFailed, len(e.Result.Issues))
}

func (e *PublicationValidationFailure) Unwrap() error { return ErrPublicationValidationFailed }

// PublicationConverter is a pure Review snapshot -> Courses domain boundary.
// It has no repositories and cannot consult mutable Draft state.
type PublicationConverter struct {
	validator PublicationValidator
}

func NewPublicationConverter() PublicationConverter {
	return PublicationConverter{validator: NewPublicationValidator()}
}

func (c PublicationConverter) Convert(cycle ReviewCycle, snapshot *ReviewSnapshot, metadata PublicationConversionMetadata) (courses.ImmutableCourseVersion, error) {
	validation := c.validator.Validate(cycle, snapshot)
	if !validation.Publishable {
		return courses.ImmutableCourseVersion{}, &PublicationValidationFailure{Result: validation}
	}
	result, err := buildImmutableCourseVersion(cycle, *snapshot, metadata)
	if err != nil {
		return courses.ImmutableCourseVersion{}, fmt.Errorf("%w: %v", ErrPublicationConversionInput, err)
	}
	return result, nil
}

func buildImmutableCourseVersion(cycle ReviewCycle, snapshot ReviewSnapshot, metadata PublicationConversionMetadata) (courses.ImmutableCourseVersion, error) {
	version, err := courses.ParseVersion(snapshot.Draft.IntendedVersion)
	if err != nil {
		return courses.ImmutableCourseVersion{}, err
	}
	versionInput := courses.CourseVersionInput{
		CourseID:           snapshot.Draft.CourseID,
		Version:            version,
		Status:             courses.CourseVersionPublished,
		Title:              snapshot.Draft.Title,
		Description:        snapshot.Draft.Description,
		LearningObjectives: append([]string{}, snapshot.Draft.Objectives...),
		SourceLanguage:     snapshot.Draft.SourceLanguage,
		Changelog:          snapshot.Draft.Changelog,
		License:            snapshot.Draft.License,
		Attribution:        append([]courses.ContributorSnapshot{}, metadata.Attribution...),
		PublishedAt:        metadata.PublishedAt,
	}
	if err := versionInput.Validate(); err != nil {
		return courses.ImmutableCourseVersion{}, err
	}

	result := courses.ImmutableCourseVersion{
		CourseVersion: versionInput,
		Provenance: courses.CourseVersionProvenance{
			ReviewID:              string(cycle.ID),
			ReviewRevision:        cycle.Revision,
			DraftID:               string(cycle.DraftID),
			DraftRevision:         cycle.DraftRevision,
			SnapshotSchemaVersion: snapshot.SchemaVersion,
			SubmittedByUserID:     cycle.SubmittedByUserID,
			SubmittedAt:           cycle.SubmittedAt,
			ApprovedByUserID:      cycle.DecidedByUserID,
			ApprovedAt:            cloneTime(cycle.DecidedAt),
			PublishedByUserID:     metadata.PublishedByUserID,
		},
		Modules: make([]courses.ImmutableCourseVersionModule, 0, len(snapshot.Modules)),
	}
	for _, module := range snapshot.Modules {
		convertedModule := courses.ImmutableCourseVersionModule{
			SourceID:    string(module.ID),
			StableKey:   module.StableKey,
			Title:       module.Title,
			Description: module.Description,
			Position:    module.Position,
			Lessons:     make([]courses.ImmutableCourseVersionLesson, 0, len(module.Lessons)),
		}
		for _, lesson := range module.Lessons {
			content, err := clonePublicationContent(lesson.Content)
			if err != nil {
				return courses.ImmutableCourseVersion{}, err
			}
			convertedModule.Lessons = append(convertedModule.Lessons, courses.ImmutableCourseVersionLesson{
				SourceID:                 string(lesson.ID),
				StableKey:                lesson.StableKey,
				Title:                    lesson.Title,
				Description:              lesson.Description,
				LearningObjectives:       append([]string{}, lesson.Objectives...),
				EstimatedDurationMinutes: cloneInt(lesson.EstimatedDurationMinutes),
				Position:                 lesson.Position,
				PrerequisiteStableKeys:   append([]string{}, lesson.PrerequisiteStableKeys...),
				Content:                  content,
			})
		}
		result.Modules = append(result.Modules, convertedModule)
	}
	return result, nil
}

func clonePublicationContent(content courses.LessonContent) (courses.LessonContent, error) {
	encoded, err := courses.MarshalLessonContent(content)
	if err != nil {
		return courses.LessonContent{}, err
	}
	return courses.ParseLessonContent(encoded)
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
