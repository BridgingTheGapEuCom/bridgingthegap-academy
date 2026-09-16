package authoring

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

func publicationValidationFixture(t *testing.T) (ReviewCycle, ReviewSnapshot) {
	t.Helper()
	draft, modules, lessons, prerequisites := reviewSnapshotFixture(t)
	snapshot, err := NewReviewSnapshot(draft, modules, lessons, prerequisites)
	if err != nil {
		t.Fatal(err)
	}
	// Empty canonical content remains valid for legacy migration rows, but new
	// publication requires content. Give this fixture a non-empty valid block.
	snapshot.Modules[1].Lessons[0].Content.Blocks = []courses.Block{{
		Key: "published-divider", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{},
	}}
	cycle := ReviewCycle{
		ID:                    "50000000-0000-4000-8000-000000000001",
		DraftID:               snapshot.Draft.ID,
		DraftRevision:         snapshot.Draft.Revision,
		SnapshotSchemaVersion: snapshot.SchemaVersion,
		Status:                ReviewApproved,
		Revision:              2,
		SubmittedByUserID:     "author-a",
	}
	return cycle, snapshot
}

func clonePublicationSnapshot(t *testing.T, snapshot ReviewSnapshot) ReviewSnapshot {
	t.Helper()
	encoded, err := MarshalReviewSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var clone ReviewSnapshot
	if err := json.Unmarshal(encoded, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func issueCodes(result PublicationValidationResult) []PublicationValidationCode {
	codes := make([]PublicationValidationCode, 0, len(result.Issues))
	for _, issue := range result.Issues {
		codes = append(codes, issue.Code)
	}
	return codes
}

func hasPublicationIssue(result PublicationValidationResult, code PublicationValidationCode) bool {
	for _, issue := range result.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestPublicationValidatorAcceptsApprovedFrozenSnapshot(t *testing.T) {
	cycle, snapshot := publicationValidationFixture(t)
	result := NewPublicationValidator().Validate(cycle, &snapshot)
	if !result.Publishable || len(result.Issues) != 0 {
		t.Fatalf("approved valid snapshot = %#v", result)
	}
}

func TestPublicationValidatorReviewEligibilityAndSnapshotVersion(t *testing.T) {
	cycle, snapshot := publicationValidationFixture(t)
	for _, status := range []ReviewStatus{ReviewInReview, ReviewChangesRequested} {
		candidate := cycle
		candidate.Status = status
		result := NewPublicationValidator().Validate(candidate, &snapshot)
		if result.Publishable || !hasPublicationIssue(result, PublicationIssueReviewNotApproved) {
			t.Fatalf("status %s = %#v", status, result)
		}
	}
	if result := NewPublicationValidator().Validate(cycle, nil); result.Publishable || !hasPublicationIssue(result, PublicationIssueMissingSnapshot) {
		t.Fatalf("missing snapshot = %#v", result)
	}
	unsupported := clonePublicationSnapshot(t, snapshot)
	unsupported.SchemaVersion = 2
	if result := NewPublicationValidator().Validate(cycle, &unsupported); result.Publishable || !hasPublicationIssue(result, PublicationIssueUnsupportedSnapshotVersion) {
		t.Fatalf("unsupported snapshot = %#v", result)
	}
	mismatched := clonePublicationSnapshot(t, snapshot)
	mismatched.Draft.Revision++
	if result := NewPublicationValidator().Validate(cycle, &mismatched); result.Publishable || !hasPublicationIssue(result, PublicationIssueSnapshotReviewMismatch) {
		t.Fatalf("snapshot/review mismatch = %#v", result)
	}
	unsupportedCycle := cycle
	unsupportedCycle.SnapshotSchemaVersion = 2
	if result := NewPublicationValidator().Validate(unsupportedCycle, &snapshot); result.Publishable || !hasPublicationIssue(result, PublicationIssueUnsupportedSnapshotVersion) {
		t.Fatalf("unsupported Review snapshot version = %#v", result)
	}
}

func TestPublicationValidatorUsesCoursesMetadataValidation(t *testing.T) {
	cycle, snapshot := publicationValidationFixture(t)
	invalidVersion := clonePublicationSnapshot(t, snapshot)
	invalidVersion.Draft.IntendedVersion = "2.3"
	result := NewPublicationValidator().Validate(cycle, &invalidVersion)
	if result.Publishable || !hasPublicationIssue(result, PublicationIssueIntendedVersionInvalid) {
		t.Fatalf("invalid version = %#v", result)
	}

	invalidLicense := clonePublicationSnapshot(t, snapshot)
	invalidLicense.Draft.License = courses.ContentLicense{Kind: courses.ContentLicenseStandard, DisplayName: "Missing identifier"}
	result = NewPublicationValidator().Validate(cycle, &invalidLicense)
	if result.Publishable || !hasPublicationIssue(result, PublicationIssueContentLicenseInvalid) {
		t.Fatalf("invalid license = %#v", result)
	}

	invalidMetadata := clonePublicationSnapshot(t, snapshot)
	invalidMetadata.Draft.Title = ""
	result = NewPublicationValidator().Validate(cycle, &invalidMetadata)
	if result.Publishable || !hasPublicationIssue(result, PublicationIssueCourseMetadataInvalid) {
		t.Fatalf("invalid metadata = %#v", result)
	}
}

func TestPublicationValidatorValidatesStructureAndPrerequisitesWithoutCycleDetection(t *testing.T) {
	cycle, snapshot := publicationValidationFixture(t)
	broken := clonePublicationSnapshot(t, snapshot)
	broken.Modules[1].Position = 4
	broken.Modules[1].StableKey = broken.Modules[0].StableKey
	broken.Modules[1].Lessons[0].ID = broken.Modules[0].Lessons[0].ID
	broken.Modules[1].Lessons[0].StableKey = broken.Modules[0].Lessons[0].StableKey
	broken.Modules[1].Lessons[0].Position = 2
	broken.Modules[0].Lessons[0].PrerequisiteStableKeys = []string{"introduction", "missing", "missing"}
	result := NewPublicationValidator().Validate(cycle, &broken)
	for _, code := range []PublicationValidationCode{
		PublicationIssueModulePositionInvalid,
		PublicationIssueModuleKeyDuplicate,
		PublicationIssueLessonIDDuplicate,
		PublicationIssueLessonKeyDuplicate,
		PublicationIssueLessonPositionInvalid,
		PublicationIssuePrerequisiteSelf,
		PublicationIssuePrerequisiteUnknown,
		PublicationIssuePrerequisiteDuplicate,
	} {
		if !hasPublicationIssue(result, code) {
			t.Fatalf("missing %s from %#v", code, result.Issues)
		}
	}

	cycleSnapshot := clonePublicationSnapshot(t, snapshot)
	cycleSnapshot.Modules[0].Lessons[0].PrerequisiteStableKeys = []string{"exercise"}
	// The fixture already has exercise -> introduction. Advisory prerequisite
	// cycles remain intentionally accepted by the current domain.
	if result := NewPublicationValidator().Validate(cycle, &cycleSnapshot); !result.Publishable {
		t.Fatalf("advisory prerequisite cycle = %#v", result)
	}
}

func TestPublicationValidatorPreservesExistingEmptyStructurePolicy(t *testing.T) {
	cycle, snapshot := publicationValidationFixture(t)
	empty := clonePublicationSnapshot(t, snapshot)
	// Courses has no aggregate restriction that a new CourseVersion must have a
	// Module or that every Module must have a Lesson. Do not invent one here.
	empty.Modules = []ReviewSnapshotModule{}
	if result := NewPublicationValidator().Validate(cycle, &empty); !result.Publishable {
		t.Fatalf("empty structure added an unsupported publication rule: %#v", result)
	}

	emptyModule := clonePublicationSnapshot(t, snapshot)
	emptyModule.Modules[0].Lessons = []ReviewSnapshotLesson{}
	emptyModule.Modules = emptyModule.Modules[:1]
	if result := NewPublicationValidator().Validate(cycle, &emptyModule); !result.Publishable {
		t.Fatalf("empty Module added an unsupported publication rule: %#v", result)
	}
}

func TestPublicationValidatorValidatesCanonicalContentAndDependencies(t *testing.T) {
	cycle, snapshot := publicationValidationFixture(t)
	broken := clonePublicationSnapshot(t, snapshot)
	lesson := &broken.Modules[0].Lessons[0]
	lesson.Content.Blocks = []courses.Block{
		{Key: "heading", Type: courses.BlockHeading, Payload: courses.HeadingBlockPayload{Level: 1, Content: []courses.RichTextInline{{Type: "text", Text: "Bad heading"}}}},
		{Key: "heading", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}},
	}
	result := NewPublicationValidator().Validate(cycle, &broken)
	if result.Publishable || !hasPublicationIssue(result, PublicationIssueHeadingInvalid) || !hasPublicationIssue(result, PublicationIssueContentBlockKeyDuplicate) {
		t.Fatalf("invalid content = %#v", result)
	}

	accessibility := clonePublicationSnapshot(t, snapshot)
	accessibility.Modules[0].Lessons[0].Content.Blocks = []courses.Block{
		{Key: "image", Type: courses.BlockImage, Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: "diagram"}}},
	}
	result = NewPublicationValidator().Validate(cycle, &accessibility)
	if result.Publishable || !hasPublicationIssue(result, PublicationIssueImageAccessibilityInvalid) {
		t.Fatalf("invalid image accessibility = %#v", result)
	}

	emptyContent := clonePublicationSnapshot(t, snapshot)
	emptyContent.Modules[0].Lessons[0].Content.Blocks = []courses.Block{}
	result = NewPublicationValidator().Validate(cycle, &emptyContent)
	if result.Publishable || !hasPublicationIssue(result, PublicationIssueLessonContentEmpty) {
		t.Fatalf("empty publication content = %#v", result)
	}

	unsupported := clonePublicationSnapshot(t, snapshot)
	unsupported.Modules[0].Lessons[0].Content.SchemaVersion = 2
	result = NewPublicationValidator().Validate(cycle, &unsupported)
	if result.Publishable || !hasPublicationIssue(result, PublicationIssueContentSchemaUnsupported) {
		t.Fatalf("unsupported content = %#v", result)
	}

	dependencies := clonePublicationSnapshot(t, snapshot)
	dependencies.Modules[0].Lessons[0].Content.Blocks = []courses.Block{
		{Key: "image", Type: courses.BlockImage, Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: "diagram"}, AltText: "Diagram"}},
		{Key: "check", Type: courses.BlockKnowledgeCheck, Payload: courses.KnowledgeCheckBlockPayload{AssessmentKey: "review-check"}},
	}
	result = NewPublicationValidator().Validate(cycle, &dependencies)
	if result.Publishable || !hasPublicationIssue(result, PublicationIssueAssetUnresolved) || !hasPublicationIssue(result, PublicationIssueAssessmentUnresolved) {
		t.Fatalf("unresolved dependencies = %#v", result)
	}
}

