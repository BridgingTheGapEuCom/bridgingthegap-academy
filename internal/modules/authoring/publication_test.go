package authoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type publicationCourseStoreFake struct {
	stored, byReview, byVersion           courses.ImmutableCourseVersion
	storeErr, reviewErr, versionErr       error
	storeCalls, reviewCalls, versionCalls int
}

func (f *publicationCourseStoreFake) Store(_ context.Context, value courses.ImmutableCourseVersion) (courses.ImmutableCourseVersion, error) {
	f.storeCalls++
	f.stored = value
	if f.storeErr != nil {
		return courses.ImmutableCourseVersion{}, f.storeErr
	}
	return storedPublication(value), nil
}
func (f *publicationCourseStoreFake) GetByReviewID(context.Context, string) (courses.ImmutableCourseVersion, error) {
	f.reviewCalls++
	return f.byReview, f.reviewErr
}
func (f *publicationCourseStoreFake) GetByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error) {
	f.versionCalls++
	return f.byVersion, f.versionErr
}

type publicationRecordRepositoryFake struct {
	record authoringPublicationRecordAlias
	err    error
	calls  int
}

// Alias prevents accidental method/field name ambiguity in terse fake code.
type authoringPublicationRecordAlias = PublicationRecord

func (f *publicationRecordRepositoryFake) RecordPublication(_ context.Context, record PublicationRecord) (PublicationRecord, error) {
	f.calls++
	f.record = record
	if f.err != nil {
		return PublicationRecord{}, f.err
	}
	record.RecordedAt = time.Date(2026, time.September, 16, 12, 1, 0, 0, time.UTC)
	return record, nil
}
func (f *publicationRecordRepositoryFake) GetPublication(context.Context, ReviewID) (PublicationRecord, error) {
	return f.record, f.err
}

func publicationCommand(cycle ReviewCycle, metadata PublicationConversionMetadata) PublishReviewCommand {
	return PublishReviewCommand{
		DraftID: cycle.DraftID, ReviewID: cycle.ID, ExpectedReviewRevision: cycle.Revision,
		PublishedByUserID: metadata.PublishedByUserID, PublishedAt: metadata.PublishedAt,
		Attribution: metadata.Attribution,
	}
}

func publicationServiceFixture(t *testing.T) (*reviewRepositoryFake, *publicationCourseStoreFake, *publicationRecordRepositoryFake, PublishReviewCommand) {
	t.Helper()
	cycle, snapshot, metadata := publicationConversionFixture(t)
	cycle.SubmittedByUserID = "60000000-0000-4000-8000-000000000001"
	reviews := &reviewRepositoryFake{cycle: cycle, snapshot: snapshot}
	versions := &publicationCourseStoreFake{}
	publications := &publicationRecordRepositoryFake{}
	return reviews, versions, publications, publicationCommand(cycle, metadata)
}

func storedPublication(value courses.ImmutableCourseVersion) courses.ImmutableCourseVersion {
	value.ID = "61000000-0000-4000-8000-000000000001"
	for moduleIndex := range value.Modules {
		value.Modules[moduleIndex].ID = courses.ModuleID("62000000-0000-4000-8000-00000000000" + string(rune('1'+moduleIndex)))
		for lessonIndex := range value.Modules[moduleIndex].Lessons {
			value.Modules[moduleIndex].Lessons[lessonIndex].ID = courses.LessonID("63000000-0000-4000-8000-0000000000" + string(rune('1'+moduleIndex)) + string(rune('1'+lessonIndex)))
		}
	}
	return value
}

