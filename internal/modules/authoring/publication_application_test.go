package authoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type reviewPublisherFake struct {
	command PublishReviewCommand
	result  PublicationResult
	err     error
	calls   int
}

func (f *reviewPublisherFake) Publish(_ context.Context, command PublishReviewCommand) (PublicationResult, error) {
	f.calls++
	f.command = command
	return f.result, f.err
}

func TestPublicationApplicationAuthorizesAndSuppliesTrustedMetadata(t *testing.T) {
	actor := resolvedActor(t)
	at := time.Date(2026, time.September, 16, 18, 30, 0, 0, time.FixedZone("test", 2*60*60))
	publisher := &reviewPublisherFake{result: PublicationResult{ReviewID: "22222222-2222-4222-8222-222222222222", CourseVersion: courses.Version{Major: 1}}}
	memberships := &membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: MemberMaintainer}}
	service := NewPublicationApplicationService(publisher, NewAuthorizationService(memberships), func() time.Time { return at })

	_, err := service.Publish(context.Background(), actor, testDraftA, "22222222-2222-4222-8222-222222222222", 2)
	if err != nil {
		t.Fatal(err)
	}
	if publisher.calls != 1 || publisher.command.DraftID != testDraftA || publisher.command.ReviewID != "22222222-2222-4222-8222-222222222222" || publisher.command.ExpectedReviewRevision != 2 {
		t.Fatalf("publication command = %#v", publisher.command)
	}
	if publisher.command.PublishedByUserID != string(actor.UserID()) || !publisher.command.PublishedAt.Equal(at.UTC()) || publisher.command.Attribution != nil {
		t.Fatalf("untrusted publication metadata = %#v", publisher.command)
	}
}

func TestPublicationApplicationAuthorizationPrecedesPublisher(t *testing.T) {
	actor := resolvedActor(t)
	for _, test := range []struct {
		name string
		role MemberRole
		err  error
		want error
	}{
		{name: "author denied", role: MemberAuthor, want: ErrNotFound},
		{name: "revoked hidden", want: ErrNotFound},
		{name: "authorization unavailable", role: MemberMaintainer, err: errors.New("private storage detail"), want: ErrAuthorizationUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			publisher := &reviewPublisherFake{}
			roles := map[DraftID]MemberRole{}
			if test.role != "" {
				roles[testDraftA] = test.role
			}
			service := NewPublicationApplicationService(publisher, NewAuthorizationService(&membershipReaderFake{roles: roles, err: test.err}), time.Now)
			_, err := service.Publish(context.Background(), actor, testDraftA, "22222222-2222-4222-8222-222222222222", 1)
			if !errors.Is(err, test.want) || publisher.calls != 0 {
				t.Fatalf("error=%v calls=%d", err, publisher.calls)
			}
		})
	}
}

func TestPublicationApplicationPreservesOrchestrationErrors(t *testing.T) {
	actor := resolvedActor(t)
	publisher := &reviewPublisherFake{err: courses.ErrCourseVersionAlreadyExists}
	service := NewPublicationApplicationService(publisher, NewAuthorizationService(&membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: MemberMaintainer}}), time.Now)
	_, err := service.Publish(context.Background(), actor, testDraftA, "22222222-2222-4222-8222-222222222222", 1)
	if !errors.Is(err, courses.ErrCourseVersionAlreadyExists) {
		t.Fatalf("publication error = %v", err)
	}
}
