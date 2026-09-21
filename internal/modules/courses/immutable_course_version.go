package courses

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidImmutableCourseVersion = errors.New("invalid immutable course version")

// CourseVersionProvenance identifies the immutable Authoring evidence from
// which a CourseVersion was constructed. Courses persists this evidence with
// the version so later reads never need to consult Authoring.
type CourseVersionProvenance struct {
	ReviewID              string
	ReviewRevision        int64
	DraftID               string
	DraftRevision         int64
	SnapshotSchemaVersion int
	SubmittedByUserID     string
	SubmittedAt           time.Time
	ApprovedByUserID      string
	ApprovedAt            *time.Time
	PublishedByUserID     string
}

// ImmutableCourseVersion is the complete Courses-domain publication value.
// Persistence inputs leave generated IDs empty; stored reconstructions populate
// them. Modules, Lessons, prerequisites, and canonical content remain in frozen
// snapshot order.
type ImmutableCourseVersion struct {
	ID                 CourseVersionID
	CourseVersion      CourseVersionInput
	Provenance         CourseVersionProvenance
	Modules            []ImmutableCourseVersionModule
	AssetBindings      []PublishedAssetBinding      `json:"-"`
	AssessmentBindings []PublishedAssessmentBinding `json:"-"`
}

type ImmutableCourseVersionModule struct {
	ID          ModuleID
	SourceID    string
	StableKey   string
	Title       string
	Description string
	Position    int
	Lessons     []ImmutableCourseVersionLesson
}

type ImmutableCourseVersionLesson struct {
	ID                       LessonID
	SourceID                 string
	StableKey                string
	Title                    string
	Description              string
	LearningObjectives       []string
	EstimatedDurationMinutes *int
	Position                 int
	PrerequisiteStableKeys   []string
	Content                  LessonContent
}

