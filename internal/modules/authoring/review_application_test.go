package authoring

import (
	"context"
	"errors"
	"testing"
)

type reviewRepositoryFake struct {
	cycle          ReviewCycle
	snapshot       ReviewSnapshot
	history        []ReviewCycle
	err            error
	lastDraft      DraftID
	lastReview     ReviewID
	lastActor      string
	lastStatus     ReviewStatus
	lastExpected   int64
	decisionCalls  int
	getReviewCalls int
}

func (r *reviewRepositoryFake) SubmitReview(_ context.Context, draft DraftID, expected int64, actor string) (ReviewCycle, ReviewSnapshot, error) {
	r.lastDraft, r.lastExpected, r.lastActor = draft, expected, actor
	return r.cycle, r.snapshot, r.err
}
func (r *reviewRepositoryFake) GetReview(context.Context, ReviewID) (ReviewCycle, ReviewSnapshot, error) {
	return r.cycle, r.snapshot, r.err
}
func (r *reviewRepositoryFake) GetReviewForDraft(_ context.Context, draft DraftID, review ReviewID) (ReviewCycle, ReviewSnapshot, error) {
	r.getReviewCalls++
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
	r.decisionCalls++
	r.lastDraft, r.lastReview, r.lastExpected, r.lastStatus, r.lastActor = draft, review, expected, status, actor
	if r.err == nil {
		r.cycle.Status = status
		r.cycle.Revision = expected + 1
	}
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
	repository := &reviewRepositoryFake{cycle: ReviewCycle{ID: reviewID, DraftID: testDraftA, Status: ReviewInReview, Revision: 1, SubmittedByUserID: "different-user"}}
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

func TestReviewApplicationEnforcesDecisionPolicyForBothDecisions(t *testing.T) {
	actor := resolvedActor(t)
	actorID := string(actor.UserID())
	reviewID := ReviewID("44444444-4444-4444-8444-444444444444")
	for _, test := range []struct {
		name        string
		decision    ReviewStatus
		required    bool
		submitter   string
		wantErr     error
		wantPersist bool
	}{
		{"approve rejects submitter when enabled", ReviewApproved, true, actorID, ErrIndependentReviewerRequired, false},
		{"approve allows independent actor", ReviewApproved, true, "other-user", nil, true},
		{"approve allows submitter when disabled", ReviewApproved, false, actorID, nil, true},
		{"request changes rejects submitter when enabled", ReviewChangesRequested, true, actorID, ErrIndependentReviewerRequired, false},
		{"request changes allows independent actor", ReviewChangesRequested, true, "other-user", nil, true},
		{"request changes allows submitter when disabled", ReviewChangesRequested, false, actorID, nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &reviewRepositoryFake{cycle: ReviewCycle{ID: reviewID, DraftID: testDraftA, Status: ReviewInReview, Revision: 1, SubmittedByUserID: test.submitter}}
			memberships := &membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: MemberMaintainer}}
			service := NewReviewApplicationServiceWithDecisionPolicy(repository, NewAuthorizationService(memberships), NewReviewDecisionPolicy(test.required))

			var err error
			if test.decision == ReviewApproved {
				_, err = service.Approve(context.Background(), actor, testDraftA, reviewID, 1, "")
			} else {
				_, err = service.RequestChanges(context.Background(), actor, testDraftA, reviewID, 1, "")
			}
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("decision error = %v, want %v", err, test.wantErr)
			}
			if repository.getReviewCalls != 1 || (repository.decisionCalls == 1) != test.wantPersist {
				t.Fatalf("reads=%d decision writes=%d", repository.getReviewCalls, repository.decisionCalls)
			}
			if !test.wantPersist && (repository.cycle.Status != ReviewInReview || repository.cycle.Revision != 1) {
				t.Fatalf("policy rejection mutated Review: %#v", repository.cycle)
			}
		})
	}
}

