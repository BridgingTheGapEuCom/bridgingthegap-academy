package community

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid community entity")
var ErrNotFound = errors.New("community entity not found")
var ErrDisabled = errors.New("community is disabled")
var ErrOpeningPost = errors.New("opening post must be moderated with its thread")

type Mode string
type Visibility string
type Capability string

const (
	CommunityEnabled  Mode       = "ENABLED"
	CommunityDisabled Mode       = "DISABLED"
	Visible           Visibility = "VISIBLE"
	Hidden            Visibility = "HIDDEN"
)
const CapabilityModerate Capability = "community.moderate"
const (
	maxTitle = 240
	maxBody  = 20000
)

type CourseCommunity struct {
	CourseID  string
	Mode      Mode
	CreatedAt time.Time
	UpdatedAt time.Time
}
type Thread struct {
	ID              string
	CourseID        string
	Title           string
	CreatedByUserID string `json:"-"`
	State           Visibility
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
type Post struct {
	ID           string
	ThreadID     string
	AuthorUserID string `json:"-"`
	Body         string
	State        Visibility
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
type ThreadInput struct {
	CourseID        string
	Title           string
	CreatedByUserID string `json:"-"`
	OpeningBody     string
}
type PostInput struct {
	CourseID     string
	ThreadID     string
	AuthorUserID string `json:"-"`
	Body         string
}
type ThreadSummary struct {
	Thread
	PostCount int
}
type Repository interface {
	CreateCommunity(context.Context, string, Mode) (CourseCommunity, error)
	GetCommunity(context.Context, string) (CourseCommunity, error)
	CreateThread(context.Context, ThreadInput) (Thread, Post, error)
	GetThread(context.Context, string) (Thread, error)
	GetPost(context.Context, string) (Post, error)
	ListVisibleThreads(context.Context, string, int, int) ([]ThreadSummary, int, error)
	GetVisibleThread(context.Context, string, string) (Thread, error)
	ListVisiblePosts(context.Context, string, int, int) ([]Post, int, error)
	CreatePost(context.Context, PostInput) (Post, error)
	SetThreadState(context.Context, string, string, Visibility) (Thread, error)
	SetPostState(context.Context, string, string, string, Visibility) (Post, error)
}

func NormalizeTitle(v string) (string, error) { return normalize(v, maxTitle) }
func NormalizeBody(v string) (string, error)  { return normalize(v, maxBody) }
func normalize(v string, max int) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > max {
		return "", ErrInvalid
	}
	return v, nil
}
func (c CourseCommunity) Validate() error {
	if !uuid(c.CourseID) || (c.Mode != CommunityEnabled && c.Mode != CommunityDisabled) || c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		return ErrInvalid
	}
	return nil
}
func (t Thread) Validate() error {
	if !uuid(t.ID) || !uuid(t.CourseID) || !uuid(t.CreatedByUserID) || !text(t.Title, maxTitle) || !visibility(t.State) || t.CreatedAt.IsZero() || t.UpdatedAt.IsZero() {
		return ErrInvalid
	}
	return nil
}
func (p Post) Validate() error {
	if !uuid(p.ID) || !uuid(p.ThreadID) || !uuid(p.AuthorUserID) || !text(p.Body, maxBody) || !visibility(p.State) || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return ErrInvalid
	}
	return nil
}
func (i ThreadInput) Validate() error {
	_, a := NormalizeTitle(i.Title)
	_, b := NormalizeBody(i.OpeningBody)
	if !uuid(i.CourseID) || !uuid(i.CreatedByUserID) || a != nil || b != nil {
		return ErrInvalid
	}
	return nil
}
func (i PostInput) Validate() error {
	_, bodyErr := NormalizeBody(i.Body)
	if !uuid(i.CourseID) || !uuid(i.ThreadID) || !uuid(i.AuthorUserID) || bodyErr != nil {
		return ErrInvalid
	}
	return nil
}
func visibility(s Visibility) bool { return s == Visible || s == Hidden }
func text(v string, max int) bool  { return v == strings.TrimSpace(v) && v != "" && len(v) <= max }
func isHex(c rune) bool            { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }

func uuid(v string) bool {
	if len(v) != 36 {
		return false
	}
	for i, c := range v {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !isHex(c) {
			return false
		}
	}
	return true
}
