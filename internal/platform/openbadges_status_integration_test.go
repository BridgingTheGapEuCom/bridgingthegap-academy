//go:build integration

package platform

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges"
	statuspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges/postgres"
	credentialspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testOpenBadgesStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	courseRepo := coursespostgres.New(pool)
	course, err := courseRepo.CreateCourse(ctx, "status-list-integration")
	if err != nil {
		t.Fatal(err)
	}
	version, err := courses.NewCourseVersionStore(courseRepo).Store(ctx, immutableCourseVersionFixture(t, course.ID, "1.0.0", "c9000000-0000-4000-8000-000000000001"))
	if err != nil {
		t.Fatal(err)
	}
	certRepo := credentialspostgres.New(pool)
	identityRepo := identitypostgres.New(pool)
	issuedAt := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	create := func() credentials.Certificate {
		learner, err := identityRepo.CreateUser(ctx, identity.UserActive)
		if err != nil {
			t.Fatal(err)
		}
		cert, err := certRepo.Create(ctx, certificateInput(t, learner.ID, version, "Complete the course."), issuedAt)
		if err != nil {
			t.Fatal(err)
		}
		return cert
	}
	a, b, c, d := create(), create(), create(), create()
	candidates := []int{42, 42, 43}
	n := 0
	allocator := statuspostgres.New(pool).WithIndexCandidate(func(capacity int) (int, error) { value := candidates[n]; n++; return value, nil })
	first, err := allocator.EnsureEntry(ctx, string(a.ID))
	if err != nil || first.StatusListIndex != 42 {
		t.Fatalf("first allocation: %#v %v", first, err)
	}
	replay, err := allocator.EnsureEntry(ctx, string(a.ID))
	if err != nil || replay != first || n != 1 {
		t.Fatalf("idempotency: %#v %v candidates=%d", replay, err, n)
	}
	second, err := allocator.EnsureEntry(ctx, string(b.ID))
	if err != nil || second.StatusListIndex != 43 || second.StatusListID != first.StatusListID {
		t.Fatalf("collision retry: %#v %v", second, err)
	}
	list, facts, err := statuspostgres.New(pool).Snapshot(ctx, first.StatusListID)
	if err != nil || len(facts) != 2 {
		t.Fatalf("snapshot: %#v %#v %v", list, facts, err)
	}
	before, err := openbadges.BuildEncodedList(list, facts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = certRepo.Revoke(ctx, a.ID, issuedAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	_, facts, err = statuspostgres.New(pool).Snapshot(ctx, first.StatusListID)
	if err != nil {
		t.Fatal(err)
	}
	after, err := openbadges.BuildEncodedList(list, facts)
	if err != nil || before == after {
		t.Fatalf("revocation did not change payload: %v", err)
	}
	stable, err := statuspostgres.New(pool).EnsureEntry(ctx, string(a.ID))
	if err != nil || stable != first {
		t.Fatalf("revocation changed mapping: %#v %v", stable, err)
	}
	var wg sync.WaitGroup
	same := make([]openbadges.StatusListEntry, 2)
	errs := make([]error, 2)
	for i := range same {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			same[i], errs[i] = statuspostgres.New(pool).EnsureEntry(ctx, string(c.ID))
		}(i)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || same[0] != same[1] {
		t.Fatalf("same Certificate race: %#v %v", same, errs)
	}
	different := []credentials.Certificate{d, create()}
	distinct := make([]openbadges.StatusListEntry, 2)
	distinctErrors := make([]error, 2)
	for i := range different {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			distinct[i], distinctErrors[i] = statuspostgres.New(pool).EnsureEntry(ctx, string(different[i].ID))
		}(i)
	}
	wg.Wait()
	if distinctErrors[0] != nil || distinctErrors[1] != nil || (distinct[0].StatusListID == distinct[1].StatusListID && distinct[0].StatusListIndex == distinct[1].StatusListIndex) {
		t.Fatalf("different Certificate race: %#v %v", distinct, distinctErrors)
	}
	fallbackCert := create()
	fallback, err := statuspostgres.New(pool).WithIndexCandidate(func(int) (int, error) { return 42, nil }).EnsureEntry(ctx, string(fallbackCert.ID))
	if err != nil || fallback.StatusListIndex == 42 {
		t.Fatalf("bounded collision fallback: %#v %v", fallback, err)
	}
	// A full allocation count rolls over without altering existing positions.
	if _, err = pool.Exec(ctx, `UPDATE open_badges.status_list SET next_index=capacity WHERE id=$1`, first.StatusListID); err != nil {
		t.Fatal(err)
	}
	e := create()
	next, err := statuspostgres.New(pool).WithIndexCandidate(func(int) (int, error) { return 99, nil }).EnsureEntry(ctx, string(e.ID))
	if err != nil || next.StatusListID == first.StatusListID || next.StatusListIndex != 99 {
		t.Fatalf("rollover: %#v %v", next, err)
	}
	stable, err = statuspostgres.New(pool).EnsureEntry(ctx, string(a.ID))
	if err != nil || stable != first {
		t.Fatalf("existing mapping changed at rollover: %#v %v", stable, err)
	}
}
