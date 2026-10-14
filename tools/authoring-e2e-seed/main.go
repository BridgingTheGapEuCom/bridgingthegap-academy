package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	infrastructurepostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/postgres"
	"github.com/jackc/pgx/v5"
)

const (
	fixtureEmail          = "authoring-e2e@example.com"
	fixturePassword       = "authoring integration password"
	fixtureTitle          = "Authoring entry integration Draft"
	fixtureSourceLanguage = "en-GB"
)

type output struct {
	Email            string `json:"email"`
	Password         string `json:"password"`
	Title            string `json:"title"`
	SourceLanguage   string `json:"sourceLanguage"`
	DraftExists      bool   `json:"draftExists"`
	DraftID          string `json:"draftId,omitempty"`
	CourseID         string `json:"courseId,omitempty"`
	MembershipRole   string `json:"membershipRole,omitempty"`
	CreatorOwnsDraft bool   `json:"creatorOwnsDraft"`
	AccessibleDrafts int    `json:"accessibleDrafts"`
}

func main() {
	if os.Getenv("BTG_AUTHORING_E2E_ALLOW_SEED") != "1" {
		fatal(errors.New("refusing to manage the authoring fixture without BTG_AUTHORING_E2E_ALLOW_SEED=1"))
	}
	databaseURL := os.Getenv("BTG_LMS_DATABASE_URL")
	if databaseURL == "" {
		fatal(errors.New("BTG_LMS_DATABASE_URL is required"))
	}
	command := "seed"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	ctx := context.Background()
	if err := infrastructurepostgres.Migrate(ctx, databaseURL); err != nil {
		fatal(err)
	}
	pool, err := infrastructurepostgres.OpenPool(ctx, databaseURL)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()
	userID, err := ensureUser(ctx, identitypostgres.New(pool))
	if err != nil {
		fatal(err)
	}

	result := output{Email: fixtureEmail, Password: fixturePassword, Title: fixtureTitle, SourceLanguage: fixtureSourceLanguage}
	switch command {
	case "seed":
		if _, err = pool.Exec(ctx, `DELETE FROM identity.sessions WHERE user_id = $1`, userID); err != nil {
			fatal(err)
		}
		rows, queryErr := pool.Query(ctx, `SELECT d.id, d.course_id FROM authoring.course_draft d JOIN authoring.workspace w ON w.draft_id=d.id WHERE w.created_by_user_id=$1`, userID)
		if queryErr != nil {
			fatal(queryErr)
		}
		var draftIDs, courseIDs []string
		for rows.Next() {
			var draftID, courseID string
			if scanErr := rows.Scan(&draftID, &courseID); scanErr != nil {
				rows.Close()
				fatal(scanErr)
			}
			draftIDs = append(draftIDs, draftID)
			courseIDs = append(courseIDs, courseID)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			fatal(err)
		}
		rows.Close()
		for index, draftID := range draftIDs {
			if _, err = pool.Exec(ctx, `DELETE FROM authoring.course_draft WHERE id=$1`, draftID); err != nil {
				fatal(err)
			}
			if _, err = pool.Exec(ctx, `DELETE FROM courses.course WHERE id=$1`, courseIDs[index]); err != nil {
				fatal(err)
			}
		}
	case "status":
		err = pool.QueryRow(ctx, `
			SELECT d.id, d.course_id, m.role, w.created_by_user_id=$1
			FROM authoring.course_draft d
			JOIN authoring.workspace w ON w.draft_id=d.id
			JOIN authoring.workspace_member m ON m.workspace_id=w.id AND m.user_id=$1 AND m.revoked_at IS NULL
			WHERE d.title=$2 AND d.source_language=$3
			ORDER BY d.created_at DESC LIMIT 1`, userID, fixtureTitle, fixtureSourceLanguage).
			Scan(&result.DraftID, &result.CourseID, &result.MembershipRole, &result.CreatorOwnsDraft)
		if err == nil {
			result.DraftExists = true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			fatal(err)
		}
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM authoring.workspace_member m JOIN authoring.workspace w ON w.id=m.workspace_id WHERE m.user_id=$1 AND m.revoked_at IS NULL`, userID).Scan(&result.AccessibleDrafts); err != nil {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("unsupported command %q", command))
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fatal(err)
	}
}

func ensureUser(ctx context.Context, repository *identitypostgres.Repository) (identity.UserID, error) {
	email, err := repository.GetUserEmailByNormalized(ctx, fixtureEmail)
	if errors.Is(err, identity.ErrNotFound) {
		user, createErr := repository.CreateUser(ctx, identity.UserActive)
		if createErr != nil {
			return "", createErr
		}
		email, err = repository.CreateUserEmail(ctx, user.ID, fixtureEmail, true)
		if err != nil {
			return "", err
		}
		if _, err = repository.MarkUserEmailVerified(ctx, email.ID, time.Now().UTC()); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	hash, err := identity.DefaultPasswordHasher().HashPassword([]byte(fixturePassword))
	if err != nil {
		return "", err
	}
	if _, err = repository.ReplaceLocalPasswordHash(ctx, email.UserID, hash); errors.Is(err, identity.ErrNotFound) {
		_, err = repository.CreateLocalPasswordCredential(ctx, email.UserID, hash)
	}
	return email.UserID, err
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
