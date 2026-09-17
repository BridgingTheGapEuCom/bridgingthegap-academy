package authoring

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

func publicationConversionFixture(t *testing.T) (ReviewCycle, ReviewSnapshot, PublicationConversionMetadata) {
	t.Helper()
	cycle, snapshot := publicationValidationFixture(t)
	snapshot.Modules[0].Lessons[0].Content = courses.LessonContent{
		SchemaVersion: courses.LessonContentSchemaVersion,
		Blocks: []courses.Block{
			{Key: "overview", Type: courses.BlockText, Payload: courses.TextBlockPayload{Content: publicationRichText("Publication overview")}},
			{Key: "key-idea", Type: courses.BlockHeading, Payload: courses.HeadingBlockPayload{Level: 2, Content: []courses.RichTextInline{{Type: "text", Text: "Key idea"}}}},
			{Key: "example", Type: courses.BlockCode, Payload: courses.CodeBlockPayload{Code: "event := published", Language: "go", Title: "Example"}},
			{Key: "quotation", Type: courses.BlockQuote, Payload: courses.QuoteBlockPayload{Text: "Frozen content stays frozen."}},
			{Key: "remember", Type: courses.BlockCallout, Payload: courses.CalloutBlockPayload{Kind: "NOTE", Title: "Remember", Content: publicationRichText("Review the immutable source.")}},
			{Key: "facts", Type: courses.BlockTable, Payload: courses.TableBlockPayload{Headers: []string{"Property", "Value"}, Rows: [][]string{{"Source", "Review snapshot"}}}},
			{Key: "separator", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}},
		},
	}
	// Empty canonical documents remain a supported migration-era representation
	// and must survive conversion without being filled or repaired.
	snapshot.Modules[1].Lessons[0].Content = courses.LessonContent{
		SchemaVersion: courses.LessonContentSchemaVersion,
		Blocks:        []courses.Block{},
	}
	submittedAt := time.Date(2026, time.September, 10, 9, 30, 0, 0, time.UTC)
	approvedAt := time.Date(2026, time.September, 11, 15, 45, 0, 0, time.UTC)
	cycle.SubmittedAt = submittedAt
	cycle.SubmittedByUserID = "60000000-0000-4000-8000-000000000001"
	cycle.DecidedAt = &approvedAt
	cycle.DecidedByUserID = "60000000-0000-4000-8000-000000000002"
	metadata := PublicationConversionMetadata{
		PublishedAt:       time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC),
		PublishedByUserID: "60000000-0000-4000-8000-000000000003",
		Attribution: []courses.ContributorSnapshot{{
			UserID:      "60000000-0000-4000-8000-000000000001",
			DisplayName: "Course author",
			Role:        courses.ContributorAuthor,
			Order:       0,
		}},
	}
	return cycle, snapshot, metadata
}

func publicationRichText(text string) courses.RichText {
	return courses.RichText{Nodes: []courses.RichTextNode{{
		Type:    "paragraph",
		Content: []courses.RichTextInline{{Type: "text", Text: text}},
	}}}
}

func TestPublicationConverterConvertsApprovedFrozenReview(t *testing.T) {
	cycle, snapshot, metadata := publicationConversionFixture(t)
	converted, err := NewPublicationConverter().Convert(cycle, &snapshot, metadata)
	if err != nil {
		t.Fatal(err)
	}
	version := converted.CourseVersion
	if version.CourseID != snapshot.Draft.CourseID || version.Version.String() != snapshot.Draft.IntendedVersion || version.Status != courses.CourseVersionPublished {
		t.Fatalf("course identity/version not preserved: %#v", version)
	}
	if version.Title != snapshot.Draft.Title || version.Description != snapshot.Draft.Description || version.SourceLanguage != snapshot.Draft.SourceLanguage || version.Changelog != snapshot.Draft.Changelog || version.License != snapshot.Draft.License {
		t.Fatalf("course metadata not preserved: %#v", version)
	}
	if !reflect.DeepEqual(version.LearningObjectives, snapshot.Draft.Objectives) || !reflect.DeepEqual(version.Attribution, metadata.Attribution) || version.PublishedAt != metadata.PublishedAt {
		t.Fatalf("publication metadata not preserved: %#v", version)
	}
	if converted.Provenance.ReviewID != string(cycle.ID) || converted.Provenance.ReviewRevision != cycle.Revision || converted.Provenance.DraftID != string(cycle.DraftID) || converted.Provenance.DraftRevision != cycle.DraftRevision || converted.Provenance.SnapshotSchemaVersion != snapshot.SchemaVersion || converted.Provenance.SubmittedByUserID != cycle.SubmittedByUserID || converted.Provenance.ApprovedByUserID != cycle.DecidedByUserID || converted.Provenance.ApprovedAt == nil || *converted.Provenance.ApprovedAt != *cycle.DecidedAt || converted.Provenance.PublishedByUserID != metadata.PublishedByUserID {
		t.Fatalf("Review provenance not preserved: %#v", converted.Provenance)
	}
	if len(converted.Modules) != len(snapshot.Modules) || converted.Modules[0].StableKey != snapshot.Modules[0].StableKey || converted.Modules[1].Position != 1 {
		t.Fatalf("Module order not preserved: %#v", converted.Modules)
	}
	lesson := converted.Modules[1].Lessons[0]
	if lesson.StableKey != "exercise" || !reflect.DeepEqual(lesson.PrerequisiteStableKeys, []string{"introduction"}) || len(lesson.Content.Blocks) != 0 {
		t.Fatalf("Lesson order/content/prerequisites not preserved: %#v", lesson)
	}
	wantBlockKeys := []string{"overview", "key-idea", "example", "quotation", "remember", "facts", "separator"}
	gotBlockKeys := make([]string, 0, len(converted.Modules[0].Lessons[0].Content.Blocks))
	for _, block := range converted.Modules[0].Lessons[0].Content.Blocks {
		gotBlockKeys = append(gotBlockKeys, block.Key)
	}
	if !reflect.DeepEqual(gotBlockKeys, wantBlockKeys) || !reflect.DeepEqual(converted.Modules[0].Lessons[0].Content, snapshot.Modules[0].Lessons[0].Content) {
		t.Fatalf("canonical content changed: keys=%#v content=%#v", gotBlockKeys, converted.Modules[0].Lessons[0].Content)
	}
	if converted.ID != "" || converted.Modules[0].ID != "" || converted.Modules[0].Lessons[0].ID != "" {
		t.Fatalf("conversion generated persistence identifiers: %#v", converted)
	}
}

