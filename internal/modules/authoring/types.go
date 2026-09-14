package authoring

import (
	"errors"
	"strings"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type DraftID string
type WorkspaceID string
type ModuleID string
type LessonID string

type DraftStatus string

const (
	DraftActive    DraftStatus = "ACTIVE"
	DraftAbandoned DraftStatus = "ABANDONED"
)

func (s DraftStatus) Valid() bool { return s == DraftActive || s == DraftAbandoned }

type MemberRole string

const (
	MemberMaintainer MemberRole = "MAINTAINER"
	MemberAuthor     MemberRole = "AUTHOR"
)

func (r MemberRole) Valid() bool { return r == MemberMaintainer || r == MemberAuthor }

// DraftMetadata is mutable workspace data. It is deliberately distinct from
// the immutable published CourseVersion and its lifecycle.
type DraftMetadata struct {
	CourseID           courses.CourseID
	IntendedVersion    courses.Version
	SourceLanguage     courses.LanguageTag
	Title              string
	Description        string
	LearningObjectives []string
	Changelog          string
	License            courses.ContentLicense
}

func (m DraftMetadata) Validate() error {
	if m.CourseID == "" || !m.IntendedVersion.Valid() {
		return errors.New("invalid draft course or intended version")
	}
	if _, err := courses.NormalizeLanguageTag(string(m.SourceLanguage)); err != nil {
		return err
	}
	if len(strings.TrimSpace(m.Title)) == 0 || len(m.Title) > 240 ||
		len(strings.TrimSpace(m.Description)) == 0 || len(m.Description) > 20000 ||
		len(strings.TrimSpace(m.Changelog)) == 0 || len(m.Changelog) > 20000 {
		return errors.New("invalid draft metadata text")
	}
	if err := validateObjectives(m.LearningObjectives); err != nil {
		return err
	}
	return m.License.Validate()
}

func validateObjectives(values []string) error {
	if len(values) == 0 || len(values) > 100 {
		return errors.New("invalid learning objectives")
	}
	for _, value := range values {
		if len(strings.TrimSpace(value)) == 0 || len(value) > 1000 {
			return errors.New("invalid learning objective")
		}
	}
	return nil
}

type CourseDraft struct {
	ID                   DraftID
	Metadata             DraftMetadata
	Status               DraftStatus
	Revision             int64
	CreatedAt, UpdatedAt time.Time
}

type AuthoringWorkspace struct {
	ID              WorkspaceID
	DraftID         DraftID
	CreatedByUserID string
	CreatedAt       time.Time
	LastActivityAt  time.Time
}

type WorkspaceMember struct {
	ID, UserID  string
	WorkspaceID WorkspaceID
	Role        MemberRole
	CreatedAt   time.Time
	RevokedAt   *time.Time
}

type ModuleInput struct {
	DraftID                       DraftID
	StableKey, Title, Description string
	Position                      int
}

func (m ModuleInput) Validate() error {
	if m.DraftID == "" {
		return errors.New("draft identifier required")
	}
	if key, err := courses.NormalizeStructureKey(m.StableKey); err != nil || key != m.StableKey {
		return errors.New("invalid module stable key")
	}
	if len(strings.TrimSpace(m.Title)) == 0 || len(m.Title) > 240 || len(m.Description) > 20000 {
		return errors.New("invalid draft module text")
	}
	if m.Position < 0 || m.Position > 100000 {
		return errors.New("invalid module position")
	}
	return nil
}

type DraftModule struct {
	ID ModuleID
	ModuleInput
	Revision             int64
	CreatedAt, UpdatedAt time.Time
}

type LessonInput struct {
	DraftID                       DraftID
	ModuleID                      ModuleID
	StableKey, Title, Description string
	LearningObjectives            []string
	EstimatedDurationMinutes      *int
	Position                      int
	Content                       courses.LessonContent
}

func (l LessonInput) Validate() error {
	if l.DraftID == "" || l.ModuleID == "" {
		return errors.New("draft and module identifiers required")
	}
	if key, err := courses.NormalizeStructureKey(l.StableKey); err != nil || key != l.StableKey {
		return errors.New("invalid lesson stable key")
	}
	if len(strings.TrimSpace(l.Title)) == 0 || len(l.Title) > 240 || len(strings.TrimSpace(l.Description)) == 0 || len(l.Description) > 20000 {
		return errors.New("invalid draft lesson text")
	}
	if err := validateObjectives(l.LearningObjectives); err != nil {
		return err
	}
	if l.EstimatedDurationMinutes != nil && (*l.EstimatedDurationMinutes < 1 || *l.EstimatedDurationMinutes > 1440) {
		return errors.New("invalid estimated lesson duration")
	}
	if l.Position < 0 || l.Position > 100000 {
		return errors.New("invalid lesson position")
	}
	return l.Content.Validate()
}

type DraftLesson struct {
	ID LessonID
	LessonInput
	Revision             int64
	CreatedAt, UpdatedAt time.Time
}

// Prerequisite is advisory metadata; no learner access decision depends on it.
type Prerequisite struct {
	LessonID        LessonID
	TargetLessonID  LessonID
	TargetStableKey string
	Position        int
}

func ValidatePrerequisiteKeys(self string, keys []string) error {
	if len(keys) > 100 {
		return errors.New("too many prerequisites")
	}
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if canonical, err := courses.NormalizeStructureKey(key); err != nil || canonical != key || key == self || seen[key] {
			return errors.New("invalid prerequisite key")
		}
		seen[key] = true
	}
	return nil
}