func TestReviewApplicationChecksAuthorizationAndMutableStateBeforePolicy(t *testing.T) {
	actor := resolvedActor(t)
	actorID := string(actor.UserID())
	reviewID := ReviewID("44444444-4444-4444-8444-444444444444")
	repository := &reviewRepositoryFake{cycle: ReviewCycle{ID: reviewID, DraftID: testDraftA, Status: ReviewInReview, Revision: 1, SubmittedByUserID: actorID}}
	memberships := &membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: MemberAuthor}}
	service := NewReviewApplicationServiceWithDecisionPolicy(repository, NewAuthorizationService(memberships), NewReviewDecisionPolicy(true))

	if _, err := service.Approve(context.Background(), actor, testDraftA, reviewID, 1, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("authorization was masked by policy: %v", err)
	}
	if repository.getReviewCalls != 0 || repository.decisionCalls != 0 {
		t.Fatal("denied authorization reached Review persistence")
	}

	memberships.roles[testDraftA] = MemberMaintainer
	if _, err := service.Approve(context.Background(), actor, testDraftA, reviewID, 2, ""); !errors.Is(err, ErrReviewStale) {
		t.Fatalf("stale state did not win before policy: %v", err)
	}
	if repository.decisionCalls != 0 {
		t.Fatal("stale decision reached mutation")
	}

	repository.cycle.Status = ReviewApproved
	if _, err := service.RequestChanges(context.Background(), actor, testDraftA, reviewID, 1, ""); !errors.Is(err, ErrReviewInvalidState) {
		t.Fatalf("terminal state did not win before policy: %v", err)
	}
	if repository.decisionCalls != 0 {
		t.Fatal("terminal decision reached mutation")
	}

	repository.cycle.Status = ReviewInReview
	readsBeforeDenial := repository.getReviewCalls
	delete(memberships.roles, testDraftA)
	if _, err := service.Approve(context.Background(), actor, testDraftA, reviewID, 1, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked decision was not denied before policy: %v", err)
	}
	if repository.getReviewCalls != readsBeforeDenial {
		t.Fatal("revoked decision loaded Review provenance")
	}
	memberships.err = errors.New("authorization storage unavailable")
	if _, err := service.Approve(context.Background(), actor, testDraftA, reviewID, 1, ""); !errors.Is(err, ErrAuthorizationUnavailable) {
		t.Fatalf("authorization failure was masked by policy: %v", err)
	}
	if repository.getReviewCalls != readsBeforeDenial {
		t.Fatal("authorization failure reached Review persistence")
	}
}

func TestReviewApplicationStaleAndTerminalStatePrecedePolicyForBothDecisions(t *testing.T) {
	actor := resolvedActor(t)
	actorID := string(actor.UserID())
	reviewID := ReviewID("44444444-4444-4444-8444-444444444444")
	for _, test := range []struct {
		name      string
		status    ReviewStatus
		expected  int64
		wantError error
	}{
		{name: "stale", status: ReviewInReview, expected: 2, wantError: ErrReviewStale},
		{name: "terminal", status: ReviewApproved, expected: 1, wantError: ErrReviewInvalidState},
	} {
		for _, decision := range []struct {
			name string
			call func(*ReviewApplicationService) error
		}{
			{name: "approve", call: func(service *ReviewApplicationService) error {
				_, err := service.Approve(context.Background(), actor, testDraftA, reviewID, test.expected, "")
				return err
			}},
			{name: "request changes", call: func(service *ReviewApplicationService) error {
				_, err := service.RequestChanges(context.Background(), actor, testDraftA, reviewID, test.expected, "")
				return err
			}},
		} {
			t.Run(test.name+" "+decision.name, func(t *testing.T) {
				repository := &reviewRepositoryFake{cycle: ReviewCycle{ID: reviewID, DraftID: testDraftA, Status: test.status, Revision: 1, SubmittedByUserID: actorID}}
				memberships := &membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: MemberMaintainer}}
				service := NewReviewApplicationServiceWithDecisionPolicy(repository, NewAuthorizationService(memberships), NewReviewDecisionPolicy(true))

				if err := decision.call(service); !errors.Is(err, test.wantError) {
					t.Fatalf("decision error = %v, want %v", err, test.wantError)
				}
				if repository.decisionCalls != 0 || repository.cycle.Revision != 1 || repository.cycle.Status != test.status {
					t.Fatalf("pre-policy conflict reached persistence mutation: %#v", repository)
				}
			})
		}
	}
}
