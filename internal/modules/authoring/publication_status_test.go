package authoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type publicationStatusFactsFake struct {
	record PublicationRecord
	err    error
}

func (f *publicationStatusFactsFake) RecordPublication(context.Context, PublicationRecord) (PublicationRecord, error) {
	return PublicationRecord{}, errors.New("not used")
}

func (f *publicationStatusFactsFake) GetPublication(context.Context, ReviewID) (PublicationRecord, error) {
	return f.record, f.err
}

func TestReviewPublicationStatusUsesExactSnapshotAndCapability(t *testing.T) {
	cycle, snapshot := publicationValidationFixture(t)
	memberships := &membershipReaderFake{roles: map[DraftID]MemberRole{cycle.DraftID: MemberMaintainer}}
	facts := &publicationStatusFactsFake{err: ErrNotFound}
	service := NewReviewPublicationStatusService(facts, NewAuthorizationService(memberships))

	status, err := service.Status(context.Background(), resolvedActor(t), cycle, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !status.CanPublish || !status.Publishable || len(status.Issues) != 0 || status.Published != nil {
		t.Fatalf("maintainer status = %#v", status)
	}
	if memberships.lastDraft != cycle.DraftID || memberships.lastUser != string(testActorID) {
		t.Fatalf("capability was not scoped to exact Draft: %#v", memberships)
	}

	memberships.roles[cycle.DraftID] = MemberAuthor
	status, err = service.Status(context.Background(), resolvedActor(t), cycle, snapshot)
	if err != nil || status.CanPublish || !status.Publishable {
		t.Fatalf("author status = %#v, %v", status, err)
	}
}

func TestReviewPublicationStatusReturnsFrozenValidationAndSafePublishedProjection(t *testing.T) {
	cycle, snapshot := publicationValidationFixture(t)
	version, err := courses.ParseVersion(snapshot.Draft.IntendedVersion)
	if err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)
	facts := &publicationStatusFactsFake{record: PublicationRecord{
		ReviewID: cycle.ID, ReviewRevision: cycle.Revision,
		DraftID: cycle.DraftID, DraftRevision: cycle.DraftRevision,
		CourseID: snapshot.Draft.CourseID, CourseVersion: version,
		CourseVersionID: "70000000-0000-4000-8000-000000000001",
		PublishedAt:     publishedAt, PublishedByUserID: "80000000-0000-4000-8000-000000000001",
	}}
	memberships := &membershipReaderFake{roles: map[DraftID]MemberRole{cycle.DraftID: MemberMaintainer}}
	service := NewReviewPublicationStatusService(facts, NewAuthorizationService(memberships))

	status, err := service.Status(context.Background(), resolvedActor(t), cycle, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if status.Published == nil || status.Published.CourseID != snapshot.Draft.CourseID || status.Published.CourseVersion != version || !status.Published.PublishedAt.Equal(publishedAt) {
		t.Fatalf("published projection = %#v", status)
	}

	broken := clonePublicationSnapshot(t, snapshot)
	broken.Draft.Title = ""
	facts.err = ErrNotFound
	status, err = service.Status(context.Background(), resolvedActor(t), cycle, broken)
	if err != nil || status.Publishable || !hasPublicationIssue(PublicationValidationResult{Issues: status.Issues}, PublicationIssueCourseMetadataInvalid) {
		t.Fatalf("frozen validation status = %#v, %v", status, err)
	}

	cycle.Status = ReviewInReview
	status, err = service.Status(context.Background(), resolvedActor(t), cycle, snapshot)
	if err != nil || status.Publishable || !hasPublicationIssue(PublicationValidationResult{Issues: status.Issues}, PublicationIssueReviewNotApproved) {
		t.Fatalf("non-approved status = %#v, %v", status, err)
	}
}

func TestReviewPublicationStatusRejectsInconsistentPublicationFact(t *testing.T) {
	cycle, snapshot := publicationValidationFixture(t)
	facts := &publicationStatusFactsFake{record: PublicationRecord{
		ReviewID: cycle.ID, ReviewRevision: cycle.Revision + 1,
		DraftID: cycle.DraftID, DraftRevision: cycle.DraftRevision,
		CourseID: snapshot.Draft.CourseID, CourseVersionID: "70000000-0000-4000-8000-000000000001",
		PublishedAt: time.Now().UTC(),
	}}
	memberships := &membershipReaderFake{roles: map[DraftID]MemberRole{cycle.DraftID: MemberMaintainer}}
	_, err := NewReviewPublicationStatusService(facts, NewAuthorizationService(memberships)).Status(context.Background(), resolvedActor(t), cycle, snapshot)
	if !errors.Is(err, ErrPublicationConflict) {
		t.Fatalf("inconsistent fact error = %v", err)
	}
}