func TestPublicationServicePublishesExactApprovedReview(t *testing.T) {
	reviews, versions, publications, command := publicationServiceFixture(t)
	result, err := NewPublicationService(reviews, versions, publications).Publish(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if reviews.getReviewCalls != 1 || reviews.lastDraft != command.DraftID || reviews.lastReview != command.ReviewID || versions.storeCalls != 1 || publications.calls != 1 {
		t.Fatalf("boundary calls: reviews=%d store=%d record=%d", reviews.getReviewCalls, versions.storeCalls, publications.calls)
	}
	if result.ReviewID != command.ReviewID || result.ReviewRevision != command.ExpectedReviewRevision || result.CourseVersionID == "" || result.Reconciled {
		t.Fatalf("publication result = %#v", result)
	}
	if publications.record.PublishedByUserID != command.PublishedByUserID || !publications.record.PublishedAt.Equal(command.PublishedAt) || publications.record.CourseVersionID != result.CourseVersionID {
		t.Fatalf("publication fact = %#v", publications.record)
	}
}

func TestPublicationServiceUsesSafeFallbackAttribution(t *testing.T) {
	reviews, versions, publications, command := publicationServiceFixture(t)
	command.Attribution = nil
	if _, err := NewPublicationService(reviews, versions, publications).Publish(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if len(versions.stored.CourseVersion.Attribution) != 1 {
		t.Fatalf("fallback attribution = %#v", versions.stored.CourseVersion.Attribution)
	}
	contributor := versions.stored.CourseVersion.Attribution[0]
	if contributor.UserID != reviews.cycle.SubmittedByUserID || contributor.DisplayName != "Author" || contributor.Role != courses.ContributorAuthor || contributor.Order != 0 {
		t.Fatalf("fallback attribution exposed opaque source identity: %#v", contributor)
	}
}

func TestPublicationServiceRejectsStaleStateAndInvalidSnapshotBeforeCourses(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*reviewRepositoryFake, *PublishReviewCommand)
		want   error
	}{
		{"stale", func(_ *reviewRepositoryFake, c *PublishReviewCommand) { c.ExpectedReviewRevision++ }, ErrReviewStale},
		{"not approved", func(r *reviewRepositoryFake, _ *PublishReviewCommand) { r.cycle.Status = ReviewChangesRequested }, ErrReviewInvalidState},
		{"invalid snapshot", func(r *reviewRepositoryFake, _ *PublishReviewCommand) { r.snapshot.Draft.Title = "" }, ErrPublicationValidationFailed},
		{"conversion input", func(_ *reviewRepositoryFake, c *PublishReviewCommand) {
			c.Attribution = []courses.ContributorSnapshot{}
		}, ErrPublicationConversionInput},
	} {
		t.Run(test.name, func(t *testing.T) {
			reviews, versions, publications, command := publicationServiceFixture(t)
			test.mutate(reviews, &command)
			_, err := NewPublicationService(reviews, versions, publications).Publish(context.Background(), command)
			if !errors.Is(err, test.want) || versions.storeCalls != 0 || publications.calls != 0 {
				t.Fatalf("result error=%v stores=%d records=%d", err, versions.storeCalls, publications.calls)
			}
		})
	}
}

func TestPublicationServiceReconcilesOnlyExactReviewReplay(t *testing.T) {
	reviews, versions, publications, command := publicationServiceFixture(t)
	service := NewPublicationService(reviews, versions, publications)
	candidate, err := service.converter.Convert(reviews.cycle, &reviews.snapshot, PublicationConversionMetadata{PublishedAt: command.PublishedAt, PublishedByUserID: command.PublishedByUserID, Attribution: command.Attribution})
	if err != nil {
		t.Fatal(err)
	}
	versions.storeErr = courses.ErrCourseVersionAlreadyExists
	versions.byReview = storedPublication(candidate)
	result, err := service.Publish(context.Background(), command)
	if err != nil || !result.Reconciled || versions.reviewCalls != 1 || publications.calls != 1 {
		t.Fatalf("exact replay result=%#v error=%v", result, err)
	}

	reviews, versions, publications, command = publicationServiceFixture(t)
	versions.storeErr = courses.ErrCourseVersionAlreadyExists
	versions.byReview = storedPublication(candidate)
	versions.byReview.CourseVersion.Title = "different persisted publication"
	_, err = NewPublicationService(reviews, versions, publications).Publish(context.Background(), command)
	if !errors.Is(err, ErrPublicationProvenanceMismatch) || publications.calls != 0 {
		t.Fatalf("mismatched replay error=%v records=%d", err, publications.calls)
	}

	reviews, versions, publications, command = publicationServiceFixture(t)
	versions.storeErr = courses.ErrCourseVersionAlreadyExists
	versions.reviewErr = courses.ErrNotFound
	versions.byVersion = storedPublication(candidate)
	_, err = NewPublicationService(reviews, versions, publications).Publish(context.Background(), command)
	if !errors.Is(err, courses.ErrCourseVersionAlreadyExists) || versions.versionCalls != 1 || publications.calls != 0 {
		t.Fatalf("foreign version conflict=%v versionLookups=%d records=%d", err, versions.versionCalls, publications.calls)
	}
}

func TestPublicationServiceLeavesCoursesIntactWhenFactWriteFails(t *testing.T) {
	reviews, versions, publications, command := publicationServiceFixture(t)
	publications.err = errors.New("authoring unavailable")
	service := NewPublicationService(reviews, versions, publications)
	if _, err := service.Publish(context.Background(), command); err == nil || versions.storeCalls != 1 {
		t.Fatalf("first publication error=%v stores=%d", err, versions.storeCalls)
	}
	versions.storeErr = courses.ErrCourseVersionAlreadyExists
	versions.byReview = storedPublication(versions.stored)
	publications.err = nil
	result, err := service.Publish(context.Background(), command)
	if err != nil || !result.Reconciled || versions.storeCalls != 2 || publications.calls != 2 {
		t.Fatalf("recovery result=%#v error=%v stores=%d records=%d", result, err, versions.storeCalls, publications.calls)
	}
}
