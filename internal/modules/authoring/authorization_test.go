package authoring

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

const (
	testDraftA  DraftID         = "11111111-1111-4111-8111-111111111111"
	testDraftB  DraftID         = "22222222-2222-4222-8222-222222222222"
	testActorID identity.UserID = "33333333-3333-4333-8333-333333333333"
)

// A real SessionService resolution creates the otherwise unforgeable actor.
type actorSessionRepository struct {
	user    identity.User
	session identity.Session
}

func (r actorSessionRepository) GetUser(_ context.Context, id identity.UserID) (identity.User, error) {
	if id != r.user.ID {
		return identity.User{}, identity.ErrNotFound
	}
	return r.user, nil
}
func (r actorSessionRepository) CreateSession(context.Context, identity.UserID, identity.SessionTokenDigest, time.Time, identity.CSRFToken) (identity.Session, error) {
	return identity.Session{}, errors.New("not used")
}
func (r actorSessionRepository) GetSessionByDigest(_ context.Context, digest identity.SessionTokenDigest) (identity.Session, error) {
	if string(digest.Bytes()) != string(r.session.TokenDigest.Bytes()) {
		return identity.Session{}, identity.ErrNotFound
	}
	return r.session, nil
}
func (r actorSessionRepository) GetSessionByID(context.Context, identity.SessionID) (identity.Session, error) {
	return identity.Session{}, errors.New("not used")
}
func (r actorSessionRepository) RevokeSession(context.Context, identity.SessionID) (identity.Session, error) {
	return identity.Session{}, errors.New("not used")
}
func (r actorSessionRepository) RevokeUserSessions(context.Context, identity.UserID) (int64, error) {
	return 0, errors.New("not used")
}

func resolvedActor(t *testing.T) identity.AuthenticatedActor {
	t.Helper()
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	raw := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	sum := sha256.Sum256([]byte(raw))
	digest, err := identity.NewSessionTokenDigest(sum[:])
	if err != nil {
		t.Fatal(err)
	}
	repo := actorSessionRepository{
		user:    identity.User{ID: testActorID, Status: identity.UserActive},
		session: identity.Session{ID: "session-1", UserID: testActorID, TokenDigest: digest, CreatedAt: now.Add(-time.Minute), LastSeenAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)},
	}
	service := identity.NewSessionService(repo, nil, func() time.Time { return now })
	resolved, err := service.ResolveSession(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	actor, err := identity.ActorFromResolvedSession(resolved)
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

type membershipReaderFake struct {
	roles     map[DraftID]MemberRole
	err       error
	calls     int
	lastDraft DraftID
	lastUser  string
}

func (f *membershipReaderFake) ActiveMembershipForDraft(_ context.Context, draft DraftID, user string) (MemberRole, bool, error) {
	f.calls++
	f.lastDraft, f.lastUser = draft, user
	if f.err != nil {
		return "", false, f.err
	}
	role, found := f.roles[draft]
	return role, found, nil
}

func TestAuthoringCapabilityMatrix(t *testing.T) {
	capabilities := []Capability{CapabilityRead, CapabilityDraftEdit, CapabilityStructureEdit, CapabilityContentEdit, CapabilityAssetUpload, CapabilityMembersManage, CapabilityDraftAbandon, CapabilityReviewRead, CapabilityReviewSubmit, CapabilityReviewDecide, CapabilityPublish}
	for _, tc := range []struct {
		role    MemberRole
		allowed []bool
	}{
		{MemberAuthor, []bool{true, true, true, true, true, false, false, true, true, false, false}},
		{MemberMaintainer, []bool{true, true, true, true, true, true, true, true, true, true, true}},
	} {
		for i, capability := range capabilities {
			if got := roleAllows(tc.role, capability); got != tc.allowed[i] {
				t.Errorf("role %s capability %s: got %t", tc.role, capability, got)
			}
		}
	}
	if roleAllows(MemberRole("ADMINISTRATOR"), CapabilityAssetUpload) || roleAllows(MemberRole("ADMINISTRATOR"), CapabilityPublish) || roleAllows(MemberMaintainer, Capability("unknown")) {
		t.Fatal("unknown role or future capability granted access")
	}
}

func TestAuthoringAuthorizationIsScopedAndCurrent(t *testing.T) {
	actor := resolvedActor(t)
	reader := &membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: MemberAuthor}}
	service := NewAuthorizationService(reader)
	ctx := context.Background()
	if err := service.Authorize(ctx, actor, CapabilityContentEdit, DraftResource(testDraftA)); err != nil {
		t.Fatal(err)
	}
	if reader.lastDraft != testDraftA || reader.lastUser != string(testActorID) {
		t.Fatal("membership lookup ignored trusted actor or exact draft")
	}
	if err := service.Authorize(ctx, actor, CapabilityContentEdit, DraftResource(testDraftB)); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("other draft allowed: %v", err)
	}
	if err := service.Authorize(ctx, actor, CapabilityMembersManage, DraftResource(testDraftA)); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("author managed members: %v", err)
	}
	reader.roles[testDraftA] = MemberMaintainer
	if err := service.Authorize(ctx, actor, CapabilityMembersManage, DraftResource(testDraftA)); err != nil {
		t.Fatal(err)
	}
	if err := service.Authorize(ctx, actor, CapabilityMembersManage, DraftResource(testDraftB)); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("maintainer crossed draft boundary: %v", err)
	}
	delete(reader.roles, testDraftA)
	if err := service.Authorize(ctx, actor, CapabilityRead, DraftResource(testDraftA)); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("revoked membership remained authorized: %v", err)
	}
	if reader.calls != 6 {
		t.Fatalf("membership was cached across decisions: %d lookups", reader.calls)
	}
}

func TestAuthoringAuthorizationFailsClosed(t *testing.T) {
	actor := resolvedActor(t)
	reader := &membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: MemberMaintainer}}
	service := NewAuthorizationService(reader)
	ctx := context.Background()
	for _, tc := range []struct {
		actor      identity.AuthenticatedActor
		capability Capability
		resource   Resource
	}{
		{identity.AuthenticatedActor{}, CapabilityRead, DraftResource(testDraftA)},
		{actor, Capability("unknown"), DraftResource(testDraftA)},
		{actor, CapabilityRead, Resource{}},
		{actor, CapabilityRead, DraftResource("not-a-uuid")},
	} {
		if err := service.Authorize(ctx, tc.actor, tc.capability, tc.resource); !errors.Is(err, ErrAuthorizationDenied) {
			t.Fatalf("untrusted request allowed: %v", err)
		}
	}
	if reader.calls != 0 {
		t.Fatal("invalid actor, resource, or capability reached repository")
	}
	privateFailure := errors.New("private database detail")
	reader.err = privateFailure
	if err := service.Authorize(ctx, actor, CapabilityRead, DraftResource(testDraftA)); !errors.Is(err, ErrAuthorizationUnavailable) || errors.Is(err, privateFailure) {
		t.Fatalf("repository failure not safely classified: %v", err)
	}
	reader.err = nil
	reader.roles[testDraftA] = MemberRole("BROKEN")
	if err := service.Authorize(ctx, actor, CapabilityRead, DraftResource(testDraftA)); !errors.Is(err, ErrAuthorizationUnavailable) {
		t.Fatalf("corrupt role accepted: %v", err)
	}
	if err := NewAuthorizationService(nil).Authorize(ctx, actor, CapabilityRead, DraftResource(testDraftA)); !errors.Is(err, ErrAuthorizationUnavailable) {
		t.Fatalf("missing reader accepted: %v", err)
	}
}
