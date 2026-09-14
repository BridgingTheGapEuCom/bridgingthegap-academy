//go:build integration

package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAuthoringAuthorization(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	course, err := courseRepository.CreateCourse(ctx, "authoring-policy-course")
	if err != nil {
		t.Fatal(err)
	}
	memberRepository := authoringpostgres.New(pool)
	// The creator ID has no Identity row; workspace membership has no Identity FK.
	creator := "44444444-4444-4444-8444-444444444444"
	draftA, workspaceA, err := memberRepository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), creator)
	if err != nil {
		t.Fatal(err)
	}
	draftB, _, err := memberRepository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), creator)
	if err != nil {
		t.Fatal(err)
	}
	identityRepository := identitypostgres.New(pool)
	user, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	sessions := identity.NewSessionService(identityRepository, nil, nil)
	created, err := sessions.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := sessions.ResolveSession(ctx, created.Token.Value())
	if err != nil {
		t.Fatal(err)
	}
	actor, err := identity.ActorFromResolvedSession(resolved)
	if err != nil {
		t.Fatal(err)
	}
	authorizer := authoring.NewAuthorizationService(memberRepository)
	can := func(capability authoring.Capability, draft authoring.DraftID) error {
		return authorizer.Authorize(ctx, actor, capability, authoring.DraftResource(draft))
	}
	if err := can(authoring.CapabilityDraftEdit, draftA.ID); !errors.Is(err, authoring.ErrAuthorizationDenied) {
		t.Fatalf("nonmember accessed draft: %v", err)
	}
	if _, err := memberRepository.AddMember(ctx, workspaceA.ID, string(user.ID), authoring.MemberAuthor); err != nil {
		t.Fatal(err)
	}
	if err := can(authoring.CapabilityDraftEdit, draftA.ID); err != nil {
		t.Fatalf("author could not edit own draft: %v", err)
	}
	if err := can(authoring.CapabilityMembersManage, draftA.ID); !errors.Is(err, authoring.ErrAuthorizationDenied) {
		t.Fatalf("author managed members: %v", err)
	}
	if err := can(authoring.CapabilityDraftEdit, draftB.ID); !errors.Is(err, authoring.ErrAuthorizationDenied) {
		t.Fatalf("author accessed another draft: %v", err)
	}
	if _, err := identityRepository.AssignGlobalRole(ctx, user.ID, identity.RoleAdministrator, nil); err != nil {
		t.Fatal(err)
	}
	if err := can(authoring.CapabilityDraftEdit, draftB.ID); !errors.Is(err, authoring.ErrAuthorizationDenied) {
		t.Fatalf("global administrator bypassed draft membership: %v", err)
	}
	if _, err := memberRepository.RevokeMember(ctx, workspaceA.ID, string(user.ID)); err != nil {
		t.Fatal(err)
	}
	if err := can(authoring.CapabilityDraftEdit, draftA.ID); !errors.Is(err, authoring.ErrAuthorizationDenied) {
		t.Fatalf("revoked author retained access on same session: %v", err)
	}
	if _, err := memberRepository.AddMember(ctx, workspaceA.ID, string(user.ID), authoring.MemberMaintainer); err != nil {
		t.Fatal(err)
	}
	if err := can(authoring.CapabilityMembersManage, draftA.ID); err != nil {
		t.Fatalf("maintainer could not manage members: %v", err)
	}
	if err := can(authoring.CapabilityMembersManage, draftB.ID); !errors.Is(err, authoring.ErrAuthorizationDenied) {
		t.Fatalf("maintainer crossed draft boundary: %v", err)
	}
	if _, err := memberRepository.RevokeMember(ctx, workspaceA.ID, string(user.ID)); err != nil {
		t.Fatal(err)
	}
	if err := can(authoring.CapabilityRead, draftA.ID); !errors.Is(err, authoring.ErrAuthorizationDenied) {
		t.Fatalf("revoked maintainer retained access: %v", err)
	}
	if err := can(authoring.CapabilityRead, authoring.DraftID("55555555-5555-4555-8555-555555555555")); !errors.Is(err, authoring.ErrAuthorizationDenied) {
		t.Fatalf("missing resource revealed a distinct authorization result: %v", err)
	}
}
