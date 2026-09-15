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

// DraftMetadataPatch contains only mutable draft-owned fields. Nil means that
// a field was omitted and must retain its current value. Course identity,
// lifecycle status, workspace state, and revisions are intentionally absent.
type DraftMetadataPatch struct {
	IntendedVersion    *courses.Version
	SourceLanguage     *courses.LanguageTag
	Title              *string
	Description        *string
	LearningObjectives *[]string
	Changelog          *string
	License            *courses.ContentLicense
}

func (p DraftMetadataPatch) Empty() bool {
	return p.IntendedVersion == nil && p.SourceLanguage == nil && p.Title == nil &&
		p.Description == nil && p.LearningObjectives == nil && p.Changelog == nil && p.License == nil
}

// Apply validates the complete resulting metadata rather than duplicating
// field rules in a transport. Objectives are copied so request slices cannot
// mutate the candidate after validation.
func (p DraftMetadataPatch) Apply(current DraftMetadata) (DraftMetadata, error) {
	if p.Empty() {
		return DraftMetadata{}, errors.New("draft metadata patch is empty")
	}
	next := current
	if p.IntendedVersion != nil {
		next.IntendedVersion = *p.IntendedVersion
	}
	if p.SourceLanguage != nil {
		next.SourceLanguage = *p.SourceLanguage
	}
	if p.Title != nil {
		next.Title = *p.Title
	}
	if p.Description != nil {
		next.Description = *p.Description
	}
	if p.LearningObjectives != nil {
		next.LearningObjectives = append([]string(nil), (*p.LearningObjectives)...)
	}
	if p.Changelog != nil {
		next.Changelog = *p.Changelog
	}
	if p.License != nil {
		next.License = *p.License
	}
	if err := next.Validate(); err != nil {
		return DraftMetadata{}, err
	}
	return next, nil
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

// MaxModulesPerDraft bounds authoring structure operations and their request
// payloads without constraining ordinary course design.
const MaxModulesPerDraft = 1000

// MaxLessonsPerDraft bounds the complete structural-order document accepted by
// Authoring while leaving ample room for substantial courses.
const MaxLessonsPerDraft = 10000

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

// DraftModulePatch deliberately excludes stable key and position. A stable
// key is publication continuity metadata, while position changes belong to
// the explicit full-order operation.
type DraftModulePatch struct {
	Title       *string
	Description *string
}

func (p DraftModulePatch) Empty() bool { return p.Title == nil && p.Description == nil }

func (p DraftModulePatch) Apply(current ModuleInput) (ModuleInput, error) {
	if p.Empty() {
		return ModuleInput{}, errors.New("draft module patch is empty")
	}
	next := current
	if p.Title != nil {
		next.Title = *p.Title
	}
	if p.Description != nil {
		next.Description = *p.Description
	}
	if err := next.Validate(); err != nil {
		return ModuleInput{}, err
	}
	return next, nil
}

func ValidateModuleOrder(order []ModuleID) error {
	if len(order) > MaxModulesPerDraft {
		return errors.New("too many modules")
	}
	seen := make(map[ModuleID]struct{}, len(order))
	for _, id := range order {
		if id == "" {
			return errors.New("module identifier required")
		}
		if _, found := seen[id]; found {
			return errors.New("duplicate module identifier")
		}
		seen[id] = struct{}{}
	}
	return nil
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

// DraftLessonPatch contains metadata only. Stable key, Module assignment,
// position, prerequisite relations, and canonical content each have explicit
// structural operations.
type DraftLessonPatch struct {
	Title                    *string
	Description              *string
	LearningObjectives       *[]string
	EstimatedDurationSet     bool
	EstimatedDurationMinutes *int
}

func (p DraftLessonPatch) Empty() bool {
	return p.Title == nil && p.Description == nil && p.LearningObjectives == nil && !p.EstimatedDurationSet
}

func (p DraftLessonPatch) Apply(current LessonInput) (LessonInput, error) {
	if p.Empty() {
		return LessonInput{}, errors.New("draft lesson patch is empty")
	}
	next := current
	if p.Title != nil {
		next.Title = *p.Title
	}
	if p.Description != nil {
		next.Description = *p.Description
	}
	if p.LearningObjectives != nil {
		next.LearningObjectives = append([]string(nil), (*p.LearningObjectives)...)
	}
	if p.EstimatedDurationSet {
		next.EstimatedDurationMinutes = p.EstimatedDurationMinutes
	}
	if err := next.Validate(); err != nil {
		return LessonInput{}, err
	}
	return next, nil
}

// ModuleLessonOrder is one module's complete Lesson order. Reorder requests
// contain every module and lesson in the Draft exactly once, allowing moves to
// remain atomic and preserving stable Lesson IDs and keys.
type ModuleLessonOrder struct {
	ModuleID  ModuleID
	LessonIDs []LessonID
}

func ValidateLessonOrder(order []ModuleLessonOrder) error {
	if len(order) > MaxModulesPerDraft {
		return errors.New("too many modules")
	}
	modules := make(map[ModuleID]struct{}, len(order))
	lessons := make(map[LessonID]struct{})
	for _, module := range order {
		if module.ModuleID == "" {
			return errors.New("module identifier required")
		}
		if _, found := modules[module.ModuleID]; found {
			return errors.New("duplicate module identifier")
		}
		modules[module.ModuleID] = struct{}{}
		for _, lessonID := range module.LessonIDs {
			if lessonID == "" {
				return errors.New("lesson identifier required")
			}
			if _, found := lessons[lessonID]; found {
				return errors.New("duplicate lesson identifier")
			}
			lessons[lessonID] = struct{}{}
			if len(lessons) > MaxLessonsPerDraft {
				return errors.New("too many lessons")
			}
		}
	}
	return nil
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
