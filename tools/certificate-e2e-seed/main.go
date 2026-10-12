package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	infrastructurepostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/postgres"
	"github.com/jackc/pgx/v5"
)

const (
	fixtureEmail = "learner-e2e@example.com"
	fixtureSlug  = "learner-reader-integration"
)

type output struct {
	Exists          bool   `json:"exists"`
	CertificateID   string `json:"certificateId,omitempty"`
	Status          string `json:"status,omitempty"`
	LearnerUserID   string `json:"learnerUserId,omitempty"`
	CourseID        string `json:"courseId,omitempty"`
	CourseVersionID string `json:"courseVersionId,omitempty"`
	Count           int    `json:"count"`
}

func main() {
	if os.Getenv("BTG_CERTIFICATE_E2E_ALLOW_INSPECT") != "1" {
		fatal(errors.New("refusing to inspect without BTG_CERTIFICATE_E2E_ALLOW_INSPECT=1"))
	}
	databaseURL := os.Getenv("BTG_LMS_DATABASE_URL")
	if databaseURL == "" {
		fatal(errors.New("BTG_LMS_DATABASE_URL is required"))
	}
	ctx := context.Background()
	pool, err := infrastructurepostgres.OpenPool(ctx, databaseURL)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()

	var result output
	err = pool.QueryRow(ctx, `SELECT c.id, c.status, c.learner_user_id, c.course_id, c.course_version_id,
		count(*) OVER () FROM credentials.certificate c
		JOIN identity.user_emails e ON e.user_id = c.learner_user_id
		JOIN courses.course course_record ON course_record.id = c.course_id
		WHERE e.normalized_email = $1 AND course_record.slug = $2`, fixtureEmail, fixtureSlug).
		Scan(&result.CertificateID, &result.Status, &result.LearnerUserID, &result.CourseID, &result.CourseVersionID, &result.Count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_ = json.NewEncoder(os.Stdout).Encode(result)
			return
		}
		fatal(err)
	}
	result.Exists = true
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
