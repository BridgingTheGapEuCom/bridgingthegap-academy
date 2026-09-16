package courses

import (
	"context"
	"errors"
	"testing"
)

type publishedReadRepositoryFake struct {
	exact        ImmutableCourseVersion
	latest       ImmutableCourseVersion
	exactErr     error
	latestErr    error
	exactCourse  CourseID
	exactVersion Version
	latestCourse CourseID
}

func (r *publishedReadRepositoryFake) GetPublishedImmutableCourseVersionByCourseAndVersion(_ context.Context, courseID CourseID, version Version) (ImmutableCourseVersion, error) {
	r.exactCourse, r.exactVersion = courseID, version
	return r.exact, r.exactErr
}

func (r *publishedReadRepositoryFake) GetLatestPublishedImmutableCourseVersion(_ context.Context, courseID CourseID) (ImmutableCourseVersion, error) {
	r.latestCourse = courseID
	return r.latest, r.latestErr
}

func publishedReadAggregate(t *testing.T) ImmutableCourseVersion {
	t.Helper()
	aggregate := validImmutableCourseVersion(t)
	aggregate.ID = "50000000-0000-4000-8000-000000000001"
	aggregate.Modules = []ImmutableCourseVersionModule{{
		ID: "60000000-0000-4000-8000-000000000001", SourceID: "61000000-0000-4000-8000-000000000001",
		StableKey: "foundations", Title: "Foundations", Description: "Canonical foundation material.", Position: 0,
		Lessons: []ImmutableCourseVersionLesson{{
			ID: "70000000-0000-4000-8000-000000000001", SourceID: "71000000-0000-4000-8000-000000000001",
			StableKey: "introduction", Title: "Introduction", Description: "Learn immutable reading.",
			LearningObjectives: []string{"Read canonical content"}, Position: 0,
			PrerequisiteStableKeys: []string{},
			Content: LessonContent{SchemaVersion: LessonContentSchemaVersion, Blocks: []Block{
				{Key: "diagram", Type: BlockImage, Payload: ImageBlockPayload{Asset: AssetReference{AssetKey: "architecture-diagram"}, AltText: "A course-version boundary diagram"}},
				{Key: "assessment", Type: BlockKnowledgeCheck, Payload: KnowledgeCheckBlockPayload{AssessmentKey: "publication-check"}},
			}},
		}},
	}}
	return aggregate
}

func TestPublishedReadServiceReturnsProvenanceFreeImmutableProjection(t *testing.T) {
	aggregate := publishedReadAggregate(t)
	repository := &publishedReadRepositoryFake{exact: aggregate}
	service := NewPublishedReadService(repository)

	got, err := service.Exact(context.Background(), aggregate.CourseVersion.CourseID, aggregate.CourseVersion.Version)
	if err != nil {
		t.Fatal(err)
	}
	if repository.exactCourse != aggregate.CourseVersion.CourseID || repository.exactVersion != aggregate.CourseVersion.Version {
		t.Fatalf("exact read did not use course ID + SemVer: %#v", repository)
	}
	if got.CourseID != aggregate.CourseVersion.CourseID || got.Version != aggregate.CourseVersion.Version || got.PublishedAt != aggregate.CourseVersion.PublishedAt || got.License != aggregate.CourseVersion.License {
		t.Fatalf("published metadata was not preserved: %#v", got)
	}
	if len(got.Contributors) != 1 || got.Contributors[0].DisplayName != "Course author" || got.Contributors[0].Role != ContributorAuthor {
		t.Fatalf("public attribution was not preserved: %#v", got.Contributors)
	}
	if len(got.Modules) != 1 || len(got.Modules[0].Lessons) != 1 || got.Modules[0].Lessons[0].Content.Blocks[0].Type != BlockImage || got.Modules[0].Lessons[0].Content.Blocks[1].Type != BlockKnowledgeCheck {
		t.Fatalf("module, lesson, asset, or assessment payload was lost: %#v", got.Modules)
	}
}

func TestPublishedReadServiceReturnsIsolatedCopies(t *testing.T) {
	aggregate := publishedReadAggregate(t)
	repository := &publishedReadRepositoryFake{exact: aggregate}
	got, err := NewPublishedReadService(repository).Exact(context.Background(), aggregate.CourseVersion.CourseID, aggregate.CourseVersion.Version)
	if err != nil {
		t.Fatal(err)
	}

	aggregate.CourseVersion.LearningObjectives[0] = "mutated"
	aggregate.Modules[0].Lessons[0].LearningObjectives[0] = "mutated"
	aggregate.Modules[0].Lessons[0].Content.Blocks[0].Key = "mutated"
	if got.LearningObjectives[0] == "mutated" || got.Modules[0].Lessons[0].LearningObjectives[0] == "mutated" || got.Modules[0].Lessons[0].Content.Blocks[0].Key == "mutated" {
		t.Fatalf("published read retained mutable aggregate references: %#v", got)
	}
}

func TestPublishedReadServiceRejectsWrongOrUnpublishedAggregate(t *testing.T) {
	aggregate := publishedReadAggregate(t)
	service := NewPublishedReadService(&publishedReadRepositoryFake{exact: aggregate})
	other, err := ParseVersion("1.2.4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Exact(context.Background(), aggregate.CourseVersion.CourseID, other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong SemVer = %v, want not found", err)
	}
	aggregate.CourseVersion.Status = CourseVersionArchived
	if _, err := NewPublishedReadService(&publishedReadRepositoryFake{exact: aggregate}).Exact(context.Background(), aggregate.CourseVersion.CourseID, aggregate.CourseVersion.Version); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unpublished aggregate = %v, want not found", err)
	}
}

func TestPublishedReadServiceLatestUsesRepositorySelectedAggregate(t *testing.T) {
	aggregate := publishedReadAggregate(t)
	aggregate.CourseVersion.Version = Version{Major: 1, Minor: 10, Patch: 0}
	repository := &publishedReadRepositoryFake{latest: aggregate}
	got, err := NewPublishedReadService(repository).Latest(context.Background(), aggregate.CourseVersion.CourseID)
	if err != nil || repository.latestCourse != aggregate.CourseVersion.CourseID || got.Version.String() != "1.10.0" {
		t.Fatalf("latest read = %#v, repository=%#v, err=%v", got, repository, err)
	}
}
