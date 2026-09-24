package courses

import (
	"context"
	"errors"
	"testing"
	"time"
)

type immutableCourseVersionRepositoryFake struct {
	stored ImmutableCourseVersion
	err    error
	calls  int
}

func (r *immutableCourseVersionRepositoryFake) StoreImmutableCourseVersion(_ context.Context, version ImmutableCourseVersion) (ImmutableCourseVersion, error) {
	r.calls++
	r.stored = version
	return version, r.err
}

func (r *immutableCourseVersionRepositoryFake) GetImmutableCourseVersion(context.Context, CourseVersionID) (ImmutableCourseVersion, error) {
	return r.stored, r.err
}

func (r *immutableCourseVersionRepositoryFake) GetImmutableCourseVersionByReviewID(context.Context, string) (ImmutableCourseVersion, error) {
	return r.stored, r.err
}

func (r *immutableCourseVersionRepositoryFake) GetImmutableCourseVersionByCourseAndVersion(context.Context, CourseID, Version) (ImmutableCourseVersion, error) {
	return r.stored, r.err
}

func validImmutableCourseVersion(t *testing.T) ImmutableCourseVersion {
	t.Helper()
	version, err := ParseVersion("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	approvedAt := time.Date(2026, time.September, 11, 12, 0, 0, 0, time.UTC)
	result := ImmutableCourseVersion{
		CourseVersion: CourseVersionInput{
			CourseID: "10000000-0000-4000-8000-000000000001", Version: version, Status: CourseVersionPublished,
			Title: "Immutable version", Description: "A complete immutable version.", LearningObjectives: []string{"Explain persistence"},
			SourceLanguage: "en", Changelog: "Initial publication.",
			License:     ContentLicense{Kind: ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"},
			Attribution: []ContributorSnapshot{{DisplayName: "Course author", Role: ContributorAuthor, Order: 0}},
			PublishedAt: time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC),
		},
		Provenance: CourseVersionProvenance{
			ReviewID: "20000000-0000-4000-8000-000000000001", ReviewRevision: 2,
			DraftID: "30000000-0000-4000-8000-000000000001", DraftRevision: 7, SnapshotSchemaVersion: 1,
			SubmittedByUserID: "40000000-0000-4000-8000-000000000001", SubmittedAt: time.Date(2026, time.September, 10, 12, 0, 0, 0, time.UTC),
			ApprovedByUserID: "40000000-0000-4000-8000-000000000002", ApprovedAt: &approvedAt,
			PublishedByUserID: "40000000-0000-4000-8000-000000000003",
		},
		Modules: []ImmutableCourseVersionModule{},
	}
	result.Publication = PublicationProvenance{Origin: PublicationOriginNative, Native: &result.Provenance}
	return result
}

func TestCourseVersionStoreValidatesBeforeRepositoryWrite(t *testing.T) {
	valid := validImmutableCourseVersion(t)
	repository := &immutableCourseVersionRepositoryFake{}
	store := NewCourseVersionStore(repository)
	if _, err := store.Store(context.Background(), valid); err != nil || repository.calls != 1 {
		t.Fatalf("valid store = %v, calls=%d", err, repository.calls)
	}

	invalid := valid
	invalid.ID = "persistence-owned-id"
	if _, err := store.Store(context.Background(), invalid); !errors.Is(err, ErrInvalidImmutableCourseVersion) || repository.calls != 1 {
		t.Fatalf("invalid store = %v, calls=%d", err, repository.calls)
	}
}

func TestImmutableCourseVersionPersistenceValidation(t *testing.T) {
	valid := validImmutableCourseVersion(t)
	if err := valid.ValidateForPersistence(); err != nil {
		t.Fatalf("valid aggregate rejected: %v", err)
	}

	approvedAt := *valid.Provenance.ApprovedAt
	tests := []struct {
		name   string
		mutate func(*ImmutableCourseVersion)
	}{
		{name: "not published", mutate: func(v *ImmutableCourseVersion) { v.CourseVersion.Status = CourseVersionArchived }},
		{name: "missing approval", mutate: func(v *ImmutableCourseVersion) { v.Provenance.ApprovedAt = nil }},
		{name: "generated ID supplied", mutate: func(v *ImmutableCourseVersion) { v.ID = "50000000-0000-4000-8000-000000000001" }},
		{name: "missing Modules collection", mutate: func(v *ImmutableCourseVersion) { v.Modules = nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Provenance.ApprovedAt = &approvedAt
			test.mutate(&candidate)
			if !errors.Is(candidate.ValidateForPersistence(), ErrInvalidImmutableCourseVersion) {
				t.Fatal("invalid persistence aggregate accepted")
			}
		})
	}
}

func TestImmutableCourseVersionRequiresExactValidAssetBindings(t *testing.T) {
	assetKey := "51000000-0000-4000-8000-000000000001"
	valid := validImmutableCourseVersion(t)
	valid.Modules = []ImmutableCourseVersionModule{{
		SourceID: "52000000-0000-4000-8000-000000000001", StableKey: "module", Title: "Module", Position: 0,
		Lessons: []ImmutableCourseVersionLesson{{
			SourceID: "53000000-0000-4000-8000-000000000001", StableKey: "lesson", Title: "Lesson", Description: "A lesson with an image.", Position: 0,
			LearningObjectives: []string{"Explain the diagram"}, PrerequisiteStableKeys: []string{},
			Content: LessonContent{SchemaVersion: 1, Blocks: []Block{{Key: "image", Type: BlockImage, Payload: ImageBlockPayload{Asset: AssetReference{AssetKey: assetKey}, AltText: "Diagram"}}}},
		}},
	}}
	valid.AssetBindings = []PublishedAssetBinding{{
		AssetKey: assetKey, StorageObjectID: "54000000-0000-4000-8000-000000000001",
		OriginalFilename: "diagram.png", MediaType: "image/png", ByteSize: 42,
		SHA256Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	if err := valid.ValidateForPersistence(); err != nil {
		t.Fatalf("valid binding rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ImmutableCourseVersion)
	}{
		{"missing", func(v *ImmutableCourseVersion) { v.AssetBindings = nil }},
		{"duplicate", func(v *ImmutableCourseVersion) { v.AssetBindings = append(v.AssetBindings, v.AssetBindings[0]) }},
		{"invalid digest", func(v *ImmutableCourseVersion) { v.AssetBindings[0].SHA256Digest = "bad" }},
		{"invalid size", func(v *ImmutableCourseVersion) { v.AssetBindings[0].ByteSize = 0 }},
		{"invalid media type", func(v *ImmutableCourseVersion) { v.AssetBindings[0].MediaType = "IMAGE/PNG" }},
		{"incompatible media type", func(v *ImmutableCourseVersion) { v.AssetBindings[0].MediaType = "text/plain" }},
		{"unreferenced", func(v *ImmutableCourseVersion) { v.AssetBindings[0].AssetKey = "55000000-0000-4000-8000-000000000001" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.AssetBindings = append([]PublishedAssetBinding{}, valid.AssetBindings...)
			test.mutate(&candidate)
			if !errors.Is(candidate.ValidateForPersistence(), ErrInvalidImmutableCourseVersion) {
				t.Fatal("invalid binding accepted")
			}
		})
	}
}

func TestImmutableCourseVersionRequiresExactValidAssessmentBindings(t *testing.T) {
	assessmentKey := "71000000-0000-4000-8000-000000000001"
	valid := validImmutableCourseVersion(t)
	valid.Modules = []ImmutableCourseVersionModule{{SourceID: "52000000-0000-4000-8000-000000000001", StableKey: "module", Title: "Module", Position: 0, Lessons: []ImmutableCourseVersionLesson{{
		SourceID: "53000000-0000-4000-8000-000000000001", StableKey: "lesson", Title: "Lesson", Description: "A lesson with a check.", Position: 0, LearningObjectives: []string{"Check learning"}, PrerequisiteStableKeys: []string{},
		Content: LessonContent{SchemaVersion: 1, Blocks: []Block{{Key: "check", Type: BlockKnowledgeCheck, Payload: KnowledgeCheckBlockPayload{AssessmentKey: assessmentKey}}}},
	}}}}
	valid.AssessmentBindings = []PublishedAssessmentBinding{{AssessmentKey: assessmentKey, Questions: []PublishedAssessmentQuestion{{StableKey: "question", Type: PublishedQuestionSingleChoice, Prompt: "Choose", Position: 0, Options: []PublishedAssessmentOption{{StableKey: "one", Text: "One", Position: 0}, {StableKey: "two", Text: "Two", Position: 1}}, CorrectOptionKeys: []string{"one"}}}}}
	if err := valid.ValidateForPersistence(); err != nil {
		t.Fatalf("valid Assessment binding rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*ImmutableCourseVersion)
	}{
		{"missing", func(v *ImmutableCourseVersion) { v.AssessmentBindings = nil }},
		{"duplicate", func(v *ImmutableCourseVersion) {
			v.AssessmentBindings = append(v.AssessmentBindings, v.AssessmentBindings[0])
		}},
		{"unreferenced", func(v *ImmutableCourseVersion) {
			v.AssessmentBindings[0].AssessmentKey = "72000000-0000-4000-8000-000000000001"
		}},
		{"empty", func(v *ImmutableCourseVersion) { v.AssessmentBindings[0].Questions = nil }},
		{"unsupported", func(v *ImmutableCourseVersion) { v.AssessmentBindings[0].Questions[0].Type = "ESSAY" }},
		{"bad answer", func(v *ImmutableCourseVersion) {
			v.AssessmentBindings[0].Questions[0].CorrectOptionKeys = []string{"missing"}
		}},
		{"bad order", func(v *ImmutableCourseVersion) { v.AssessmentBindings[0].Questions[0].Options[0].Position = 2 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.AssessmentBindings = append([]PublishedAssessmentBinding(nil), valid.AssessmentBindings...)
			candidate.AssessmentBindings[0].Questions = append([]PublishedAssessmentQuestion(nil), valid.AssessmentBindings[0].Questions...)
			candidate.AssessmentBindings[0].Questions[0].Options = append([]PublishedAssessmentOption(nil), valid.AssessmentBindings[0].Questions[0].Options...)
			candidate.AssessmentBindings[0].Questions[0].CorrectOptionKeys = append([]string(nil), valid.AssessmentBindings[0].Questions[0].CorrectOptionKeys...)
			test.mutate(&candidate)
			if !errors.Is(candidate.ValidateForPersistence(), ErrInvalidImmutableCourseVersion) {
				t.Fatal("invalid Assessment binding accepted")
			}
		})
	}
}
