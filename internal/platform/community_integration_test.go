//go:build integration

package platform

import (
	"context"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/community"
	communitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/community/postgres"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"testing"
)

func testCourseCommunityPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	cr := coursespostgres.New(pool)
	c, err := cr.CreateCourse(ctx, "community-persistence")
	if err != nil {
		t.Fatal(err)
	}
	u, err := identitypostgres.New(pool).CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	r := communitypostgres.New(pool)
	if _, err = r.CreateCommunity(ctx, string(c.ID), community.CommunityEnabled); err != nil {
		t.Fatal(err)
	}
	thread, opening, err := r.CreateThread(ctx, community.ThreadInput{CourseID: string(c.ID), Title: " Course question ", CreatedByUserID: string(u.ID), OpeningBody: "First line\nsecond line"})
	if err != nil || thread.Title != "Course question" || opening.ThreadID != thread.ID || opening.Body != "First line\nsecond line" {
		t.Fatalf("thread/opening=%#v %#v %v", thread, opening, err)
	}
	if got, err := r.GetPost(ctx, opening.ID); err != nil || got.AuthorUserID != string(u.ID) {
		t.Fatalf("post=%#v %v", got, err)
	}
	other, err := cr.CreateCourse(ctx, "community-other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.CreateCommunity(ctx, string(other.ID), community.CommunityEnabled); err != nil {
		t.Fatal(err)
	}
	if _, _, err = r.CreateThread(ctx, community.ThreadInput{CourseID: string(other.ID), Title: "x", CreatedByUserID: string(u.ID), OpeningBody: "x"}); err != nil {
		t.Fatal(err)
	}
}
