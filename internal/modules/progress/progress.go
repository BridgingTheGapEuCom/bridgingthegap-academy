// Package progress owns private learner completion facts through immutable Courses versions.
package progress

import (
	"errors"
	"sort"
	"time"
)

var ErrInvalidProgress = errors.New("invalid course progress")
var ErrProgressNotFound = errors.New("course progress not found")
var ErrRevisionMismatch = errors.New("course progress revision mismatch")

type CourseProgress struct {
	LearnerUserID       string `json:"-"`
	CourseVersionID     string `json:"-"`
	Revision            int64
	CompletedLessonKeys []string `json:"-"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (p CourseProgress) Validate() error {
	if !validUUID(p.LearnerUserID) || !validUUID(p.CourseVersionID) || p.Revision < 1 || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() || !strictKeys(p.CompletedLessonKeys) {
		return ErrInvalidProgress
	}
	return nil
}
func New(learner, version string, now time.Time) (CourseProgress, error) {
	p := CourseProgress{LearnerUserID: learner, CourseVersionID: version, Revision: 1, CompletedLessonKeys: []string{}, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
	return p, p.Validate()
}
func (p CourseProgress) MarkLessonCompleted(key string, at time.Time) (CourseProgress, bool, error) {
	if p.Validate() != nil || !validKey(key) || at.IsZero() {
		return CourseProgress{}, false, ErrInvalidProgress
	}
	for _, v := range p.CompletedLessonKeys {
		if v == key {
			return p, false, nil
		}
	}
	q := p
	q.CompletedLessonKeys = append(append([]string{}, p.CompletedLessonKeys...), key)
	sort.Strings(q.CompletedLessonKeys)
	q.Revision++
	q.UpdatedAt = at.UTC()
	return q, true, q.Validate()
}
func IsCourseComplete(lessonKeys []string, p CourseProgress) bool {
	if len(lessonKeys) == 0 {
		return false
	}
	set := map[string]bool{}
	for _, v := range p.CompletedLessonKeys {
		set[v] = true
	}
	for _, v := range lessonKeys {
		if !set[v] {
			return false
		}
	}
	return true
}
func validUUID(v string) bool {
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
func validKey(v string) bool {
	if len(v) < 1 || len(v) > 160 {
		return false
	}
	for i, c := range v {
		if !isLowerAlnum(c) && (c != '-' || i == 0 || i == len(v)-1) {
			return false
		}
	}
	return true
}
func isHex(c rune) bool        { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
func isLowerAlnum(c rune) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }

func strictKeys(keys []string) bool {
	last := ""
	for _, v := range keys {
		if !validKey(v) || (last != "" && v <= last) {
			return false
		}
		last = v
	}
	return true
}
