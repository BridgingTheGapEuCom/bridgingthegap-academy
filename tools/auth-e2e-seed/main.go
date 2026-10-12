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
)

const (
	fixtureEmail    = "auth-e2e@example.com"
	fixturePassword = "auth integration password"
)

type statusOutput struct {
	Active  int `json:"active"`
	Expired int `json:"expired"`
	Revoked int `json:"revoked"`
}

func main() {
	if os.Getenv("BTG_AUTH_E2E_ALLOW_SEED") != "1" {
		fatal(errors.New("refusing to manage the auth fixture without BTG_AUTH_E2E_ALLOW_SEED=1"))
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
	switch command {
	case "seed":
		_, err = pool.Exec(ctx, `DELETE FROM identity.sessions WHERE user_id = $1`, userID)
	case "expire":
		_, err = pool.Exec(ctx, `
			UPDATE identity.sessions
			SET created_at = now() - interval '2 seconds',
				last_seen_at = now() - interval '2 seconds',
				expires_at = now() - interval '1 second'
			WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()`, userID)
	case "status":
		var result statusOutput
		err = pool.QueryRow(ctx, `
			SELECT
				count(*) FILTER (WHERE revoked_at IS NULL AND expires_at > now()),
				count(*) FILTER (WHERE revoked_at IS NULL AND expires_at <= now()),
				count(*) FILTER (WHERE revoked_at IS NOT NULL)
			FROM identity.sessions WHERE user_id = $1`, userID).Scan(&result.Active, &result.Expired, &result.Revoked)
		if err == nil {
			err = json.NewEncoder(os.Stdout).Encode(result)
		}
	default:
		err = fmt.Errorf("unsupported command %q", command)
	}
	if err != nil {
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
