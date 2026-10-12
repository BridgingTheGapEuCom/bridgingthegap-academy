package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	infrastructurepostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/postgres"
)

const (
	fixtureEmail = "learner-e2e@example.com"
	fixtureSlug  = "learner-reader-integration"
	threadTitle  = "Real community persistence"
	replyBody    = "A durable real-stack reply."
)

type output struct {
	CourseID      string `json:"courseId"`
	Threads       int    `json:"threads"`
	Posts         int    `json:"posts"`
	MatchingTitle int    `json:"matchingTitle"`
	MatchingReply int    `json:"matchingReply"`
	HiddenPosts   int    `json:"hiddenPosts"`
}

func main() {
	if os.Getenv("BTG_COMMUNITY_E2E_ALLOW_SEED") != "1" {
		fatal(errors.New("refusing to manage the Community fixture without BTG_COMMUNITY_E2E_ALLOW_SEED=1"))
	}
	databaseURL := os.Getenv("BTG_LMS_DATABASE_URL")
	if databaseURL == "" {
		fatal(errors.New("BTG_LMS_DATABASE_URL is required"))
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
	var courseID string
	err = pool.QueryRow(ctx, `SELECT c.id FROM courses.course c WHERE c.slug=$1 AND EXISTS (SELECT 1 FROM identity.user_emails e WHERE e.normalized_email=$2)`, fixtureSlug, fixtureEmail).Scan(&courseID)
	if err != nil {
		fatal(fmt.Errorf("seed the learner E2E fixture first: %w", err))
	}
	command := "seed"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	switch command {
	case "seed":
		_, err = pool.Exec(ctx, `DELETE FROM community.post WHERE thread_id IN (SELECT id FROM community.thread WHERE course_id=$1)`, courseID)
		if err == nil {
			_, err = pool.Exec(ctx, `DELETE FROM community.thread WHERE course_id=$1`, courseID)
		}
		if err == nil {
			_, err = pool.Exec(ctx, `INSERT INTO community.course_community(course_id,mode) VALUES($1,'ENABLED') ON CONFLICT(course_id) DO UPDATE SET mode='ENABLED',updated_at=now()`, courseID)
		}
		if err == nil {
			err = json.NewEncoder(os.Stdout).Encode(output{CourseID: courseID})
		}
	case "status":
		var result output
		result.CourseID = courseID
		err = pool.QueryRow(ctx, `SELECT
			count(DISTINCT t.id), count(p.id),
			count(DISTINCT t.id) FILTER (WHERE t.title=$2),
			count(p.id) FILTER (WHERE p.body=$3),
			count(p.id) FILTER (WHERE p.state='HIDDEN')
			FROM community.thread t LEFT JOIN community.post p ON p.thread_id=t.id
			WHERE t.course_id=$1`, courseID, threadTitle, replyBody).Scan(&result.Threads, &result.Posts, &result.MatchingTitle, &result.MatchingReply, &result.HiddenPosts)
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

func fatal(err error) { _, _ = fmt.Fprintln(os.Stderr, err); os.Exit(1) }
