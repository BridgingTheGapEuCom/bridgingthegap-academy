package authoring

import (
	"context"
	"fmt"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

// PublicationValidationCode is a stable, API-safe reason that a frozen Review
// snapshot cannot yet become a published CourseVersion.
type PublicationValidationCode string

const (
	PublicationIssueReviewMissing              PublicationValidationCode = "review_missing"
	PublicationIssueReviewNotApproved          PublicationValidationCode = "review_not_approved"
	PublicationIssueMissingSnapshot            PublicationValidationCode = "missing_snapshot"
	PublicationIssueUnsupportedSnapshotVersion PublicationValidationCode = "unsupported_snapshot_version"
	PublicationIssueSnapshotReviewMismatch     PublicationValidationCode = "snapshot_review_mismatch"
	PublicationIssueCourseReferenceInvalid     PublicationValidationCode = "course_reference_invalid"
	PublicationIssueIntendedVersionInvalid     PublicationValidationCode = "intended_version_invalid"
	PublicationIssueSourceLanguageInvalid      PublicationValidationCode = "source_language_invalid"
	PublicationIssueCourseMetadataInvalid      PublicationValidationCode = "course_metadata_invalid"
	PublicationIssueContentLicenseInvalid      PublicationValidationCode = "content_license_invalid"
	PublicationIssueStructureInvalid           PublicationValidationCode = "structure_invalid"
	PublicationIssueModuleIDDuplicate          PublicationValidationCode = "duplicate_module_id"
	PublicationIssueModuleKeyDuplicate         PublicationValidationCode = "duplicate_module_stable_key"
	PublicationIssueModulePositionInvalid      PublicationValidationCode = "invalid_module_position"
	PublicationIssueModuleMetadataInvalid      PublicationValidationCode = "invalid_module_metadata"
	PublicationIssueLessonIDDuplicate          PublicationValidationCode = "duplicate_lesson_id"
	PublicationIssueLessonKeyDuplicate         PublicationValidationCode = "duplicate_lesson_stable_key"
	PublicationIssueLessonPositionInvalid      PublicationValidationCode = "invalid_lesson_position"
	PublicationIssueLessonMetadataInvalid      PublicationValidationCode = "invalid_lesson_metadata"
	PublicationIssuePrerequisiteUnknown        PublicationValidationCode = "unknown_prerequisite"
	PublicationIssuePrerequisiteSelf           PublicationValidationCode = "self_prerequisite"
	PublicationIssuePrerequisiteDuplicate      PublicationValidationCode = "duplicate_prerequisite"
	PublicationIssueContentSchemaUnsupported   PublicationValidationCode = "unsupported_content_schema_version"
	PublicationIssueLessonContentInvalid       PublicationValidationCode = "invalid_lesson_content"
	PublicationIssueContentBlockKeyDuplicate   PublicationValidationCode = "duplicate_content_block_key"
	PublicationIssueContentBlockKeyInvalid     PublicationValidationCode = "invalid_content_block_key"
	PublicationIssueContentBlockInvalid        PublicationValidationCode = "invalid_content_block"
	PublicationIssueHeadingInvalid             PublicationValidationCode = "invalid_heading"
	PublicationIssueImageAccessibilityInvalid  PublicationValidationCode = "invalid_image_accessibility"
	PublicationIssueMediaAccessibilityInvalid  PublicationValidationCode = "invalid_media_accessibility"
	PublicationIssueTableInvalid               PublicationValidationCode = "invalid_table"
	PublicationIssueAssetUnresolved            PublicationValidationCode = "unresolved_asset_reference"
	PublicationIssueAssessmentUnresolved       PublicationValidationCode = "unresolved_assessment_reference"
)

// PublicationValidationIssue always blocks publication in this milestone.
// Paths name canonical snapshot locations and intentionally never use mutable
// Draft state or database row lookup.
type PublicationValidationIssue struct {
	Code    PublicationValidationCode
	Path    string
	Message string
}

type PublicationValidationResult struct {
	Publishable bool
	Issues      []PublicationValidationIssue
}

// PublicationValidator validates only a persisted Review cycle and its frozen
// canonical snapshot. It has no repository or persistence dependency, so the
// same input always yields the same ordered result.
type PublicationValidator struct{}

func NewPublicationValidator() PublicationValidator { return PublicationValidator{} }

func (PublicationValidator) Validate(cycle ReviewCycle, snapshot *ReviewSnapshot) PublicationValidationResult {
	return validatePublication(cycle, snapshot, nil, nil)
}

// ValidateResolved keeps canonical validation pure while allowing the
// application resolver to attest which asset keys were authoritatively bound.
func (PublicationValidator) ValidateResolved(cycle ReviewCycle, snapshot *ReviewSnapshot, bindings []courses.PublishedAssetBinding) PublicationValidationResult {
	return NewPublicationValidator().ValidateResolvedBindings(cycle, snapshot, bindings, nil)
}

func (PublicationValidator) ValidateResolvedBindings(cycle ReviewCycle, snapshot *ReviewSnapshot, assetBindings []courses.PublishedAssetBinding, assessmentBindings []courses.PublishedAssessmentBinding) PublicationValidationResult {
	resolved := make(map[string]struct{}, len(assetBindings))
	for _, binding := range assetBindings {
		resolved[binding.AssetKey] = struct{}{}
	}
	resolvedAssessments := make(map[string]struct{}, len(assessmentBindings))
	for _, binding := range assessmentBindings {
		resolvedAssessments[binding.AssessmentKey] = struct{}{}
	}
	return validatePublication(cycle, snapshot, resolved, resolvedAssessments)
}

func validatePublication(cycle ReviewCycle, snapshot *ReviewSnapshot, resolvedAssets, resolvedAssessments map[string]struct{}) PublicationValidationResult {
	issues := make([]PublicationValidationIssue, 0)
	add := func(code PublicationValidationCode, path, message string) {
		issues = append(issues, PublicationValidationIssue{Code: code, Path: path, Message: message})
	}

	if cycle.ID == "" {
		add(PublicationIssueReviewMissing, "review", "A Review cycle is required for publication.")
	}
	if cycle.Status != ReviewApproved {
		add(PublicationIssueReviewNotApproved, "review.status", "Only an approved Review can be published.")
	}
	if snapshot == nil || snapshotMissing(snapshot) {
		add(PublicationIssueMissingSnapshot, "snapshot", "The approved Review has no frozen snapshot.")
		return publicationValidationResult(issues)
	}
	if snapshot.SchemaVersion != 1 {
		add(PublicationIssueUnsupportedSnapshotVersion, "snapshot.schemaVersion", "This Review snapshot schema version is not supported for publication.")
		return publicationValidationResult(issues)
	}
	if cycle.SnapshotSchemaVersion != 1 {
		add(PublicationIssueUnsupportedSnapshotVersion, "review.snapshotSchemaVersion", "This Review records an unsupported snapshot schema version.")
	}
	if cycle.SnapshotSchemaVersion != snapshot.SchemaVersion || cycle.DraftID != snapshot.Draft.ID || cycle.DraftRevision != snapshot.Draft.Revision {
		add(PublicationIssueSnapshotReviewMismatch, "snapshot", "The Review cycle and frozen snapshot do not describe the same Draft revision.")
	}

	validatePublicationMetadata(*snapshot, add)
	validatePublicationStructure(*snapshot, resolvedAssets, resolvedAssessments, add)
	return publicationValidationResult(issues)
}

func snapshotMissing(snapshot *ReviewSnapshot) bool {
	return snapshot.SchemaVersion == 0 && snapshot.Draft.ID == "" && snapshot.Modules == nil
}

func publicationValidationResult(issues []PublicationValidationIssue) PublicationValidationResult {
	return PublicationValidationResult{Publishable: len(issues) == 0, Issues: issues}
}

func validatePublicationMetadata(snapshot ReviewSnapshot, add func(PublicationValidationCode, string, string)) {
	draft := snapshot.Draft
	metadataCourseID := draft.CourseID
	if draft.CourseID == "" {
		add(PublicationIssueCourseReferenceInvalid, "course.id", "A Course reference is required for publication.")
		metadataCourseID = "frozen-review"
	}
	version, versionErr := courses.ParseVersion(draft.IntendedVersion)
	if versionErr != nil {
		add(PublicationIssueIntendedVersionInvalid, "course.intendedVersion", "The intended version must use the supported semantic-version format.")
		version = courses.Version{}
	}
	metadataLanguage := draft.SourceLanguage
	if _, err := courses.NormalizeLanguageTag(string(draft.SourceLanguage)); err != nil || draft.SourceLanguage == "" {
		add(PublicationIssueSourceLanguageInvalid, "course.sourceLanguage", "The source language is invalid.")
		metadataLanguage = "en"
	}
	metadataLicense := draft.License
	if err := draft.License.Validate(); err != nil {
		add(PublicationIssueContentLicenseInvalid, "course.license", "The content license is invalid.")
		metadataLicense = courses.ContentLicense{Kind: courses.ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"}
	}

	// Reuse the Courses metadata value object for title, description,
	// objectives, changelog, and any other shared published-artifact rules.
	if err := (courses.CourseVersionMetadata{
		CourseID:           metadataCourseID,
		Version:            version,
		Title:              draft.Title,
		Description:        draft.Description,
		LearningObjectives: draft.Objectives,
		SourceLanguage:     metadataLanguage,
		Changelog:          draft.Changelog,
		License:            metadataLicense,
	}).Validate(); err != nil {
		add(PublicationIssueCourseMetadataInvalid, "course", "Course metadata does not meet published CourseVersion requirements.")
	}
}

func validatePublicationStructure(snapshot ReviewSnapshot, resolvedAssets, resolvedAssessments map[string]struct{}, add func(PublicationValidationCode, string, string)) {
	if snapshot.Modules == nil {
		add(PublicationIssueStructureInvalid, "modules", "The snapshot structure is missing its Modules collection.")
		return
	}

	moduleIDs := make(map[ModuleID]struct{}, len(snapshot.Modules))
	moduleKeys := make(map[string]struct{}, len(snapshot.Modules))
	lessonIDs := make(map[LessonID]struct{})
	lessonKeys := make(map[string]struct{})

	for moduleIndex, module := range snapshot.Modules {
		modulePath := fmt.Sprintf("modules[%d]", moduleIndex)
		if module.Position != moduleIndex {
			add(PublicationIssueModulePositionInvalid, modulePath+".position", "Module positions must be contiguous and zero-based.")
		}
		if module.ID == "" {
			add(PublicationIssueStructureInvalid, modulePath+".id", "A Module identifier is required in the frozen snapshot.")
		} else if _, found := moduleIDs[module.ID]; found {
			add(PublicationIssueModuleIDDuplicate, modulePath+".id", "Module identifiers must be unique in the frozen snapshot.")
		} else {
			moduleIDs[module.ID] = struct{}{}
		}
		if _, found := moduleKeys[module.StableKey]; found {
			add(PublicationIssueModuleKeyDuplicate, modulePath+".stableKey", "Module stable keys must be unique.")
		} else {
			moduleKeys[module.StableKey] = struct{}{}
		}
		if err := (courses.ModuleInput{
			CourseVersionID: "frozen-review",
			StableKey:       module.StableKey,
			Title:           module.Title,
			Description:     module.Description,
			Position:        module.Position,
		}).Validate(); err != nil {
			add(PublicationIssueModuleMetadataInvalid, modulePath, "Module metadata does not meet published CourseVersion requirements.")
		}
		if module.Lessons == nil {
			add(PublicationIssueStructureInvalid, modulePath+".lessons", "The Module Lessons collection is missing.")
			continue
		}

		for lessonIndex, lesson := range module.Lessons {
			lessonPath := fmt.Sprintf("%s.lessons[%d]", modulePath, lessonIndex)
			if lesson.Position != lessonIndex {
				add(PublicationIssueLessonPositionInvalid, lessonPath+".position", "Lesson positions must be contiguous and zero-based within their Module.")
			}
			if lesson.ID == "" {
				add(PublicationIssueStructureInvalid, lessonPath+".id", "A Lesson identifier is required in the frozen snapshot.")
			} else if _, found := lessonIDs[lesson.ID]; found {
				add(PublicationIssueLessonIDDuplicate, lessonPath+".id", "Lesson identifiers must be unique in the frozen snapshot.")
			} else {
				lessonIDs[lesson.ID] = struct{}{}
			}
			if _, found := lessonKeys[lesson.StableKey]; found {
				add(PublicationIssueLessonKeyDuplicate, lessonPath+".stableKey", "Lesson stable keys must be unique across the CourseVersion.")
			} else {
				lessonKeys[lesson.StableKey] = struct{}{}
			}
			if err := (courses.LessonInput{
				CourseVersionID:          "frozen-review",
				ModuleID:                 courses.ModuleID(module.ID),
				StableKey:                lesson.StableKey,
				Title:                    lesson.Title,
				Description:              lesson.Description,
				LearningObjectives:       lesson.Objectives,
				EstimatedDurationMinutes: lesson.EstimatedDurationMinutes,
				Position:                 lesson.Position,
			}).ValidateMetadata(); err != nil {
				add(PublicationIssueLessonMetadataInvalid, lessonPath, "Lesson metadata does not meet published CourseVersion requirements.")
			}
			validatePublicationLessonContent(lesson.Content, lessonPath+".content", resolvedAssets, resolvedAssessments, add)
		}
	}

	for moduleIndex, module := range snapshot.Modules {
		for lessonIndex, lesson := range module.Lessons {
			lessonPath := fmt.Sprintf("modules[%d].lessons[%d]", moduleIndex, lessonIndex)
			validatePublicationPrerequisites(lesson.StableKey, lesson.PrerequisiteStableKeys, lessonKeys, lessonPath+".prerequisites", add)
		}
	}
}

func validatePublicationPrerequisites(self string, keys []string, lessonKeys map[string]struct{}, path string, add func(PublicationValidationCode, string, string)) {
	if keys == nil {
		add(PublicationIssueStructureInvalid, path, "The Lesson prerequisite collection is missing.")
		return
	}
	seen := make(map[string]struct{}, len(keys))
	for index, key := range keys {
		keyPath := fmt.Sprintf("%s[%d]", path, index)
		if key == self {
			add(PublicationIssuePrerequisiteSelf, keyPath, "A Lesson cannot recommend itself as a prerequisite.")
		}
		if _, found := seen[key]; found {
			add(PublicationIssuePrerequisiteDuplicate, keyPath, "A prerequisite can appear only once.")
		} else {
			seen[key] = struct{}{}
		}
		if _, found := lessonKeys[key]; !found {
			add(PublicationIssuePrerequisiteUnknown, keyPath, "The prerequisite stable key does not resolve in this frozen snapshot.")
		}
	}
}

func validatePublicationLessonContent(content courses.LessonContent, path string, resolvedAssets, resolvedAssessments map[string]struct{}, add func(PublicationValidationCode, string, string)) {
	if content.SchemaVersion != courses.LessonContentSchemaVersion {
		add(PublicationIssueContentSchemaUnsupported, path+".schemaVersion", "This LessonContent schema version is not supported for publication.")
		return
	}
	if content.Blocks == nil || len(content.Blocks) > courses.MaxLessonBlocks {
		add(PublicationIssueLessonContentInvalid, path+".blocks", "The LessonContent block collection is invalid.")
		return
	}
	seen := make(map[string]struct{}, len(content.Blocks))
	for index, block := range content.Blocks {
		blockPath := fmt.Sprintf("%s.blocks[%d]", path, index)
		if key, err := courses.NormalizeStructureKey(block.Key); err != nil || key != block.Key {
			add(PublicationIssueContentBlockKeyInvalid, blockPath+".key", "LessonContent block keys must be valid stable keys.")
		}
		if _, found := seen[block.Key]; found {
			add(PublicationIssueContentBlockKeyDuplicate, blockPath+".key", "LessonContent block keys must be unique within a Lesson.")
		} else {
			seen[block.Key] = struct{}{}
		}
		if err := block.Validate(); err != nil {
			add(publicationBlockIssueCode(block), blockPath, "The canonical block payload is invalid.")
			continue
		}
		switch block.Type {
		case courses.BlockImage, courses.BlockVideo, courses.BlockAudio, courses.BlockDownload:
			if !publicationBlockAssetsResolved(block, resolvedAssets) {
				add(PublicationIssueAssetUnresolved, blockPath, "This asset reference has not been resolved for publication.")
			}
		case courses.BlockKnowledgeCheck:
			payload := block.Payload.(courses.KnowledgeCheckBlockPayload)
			if _, resolved := resolvedAssessments[payload.AssessmentKey]; !resolved {
				add(PublicationIssueAssessmentUnresolved, blockPath, "This Assessment reference has not been resolved for publication.")
			}
		}
	}
}

func publicationBlockAssetsResolved(block courses.Block, resolved map[string]struct{}) bool {
	if resolved == nil {
		return false
	}
	has := func(key string) bool { _, ok := resolved[key]; return ok }
	switch payload := block.Payload.(type) {
	case courses.ImageBlockPayload:
		return has(payload.Asset.AssetKey)
	case courses.VideoBlockPayload:
		if !has(payload.Asset.AssetKey) || !has(payload.CaptionsAsset.AssetKey) {
			return false
		}
		return payload.TranscriptAsset == nil || has(payload.TranscriptAsset.AssetKey)
	case courses.AudioBlockPayload:
		return has(payload.Asset.AssetKey) && (payload.TranscriptAsset == nil || has(payload.TranscriptAsset.AssetKey))
	case courses.DownloadBlockPayload:
		return has(payload.Asset.AssetKey)
	default:
		return false
	}
}

func publicationBlockIssueCode(block courses.Block) PublicationValidationCode {
	switch block.Type {
	case courses.BlockHeading:
		return PublicationIssueHeadingInvalid
	case courses.BlockImage:
		return PublicationIssueImageAccessibilityInvalid
	case courses.BlockVideo, courses.BlockAudio:
		return PublicationIssueMediaAccessibilityInvalid
	case courses.BlockTable:
		return PublicationIssueTableInvalid
	default:
		return PublicationIssueContentBlockInvalid
	}
}

// PublicationValidationService is the future publishing orchestration input.
// It loads one exact Draft-scoped Review, then validates only its frozen data.
// It performs no authorization or mutation; those boundaries belong to later
// transport and publishing orchestration.
type PublicationValidationService struct {
	reviews   ReviewRepository
	validator PublicationValidator
}

func NewPublicationValidationService(reviews ReviewRepository) *PublicationValidationService {
	return &PublicationValidationService{reviews: reviews, validator: NewPublicationValidator()}
}

func (s *PublicationValidationService) ValidateReviewForPublication(ctx context.Context, draft DraftID, review ReviewID) (PublicationValidationResult, error) {
	if s == nil || s.reviews == nil || draft == "" || review == "" {
		return PublicationValidationResult{}, ErrReviewNotFound
	}
	cycle, snapshot, err := s.reviews.GetReviewForDraft(ctx, draft, review)
	if err != nil {
		return PublicationValidationResult{}, err
	}
	return s.validator.Validate(cycle, &snapshot), nil
}
