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
	return ImmutableCourseVersion{
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
