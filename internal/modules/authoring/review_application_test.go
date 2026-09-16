package authoring

import (
	"context"
	"errors"
	"testing"
)

type reviewRepositoryFake struct {
	cycle        ReviewCycle
	snapshot     ReviewSnapshot
	history      []ReviewCycle
	err          error
	lastDraft    DraftID
	lastReview   ReviewID
	lastActor    string
	lastStatus   ReviewStatus
	lastExpected int64
}

func (r *reviewRepositoryFake) SubmitReview(_ context.Context, draft DraftID, expected int64, actor string) (ReviewCycle, ReviewSnapshot, error) {
	r.lastDraft, r.lastExpected, r.lastActor = draft, expected, actor
	return r.cycle, r.snapshot, r.err
}
func (r *reviewRepositoryFake) GetReview(context.Context, ReviewID) (ReviewCycle, ReviewSnapshot, error) {
	return r.cycle, r.snapshot, r.err
}
func (r *reviewRepositoryFake) GetReviewForDraft(_ context.Context, draft DraftID, review ReviewID) (ReviewCycle, ReviewSnapshot, error) {
	r.lastDraft, r.lastReview = draft, review
	return r.cycle, r.snapshot, r.err
}
func (r *reviewRepositoryFake) LatestReview(_ context.Context, draft DraftID) (ReviewCycle, ReviewSnapshot, error) {
	r.lastDraft = draft
	return r.cycle, r.snapshot, r.err
}
func (r *reviewRepositoryFake) ActiveReview(_ context.Context, draft DraftID) (ReviewCycle, ReviewSnapshot, error) {
	r.lastDraft = draft
	return r.cycle, r.snapshot, r.err
}
func (r *reviewRepositoryFake) ListReviewHistory(_ context.Context, draft DraftID) ([]ReviewCycle, error) {
	r.lastDraft = draft
	return r.history, r.err
}
func (r *reviewRepositoryFake) DecideReview(context.Context, ReviewID, int64, ReviewStatus, string, string) (ReviewCycle, error) {
	return r.cycle, r.err
}
func (r *reviewRepositoryFake) DecideReviewForDraft(_ context.Context, draft DraftID, review ReviewID, expected int64, status ReviewStatus, actor, _ string) (ReviewCycle, error) {
	r.lastDraft, r.lastReview, r.lastExpected, r.lastStatus, r.lastActor = draft, review, expected, status, actor
	return r.cycle, r.err
}
func (r *reviewRepositoryFake) ListReviewEvents(context.Context, ReviewID) ([]ReviewEvent, error) {
	return nil, r.err
}
func (r *reviewRepositoryFake) ApprovedReviewForRevision(context.Context, DraftID, int64) (ReviewCycle, ReviewSnapshot, error) {
	return r.cycle, r.snapshot, r.err
}

func TestReviewApplicationAuthorizationAndScope(t *testing.T) {
	actor := resolvedActor(t)
	reviewID := ReviewID("44444444-4444-4444-8444-444444444444")
	repository := &reviewRepositoryFake{cycle: ReviewCycle{ID: reviewID, DraftID: testDraftA, Status: ReviewInReview, Revision: 1}}
	memberships := &membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: MemberAuthor}}
	service := NewReviewApplicationService(repository, NewAuthorizationService(memberships))

	if _, _, err := service.Submit(context.Background(), actor, testDraftA, 3); err != nil {
		t.Fatalf("AUTHOR submit: %v", err)
	}
	if repository.lastDraft != testDraftA || repository.lastExpected != 3 || repository.lastActor != string(actor.UserID()) {
		t.Fatal("submit ignored trusted actor or exact Draft")
	}
	if _, _, err := service.Review(context.Background(), actor, testDraftA, reviewID); err != nil || repository.lastReview != reviewID {
		t.Fatalf("AUTHOR read: %v", err)
	}
	if _, err := service.Approve(context.Background(), actor, testDraftA, reviewID, 1, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AUTHOR decision was not hidden: %v", err)
	}
	memberships.roles[testDraftA] = MemberMaintainer
	if _, err := service.Approve(context.Background(), actor, testDraftA, reviewID, 1, "approved"); err != nil {
		t.Fatalf("MAINTAINER decision: %v", err)
	}
	if repository.lastStatus != ReviewApproved || repository.lastExpected != 1 || repository.lastActor != string(actor.UserID()) {
		t.Fatal("decision did not use scoped repository CAS")
	}
	delete(memberships.roles, testDraftA)
	if _, _, err := service.Active(context.Background(), actor, testDraftA); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked membership retained review access: %v", err)
	}
	if memberships.calls != 5 {
		t.Fatalf("review authorization was cached: %d checks", memberships.calls)
	}
}

func TestReviewApplicationAuthorizationFailureIsUnavailable(t *testing.T) {
	actor := resolvedActor(t)
	repository := &reviewRepositoryFake{}
	memberships := &membershipReaderFake{err: errors.New("database down")}
	service := NewReviewApplicationService(repository, NewAuthorizationService(memberships))
	if _, err := service.History(context.Background(), actor, testDraftA); !errors.Is(err, ErrAuthorizationUnavailable) {
		t.Fatalf("infrastructure failure became denial: %v", err)
	}
}

func TestReviewApplicationAcceptsPolicyWithoutEnforcingItBeforeM43b(t *testing.T) {
	actor := resolvedActor(t)
	repository := &reviewRepositoryFake{cycle: ReviewCycle{ID: ReviewID("44444444-4444-4444-8444-444444444444"), DraftID: testDraftA, Status: ReviewInReview, Revision: 1}}
	memberships := &membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: MemberMaintainer}}
	service := NewReviewApplicationServiceWithDecisionPolicy(repository, NewAuthorizationService(memberships), NewReviewDecisionPolicy(true))

	if _, err := service.Approve(context.Background(), actor, testDraftA, repository.cycle.ID, 1, ""); err != nil {
		t.Fatalf("M4.3a policy changed M4.2 decision behavior: %v", err)
	}
}