func TestPublicationConverterRequiresSuccessfulValidation(t *testing.T) {
	cycle, snapshot, metadata := publicationConversionFixture(t)
	tests := []struct {
		name   string
		mutate func(*ReviewCycle, *ReviewSnapshot)
		code   PublicationValidationCode
	}{
		{name: "in review", mutate: func(c *ReviewCycle, _ *ReviewSnapshot) { c.Status = ReviewInReview }, code: PublicationIssueReviewNotApproved},
		{name: "changes requested", mutate: func(c *ReviewCycle, _ *ReviewSnapshot) { c.Status = ReviewChangesRequested }, code: PublicationIssueReviewNotApproved},
		{name: "unsupported snapshot", mutate: func(_ *ReviewCycle, s *ReviewSnapshot) { s.SchemaVersion = 2 }, code: PublicationIssueUnsupportedSnapshotVersion},
		{name: "missing Course identity", mutate: func(_ *ReviewCycle, s *ReviewSnapshot) { s.Draft.CourseID = "" }, code: PublicationIssueCourseReferenceInvalid},
		{name: "invalid metadata", mutate: func(_ *ReviewCycle, s *ReviewSnapshot) { s.Draft.IntendedVersion = "2.3" }, code: PublicationIssueIntendedVersionInvalid},
		{name: "malformed content", mutate: func(_ *ReviewCycle, s *ReviewSnapshot) {
			s.Modules[0].Lessons[0].Content.Blocks[0] = courses.Block{
				Key: "bad-heading", Type: courses.BlockHeading,
				Payload: courses.HeadingBlockPayload{Level: 1, Content: []courses.RichTextInline{{Type: "text", Text: "Invalid"}}},
			}
		}, code: PublicationIssueHeadingInvalid},
		{name: "provenance mismatch", mutate: func(c *ReviewCycle, _ *ReviewSnapshot) { c.DraftRevision++ }, code: PublicationIssueSnapshotReviewMismatch},
		{name: "different Review", mutate: func(c *ReviewCycle, _ *ReviewSnapshot) { c.DraftID = "10000000-0000-4000-8000-000000000099" }, code: PublicationIssueSnapshotReviewMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidateCycle := cycle
			candidateSnapshot := clonePublicationSnapshot(t, snapshot)
			test.mutate(&candidateCycle, &candidateSnapshot)
			converted, err := NewPublicationConverter().Convert(candidateCycle, &candidateSnapshot, metadata)
			if !errors.Is(err, ErrPublicationValidationFailed) || !reflect.DeepEqual(converted, courses.ImmutableCourseVersion{}) {
				t.Fatalf("conversion result = %#v, %v", converted, err)
			}
			var validationFailure *PublicationValidationFailure
			if !errors.As(err, &validationFailure) || !hasPublicationIssue(validationFailure.Result, test.code) {
				t.Fatalf("validation issues = %#v", validationFailure)
			}
		})
	}

	if converted, err := NewPublicationConverter().Convert(cycle, nil, metadata); !errors.Is(err, ErrPublicationValidationFailed) || !reflect.DeepEqual(converted, courses.ImmutableCourseVersion{}) {
		t.Fatalf("missing snapshot conversion = %#v, %v", converted, err)
	}
}