// ValidateForPersistence enforces Courses-owned aggregate invariants without
// repeating Authoring publication validation. Database identifiers must still
// be empty because the atomic writer owns their generation.
func (v ImmutableCourseVersion) ValidateForPersistence() error {
	if v.ID != "" || v.CourseVersion.Status != CourseVersionPublished || v.CourseVersion.Validate() != nil || v.Modules == nil {
		return ErrInvalidImmutableCourseVersion
	}
	if !uuidPattern.MatchString(v.Provenance.ReviewID) || v.Provenance.ReviewRevision < 1 ||
		!uuidPattern.MatchString(v.Provenance.DraftID) || v.Provenance.DraftRevision < 1 || v.Provenance.SnapshotSchemaVersion < 1 ||
		!uuidPattern.MatchString(v.Provenance.SubmittedByUserID) || v.Provenance.SubmittedAt.IsZero() ||
		!uuidPattern.MatchString(v.Provenance.ApprovedByUserID) || v.Provenance.ApprovedAt == nil || v.Provenance.ApprovedAt.IsZero() ||
		!uuidPattern.MatchString(v.Provenance.PublishedByUserID) {
		return ErrInvalidImmutableCourseVersion
	}

	moduleKeys := make(map[string]struct{}, len(v.Modules))
	lessonKeys := make(map[string]struct{})
	referencedAssets := make([]contentAssetReference, 0)
	referencedAssessments := make([]string, 0)
	for moduleIndex, module := range v.Modules {
		if module.ID != "" || !uuidPattern.MatchString(module.SourceID) || module.Position != moduleIndex || module.Lessons == nil {
			return ErrInvalidImmutableCourseVersion
		}
		if _, exists := moduleKeys[module.StableKey]; exists {
			return ErrInvalidImmutableCourseVersion
		}
		moduleKeys[module.StableKey] = struct{}{}
		if (ModuleInput{CourseVersionID: "publication-candidate", StableKey: module.StableKey, Title: module.Title, Description: module.Description, Position: module.Position}).Validate() != nil {
			return ErrInvalidImmutableCourseVersion
		}
		for lessonIndex, lesson := range module.Lessons {
			if lesson.ID != "" || !uuidPattern.MatchString(lesson.SourceID) || lesson.Position != lessonIndex || lesson.PrerequisiteStableKeys == nil {
				return ErrInvalidImmutableCourseVersion
			}
			if _, exists := lessonKeys[lesson.StableKey]; exists {
				return ErrInvalidImmutableCourseVersion
			}
			lessonKeys[lesson.StableKey] = struct{}{}
			if (LessonInput{
				CourseVersionID:          "publication-candidate",
				ModuleID:                 "publication-module",
				StableKey:                lesson.StableKey,
				Title:                    lesson.Title,
				Description:              lesson.Description,
				LearningObjectives:       lesson.LearningObjectives,
				EstimatedDurationMinutes: lesson.EstimatedDurationMinutes,
				Position:                 lesson.Position,
				Content:                  lesson.Content,
			}).Validate() != nil {
				return ErrInvalidImmutableCourseVersion
			}
			referencedAssets = append(referencedAssets, contentAssetReferences(lesson.Content)...)
			referencedAssessments = append(referencedAssessments, contentAssessmentReferences(lesson.Content)...)
		}
	}

	bindings := make(map[string]PublishedAssetBinding, len(v.AssetBindings))
	previousAssetKey := ""
	for _, binding := range v.AssetBindings {
		if binding.Validate() != nil {
			return ErrInvalidImmutableCourseVersion
		}
		if previousAssetKey != "" && binding.AssetKey <= previousAssetKey {
			return ErrInvalidImmutableCourseVersion
		}
		if _, duplicate := bindings[binding.AssetKey]; duplicate {
			return ErrInvalidImmutableCourseVersion
		}
		referenced := false
		for _, reference := range referencedAssets {
			if reference.key == binding.AssetKey {
				referenced = true
				break
			}
		}
		if !referenced {
			return ErrInvalidImmutableCourseVersion
		}
		bindings[binding.AssetKey] = binding
		previousAssetKey = binding.AssetKey
	}
	for _, reference := range referencedAssets {
		binding, bound := bindings[reference.key]
		if !bound || (reference.mediaPrefix != "" && !strings.HasPrefix(binding.MediaType, reference.mediaPrefix)) {
			return ErrInvalidImmutableCourseVersion
		}
	}

	assessmentBindings := make(map[string]PublishedAssessmentBinding, len(v.AssessmentBindings))
	previousAssessmentKey := ""
	for _, binding := range v.AssessmentBindings {
		if binding.Validate() != nil || previousAssessmentKey != "" && binding.AssessmentKey <= previousAssessmentKey {
			return ErrInvalidImmutableCourseVersion
		}
		if _, duplicate := assessmentBindings[binding.AssessmentKey]; duplicate {
			return ErrInvalidImmutableCourseVersion
		}
		referenced := false
		for _, key := range referencedAssessments {
			if key == binding.AssessmentKey {
				referenced = true
				break
			}
		}
		if !referenced {
			return ErrInvalidImmutableCourseVersion
		}
		assessmentBindings[binding.AssessmentKey] = binding
		previousAssessmentKey = binding.AssessmentKey
	}
	for _, key := range referencedAssessments {
		if _, bound := assessmentBindings[key]; !bound {
			return ErrInvalidImmutableCourseVersion
		}
	}

	for _, module := range v.Modules {
		for _, lesson := range module.Lessons {
			seen := make(map[string]struct{}, len(lesson.PrerequisiteStableKeys))
			for _, prerequisite := range lesson.PrerequisiteStableKeys {
				if prerequisite == lesson.StableKey {
					return ErrInvalidImmutableCourseVersion
				}
				if _, exists := lessonKeys[prerequisite]; !exists {
					return ErrInvalidImmutableCourseVersion
				}
				if _, exists := seen[prerequisite]; exists {
					return ErrInvalidImmutableCourseVersion
				}
				seen[prerequisite] = struct{}{}
			}
		}
	}
	return nil
}
