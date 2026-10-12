//go:build integration

package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/progress"
	progresspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/progress/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testLearnerProgressAPI(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	store := courses.NewCourseVersionStore(courseRepository)
	course, err := courseRepository.CreateCourse(ctx, "learner-progress-api")
	if err != nil {
		t.Fatal(err)
	}
	published, err := store.Store(ctx, immutableCourseVersionFixture(t, course.ID, "1.0.0", "a2000000-0000-4000-8000-000000000001"))
	if err != nil {
		t.Fatal(err)
	}
	identities := identitypostgres.New(pool)
	learner, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	sessions := identity.NewSessionService(identities, nil, nil)
	session, err := sessions.CreateSession(ctx, learner.ID)
	if err != nil {
		t.Fatal(err)
	}
	repository := progresspostgres.New(pool)
	router := authTestRouter(&authHTTP{sessions: sessions, learnerProgress: progress.NewService(repository, courseRepository)})
	base := "/api/courses/by-id/" + string(course.ID) + "/versions/1.0.0/progress"

	first := authRequest(router, http.MethodGet, base, "", &http.Cookie{Name: sessionCookieName, Value: session.Token.Value()})
	var value learnerProgressDTO
	if first.Code != http.StatusOK || json.Unmarshal(first.Body.Bytes(), &value) != nil || value.Revision != 1 || len(value.CompletedLessonKeys) != 0 || !strings.Contains(first.Body.String(), `"completedLessonKeys":[]`) {
		t.Fatalf("first progress=%d %#v body=%s", first.Code, value, first.Body.String())
	}
	updated := authRequest(router, http.MethodPut, base+"/lessons/introduction", `{"expectedRevision":1}`, &http.Cookie{Name: sessionCookieName, Value: session.Token.Value()}, authTestCSRFToken().Value())
	if updated.Code != http.StatusOK || json.Unmarshal(updated.Body.Bytes(), &value) != nil || value.Revision != 2 || len(value.CompletedLessonKeys) != 1 || value.CompletedLessonKeys[0] != "introduction" {
		t.Fatalf("updated progress=%d %#v body=%s", updated.Code, value, updated.Body.String())
	}
	persisted, err := repository.Get(ctx, string(learner.ID), string(published.ID))
	if err != nil || persisted.Revision != 2 || len(persisted.CompletedLessonKeys) != 1 || persisted.CompletedLessonKeys[0] != "introduction" {
		t.Fatalf("persisted progress=%#v err=%v", persisted, err)
	}
	stale := authRequest(router, http.MethodPut, base+"/lessons/transactions", `{"expectedRevision":1}`, &http.Cookie{Name: sessionCookieName, Value: session.Token.Value()}, authTestCSRFToken().Value())
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale progress=%d body=%s", stale.Code, stale.Body.String())
	}
}