func TestPublicationConverterRequiresExplicitValidPublicationMetadata(t *testing.T) {
	cycle, snapshot, metadata := publicationConversionFixture(t)
	metadata.PublishedAt = time.Time{}
	if converted, err := NewPublicationConverter().Convert(cycle, &snapshot, metadata); !errors.Is(err, ErrPublicationConversionInput) || !reflect.DeepEqual(converted, courses.ImmutableCourseVersion{}) {
		t.Fatalf("missing publication time = %#v, %v", converted, err)
	}
	metadata.PublishedAt = time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC)
	metadata.Attribution = nil
	if converted, err := NewPublicationConverter().Convert(cycle, &snapshot, metadata); !errors.Is(err, ErrPublicationConversionInput) || !reflect.DeepEqual(converted, courses.ImmutableCourseVersion{}) {
		t.Fatalf("missing attribution = %#v, %v", converted, err)
	}

	_, _, metadata = publicationConversionFixture(t)
	metadata.PublishedByUserID = ""
	if converted, err := NewPublicationConverter().Convert(cycle, &snapshot, metadata); !errors.Is(err, ErrPublicationConversionInput) || !reflect.DeepEqual(converted, courses.ImmutableCourseVersion{}) {
		t.Fatalf("missing publication actor = %#v, %v", converted, err)
	}

	cycle, _, metadata = publicationConversionFixture(t)
	cycle.SubmittedAt = time.Time{}
	if converted, err := NewPublicationConverter().Convert(cycle, &snapshot, metadata); !errors.Is(err, ErrPublicationConversionInput) || !reflect.DeepEqual(converted, courses.ImmutableCourseVersion{}) {
		t.Fatalf("missing Review provenance = %#v, %v", converted, err)
	}
}

func TestPublicationConversionCopiesCanonicalDependencyReferencesWithoutResolvingThem(t *testing.T) {
	cycle, snapshot, metadata := publicationConversionFixture(t)
	snapshot.Modules[0].Lessons[0].Content.Blocks = []courses.Block{
		{Key: "architecture-image", Type: courses.BlockImage, Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: "architecture-diagram"}, AltText: "Architecture diagram"}},
		{Key: "knowledge-check", Type: courses.BlockKnowledgeCheck, Payload: courses.KnowledgeCheckBlockPayload{AssessmentKey: "architecture-check"}},
	}

	// M4.5a currently blocks unresolved delivery dependencies. Exercise the
	// pure copy stage directly to prove that a later resolver can clear those
	// issues without requiring any lossy change to this conversion boundary.
	converted, err := buildImmutableCourseVersion(cycle, snapshot, metadata)
	if err != nil {
		t.Fatal(err)
	}
	blocks := converted.Modules[0].Lessons[0].Content.Blocks
	image, ok := blocks[0].Payload.(courses.ImageBlockPayload)
	if !ok || image.Asset.AssetKey != "architecture-diagram" || image.AltText != "Architecture diagram" {
		t.Fatalf("asset/accessibility payload changed: %#v", blocks[0])
	}
	check, ok := blocks[1].Payload.(courses.KnowledgeCheckBlockPayload)
	if !ok || check.AssessmentKey != "architecture-check" {
		t.Fatalf("assessment reference changed: %#v", blocks[1])
	}
}

func TestPublicationConverterIsDeterministicAndIsolatesOutputFromInput(t *testing.T) {
	cycle, snapshot, metadata := publicationConversionFixture(t)
	first, err := NewPublicationConverter().Convert(cycle, &snapshot, metadata)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewPublicationConverter().Convert(cycle, &snapshot, metadata)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("non-deterministic conversion:\nfirst=%#v\nsecond=%#v", first, second)
	}

	encodedBefore, err := json.Marshal(first.Modules[0].Lessons[0].Content)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Draft.Title = "Changed mutable Draft snapshot"
	snapshot.Draft.Objectives[0] = "Changed objective"
	snapshot.Modules[0].Title = "Changed Module"
	snapshot.Modules[0].Lessons[0].Objectives[0] = "Changed Lesson objective"
	snapshot.Modules[0].Lessons[0].Content.Blocks[0].Key = "changed-block"
	snapshot.Modules[1].Lessons[0].PrerequisiteStableKeys[0] = "changed-prerequisite"
	metadata.Attribution[0].DisplayName = "Changed attribution"
	cycle.DecidedByUserID = "changed-reviewer"
	*cycle.DecidedAt = cycle.DecidedAt.Add(24 * time.Hour)
	encodedAfter, err := json.Marshal(first.Modules[0].Lessons[0].Content)
	if err != nil {
		t.Fatal(err)
	}
	if first.CourseVersion.Title == snapshot.Draft.Title || first.CourseVersion.LearningObjectives[0] == snapshot.Draft.Objectives[0] || first.Modules[0].Title == snapshot.Modules[0].Title || first.Modules[0].Lessons[0].LearningObjectives[0] == snapshot.Modules[0].Lessons[0].Objectives[0] || first.Modules[1].Lessons[0].PrerequisiteStableKeys[0] == snapshot.Modules[1].Lessons[0].PrerequisiteStableKeys[0] || first.CourseVersion.Attribution[0].DisplayName == metadata.Attribution[0].DisplayName || first.Provenance.ApprovedByUserID == cycle.DecidedByUserID || first.Provenance.ApprovedAt == cycle.DecidedAt || first.Provenance.ApprovedAt.Equal(*cycle.DecidedAt) || string(encodedBefore) != string(encodedAfter) {
		t.Fatal("converted CourseVersion retained mutable Review/conversion input aliases")
	}
}