func TestPublicationValidatorIsDeterministicAndDoesNotMutateSnapshot(t *testing.T) {
	cycle, snapshot := publicationValidationFixture(t)
	broken := clonePublicationSnapshot(t, snapshot)
	broken.Draft.IntendedVersion = "invalid"
	broken.Modules[0].Position = 3
	broken.Modules[0].Lessons[0].Content.Blocks = []courses.Block{{Key: "", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}}}
	before := clonePublicationSnapshot(t, snapshot)
	inputBefore, err := json.Marshal(broken)
	if err != nil {
		t.Fatal(err)
	}
	first := NewPublicationValidator().Validate(cycle, &broken)
	second := NewPublicationValidator().Validate(cycle, &broken)
	inputAfter, err := json.Marshal(broken)
	if err != nil {
		t.Fatal(err)
	}
	if first.Publishable || !reflect.DeepEqual(first, second) || string(inputBefore) != string(inputAfter) {
		t.Fatalf("non-deterministic or mutating validation: first=%#v second=%#v before=%s after=%s", first, second, inputBefore, inputAfter)
	}
	if !reflect.DeepEqual(snapshot, before) {
		t.Fatal("fixture clone changed source snapshot")
	}
	if len(first.Issues) < 3 || first.Issues[0].Code != PublicationIssueIntendedVersionInvalid || first.Issues[1].Code != PublicationIssueModulePositionInvalid || first.Issues[2].Code != PublicationIssueContentBlockKeyInvalid {
		t.Fatalf("issue order = %#v", first.Issues)
	}
}

func TestPublicationValidationServiceLoadsOnlyExactFrozenReview(t *testing.T) {
	cycle, snapshot := publicationValidationFixture(t)
	repository := &reviewRepositoryFake{cycle: cycle, snapshot: snapshot}
	service := NewPublicationValidationService(repository)
	result, err := service.ValidateReviewForPublication(context.Background(), cycle.DraftID, cycle.ID)
	if err != nil || !result.Publishable {
		t.Fatalf("validation service = %#v, %v", result, err)
	}
	if repository.getReviewCalls != 1 || repository.lastDraft != cycle.DraftID || repository.lastReview != cycle.ID {
		t.Fatalf("service did not use exact Review scope: %#v", repository)
	}
	repository.err = errors.New("storage unavailable")
	if _, err := service.ValidateReviewForPublication(context.Background(), cycle.DraftID, cycle.ID); !errors.Is(err, repository.err) {
		t.Fatalf("repository failure = %v", err)
	}
}
