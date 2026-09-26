//go:build integration

package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAuthoringDraftCreation(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	identityRepository := identitypostgres.New(pool)
	user, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	sessions := identity.NewSessionService(identityRepository, nil, nil)
	session, err := sessions.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	repository := authoringpostgres.New(pool)
	read := authoring.NewReadService(repository, authoring.NewAuthorizationService(repository))
	router := authTestRouter(&authHTTP{
		sessions: sessions, authoring: read, authoringCreation: authoring.NewDraftCreationService(repository),
	})
	cookie := &http.Cookie{Name: sessionCookieName, Value: session.Token.Value()}
	body := `{"title":"First authoring Draft","intendedVersion":"0.1.0","sourceLanguage":"en-GB","description":"A complete initial Draft description.","objectives":["Explain the initial topic"],"changelog":"Initial Draft."}`
	response := authRequest(router, http.MethodPost, "/api/authoring/drafts", body, cookie, authTestCSRFToken().Value())
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("creation response=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	var created authoringDraftDTO
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil || created.ID == "" || created.Revision != 1 || created.Status != authoring.DraftActive || created.Title != "First authoring Draft" || created.SourceLanguage != "en-GB" || created.CourseID == "" {
		t.Fatalf("created draft=%#v err=%v", created, err)
	}
	if response.Header().Get("Location") != "/api/authoring/drafts/"+created.ID {
		t.Fatalf("location=%q", response.Header().Get("Location"))
	}

	draft, err := repository.GetDraft(ctx, authoring.DraftID(created.ID))
	if err != nil || draft.Metadata.Title != created.Title || draft.Metadata.IntendedVersion.String() != "0.1.0" {
		t.Fatalf("created draft persistence=%#v err=%v", draft, err)
	}
	workspace, err := repository.GetWorkspace(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	members, err := repository.ListMembers(ctx, workspace.ID)
	if err != nil || len(members) != 1 || members[0].UserID != string(user.ID) || members[0].Role != authoring.MemberMaintainer || members[0].RevokedAt != nil {
		t.Fatalf("creator membership=%#v err=%v", members, err)
	}
	if response = authRequest(router, http.MethodGet, "/api/authoring/drafts", "", cookie); response.Code != http.StatusOK || !containsJSONID(response.Body.Bytes(), created.ID) {
		t.Fatalf("created draft discovery=%d %s", response.Code, response.Body.String())
	}
	if response = authRequest(router, http.MethodGet, "/api/authoring/drafts/"+created.ID, "", cookie); response.Code != http.StatusOK {
		t.Fatalf("creator cannot read created draft=%d", response.Code)
	}

	t.Run("membership failure rolls back the Draft and Courses identity", func(t *testing.T) {
		failedUser, err := identityRepository.CreateUser(ctx, identity.UserActive)
		if err != nil {
			t.Fatal(err)
		}
		beforeDrafts := countAuthoringDrafts(t, ctx, pool)
		beforeCourses := countCourseIdentities(t, ctx, pool)
		beforeMembers := countAuthoringMembers(t, ctx, pool)
		if _, err := pool.Exec(ctx, `
			CREATE FUNCTION authoring.fail_test_creator_membership() RETURNS trigger AS $$
			BEGIN
				IF NEW.user_id = '`+string(failedUser.ID)+`'::uuid THEN
					RAISE EXCEPTION 'forced membership failure';
				END IF;
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql;
			CREATE TRIGGER fail_test_creator_membership
			BEFORE INSERT ON authoring.workspace_member
			FOR EACH ROW EXECUTE FUNCTION authoring.fail_test_creator_membership();
		`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := pool.Exec(ctx, `DROP TRIGGER IF EXISTS fail_test_creator_membership ON authoring.workspace_member; DROP FUNCTION IF EXISTS authoring.fail_test_creator_membership();`); err != nil {
				t.Error(err)
			}
		})
		input := authoring.DraftCreationInput{Title: "Rollback Draft", IntendedVersion: draft.Metadata.IntendedVersion, SourceLanguage: draft.Metadata.SourceLanguage, Description: "A description for the forced rollback.", LearningObjectives: []string{"Explain rollback"}, Changelog: "Initial Draft."}
		if _, err := repository.CreateDraftForCreator(ctx, input, string(failedUser.ID)); err == nil {
			t.Fatal("creation unexpectedly succeeded after forced membership failure")
		}
		if got := countAuthoringDrafts(t, ctx, pool); got != beforeDrafts {
			t.Fatalf("drafts after rollback=%d want %d", got, beforeDrafts)
		}
		if got := countCourseIdentities(t, ctx, pool); got != beforeCourses {
			t.Fatalf("course identities after rollback=%d want %d", got, beforeCourses)
		}
		if got := countAuthoringMembers(t, ctx, pool); got != beforeMembers {
			t.Fatalf("memberships after rollback=%d want %d", got, beforeMembers)
		}
	})
}

func countAuthoringDrafts(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM authoring.course_draft").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func countCourseIdentities(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM courses.course").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func countAuthoringMembers(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM authoring.workspace_member").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func containsJSONID(data []byte, id string) bool {
	var list struct {
		Drafts []struct {
			ID string `json:"id"`
		} `json:"drafts"`
	}
	if json.Unmarshal(data, &list) != nil {
		return false
	}
	for _, draft := range list.Drafts {
		if draft.ID == id {
			return true
		}
	}
	return false
}
