package courses

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/language"
)

type CourseID string
type CourseVersionID string
type ModuleID string
type LessonID string

var (
	slugPattern       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	versionPattern    = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9.+-]+$`)
	uuidPattern       = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type Course struct {
	ID        CourseID
	Slug      string
	CreatedAt time.Time
}

func NormalizeSlug(value string) (string, error) {
	slug := strings.TrimSpace(value)
	if len(slug) < 3 || len(slug) > 96 || !slugPattern.MatchString(slug) {
		return "", errors.New("invalid course slug")
	}
	return slug, nil
}

// NormalizeStructureKey applies the Course key rules to Module and Lesson
// logical keys. Keys are immutable references, never display titles.
func NormalizeStructureKey(value string) (string, error) { return NormalizeSlug(value) }

// Version is a constrained SemVer-like identifier used for published artifacts.
// Pre-release and build metadata are intentionally deferred until a product need exists.
type Version struct {
	Major int
	Minor int
	Patch int
}

func ParseVersion(value string) (Version, error) {
	match := versionPattern.FindStringSubmatch(value)
	if match == nil {
		return Version{}, errors.New("invalid course version")
	}
	parts := Version{}
	var err error
	if parts.Major, err = strconv.Atoi(match[1]); err != nil {
		return Version{}, errors.New("invalid course version")
	}
	if parts.Minor, err = strconv.Atoi(match[2]); err != nil {
		return Version{}, errors.New("invalid course version")
	}
	if parts.Patch, err = strconv.Atoi(match[3]); err != nil {
		return Version{}, errors.New("invalid course version")
	}
	return parts, nil
}

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

func (v Version) Valid() bool {
	if v.Major < 0 || v.Minor < 0 || v.Patch < 0 {
		return false
	}
	return versionPattern.MatchString(v.String())
}

func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		return compareInt(v.Major, other.Major)
	}
	if v.Minor != other.Minor {
		return compareInt(v.Minor, other.Minor)
	}
	return compareInt(v.Patch, other.Patch)
}

func compareInt(left, right int) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

type CourseVersionStatus string

const (
	CourseVersionPublished  CourseVersionStatus = "PUBLISHED"
	CourseVersionDeprecated CourseVersionStatus = "DEPRECATED"
	CourseVersionArchived   CourseVersionStatus = "ARCHIVED"
	CourseVersionWithdrawn  CourseVersionStatus = "WITHDRAWN"
)

func ParseCourseVersionStatus(value string) (CourseVersionStatus, error) {
	status := CourseVersionStatus(value)
	switch status {
	case CourseVersionPublished, CourseVersionDeprecated, CourseVersionArchived, CourseVersionWithdrawn:
		return status, nil
	default:
		return "", errors.New("invalid course version status")
	}
}

func (s CourseVersionStatus) CanTransitionTo(next CourseVersionStatus) bool {
	switch s {
	case CourseVersionPublished:
		return next == CourseVersionDeprecated || next == CourseVersionArchived || next == CourseVersionWithdrawn
	case CourseVersionDeprecated:
		return next == CourseVersionArchived || next == CourseVersionWithdrawn
	case CourseVersionArchived:
		return next == CourseVersionWithdrawn
	default:
		return false
	}
}

type LanguageTag string

// NormalizeLanguageTag accepts one BCP 47 language tag and returns the
// canonical spelling retained by golang.org/x/text. Tags are domain data, not
// display names: callers must not map names such as "English" or aliases such
// as "eng" into a different language on a user's behalf.
func NormalizeLanguageTag(value string) (LanguageTag, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 || strings.ContainsAny(value, "_ ") || strings.HasPrefix(value, "-") || strings.HasSuffix(value, "-") || strings.Contains(value, "--") {
		return "", errors.New("invalid source language")
	}
	tag, err := language.Parse(value)
	if err != nil {
		return "", errors.New("invalid source language")
	}
	canonical := tag.String()
	if canonical == "und" && !strings.EqualFold(value, "und") {
		return "", errors.New("invalid source language")
	}
	// x/text intentionally canonicalizes deprecated aliases. Do not silently
	// reinterpret a legacy three-letter alias such as "eng" as a distinct
	// user-selected language; callers can submit the canonical tag instead.
	inputBase := strings.Split(value, "-")[0]
	canonicalBase := strings.Split(canonical, "-")[0]
	if len(inputBase) == 3 && !strings.EqualFold(inputBase, canonicalBase) {
		return "", errors.New("invalid source language")
	}
	return LanguageTag(canonical), nil
}

type ContentLicenseKind string

const (
	ContentLicenseStandard          ContentLicenseKind = "STANDARD"
	ContentLicenseAllRightsReserved ContentLicenseKind = "ALL_RIGHTS_RESERVED"
	ContentLicenseCustom            ContentLicenseKind = "CUSTOM"
)

type ContentLicense struct {
	Kind        ContentLicenseKind
	Identifier  string
	DisplayName string
	URL         string
	CustomText  string
}

func (l ContentLicense) Validate() error {
	if len(l.DisplayName) == 0 || len(strings.TrimSpace(l.DisplayName)) == 0 || len(l.DisplayName) > 256 {
		return errors.New("invalid content license display name")
	}
	if l.URL != "" {
		parsed, err := url.ParseRequestURI(l.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || len(l.URL) > 2048 {
			return errors.New("invalid content license URL")
		}
	}
	switch l.Kind {
	case ContentLicenseStandard:
		if len(l.Identifier) == 0 || len(l.Identifier) > 128 || !identifierPattern.MatchString(l.Identifier) || l.CustomText != "" {
			return errors.New("invalid standard content license")
		}
	case ContentLicenseAllRightsReserved:
		if l.Identifier != "" || l.CustomText != "" {
			return errors.New("invalid all rights reserved content license")
		}
	case ContentLicenseCustom:
		if l.Identifier != "" || len(strings.TrimSpace(l.CustomText)) == 0 || len(l.CustomText) > 20000 {
			return errors.New("invalid custom content license")
		}
	default:
		return errors.New("invalid content license kind")
	}
	return nil
}

type ContributorRole string

const (
	ContributorAuthor     ContributorRole = "AUTHOR"
	ContributorMaintainer ContributorRole = "MAINTAINER"
)

func ParseContributorRole(value string) (ContributorRole, error) {
	role := ContributorRole(value)
	if role != ContributorAuthor && role != ContributorMaintainer {
		return "", errors.New("invalid contributor role")
	}
	return role, nil
}

// ContributorSnapshot is version-owned historical attribution. UserID is optional
// because an attributed person need not have a current Identity account.
type ContributorSnapshot struct {
	UserID      string          `json:"user_id,omitempty"`
	DisplayName string          `json:"display_name"`
	Role        ContributorRole `json:"role"`
	Order       int             `json:"order"`
}

func ValidateContributorSnapshots(contributors []ContributorSnapshot) error {
	if len(contributors) == 0 || len(contributors) > 100 {
		return errors.New("course version requires contributor attribution")
	}
	for index, contributor := range contributors {
		if contributor.Order != index {
			return errors.New("contributor attribution order must be contiguous")
		}
		if len(strings.TrimSpace(contributor.DisplayName)) == 0 || len(contributor.DisplayName) > 256 {
			return errors.New("invalid contributor display name")
		}
		if _, err := ParseContributorRole(string(contributor.Role)); err != nil {
			return err
		}
		if contributor.UserID != "" && !uuidPattern.MatchString(contributor.UserID) {
			return errors.New("invalid contributor user identifier")
		}
	}
	return nil
}

type CourseVersionInput struct {
	CourseID           CourseID
	Version            Version
	Status             CourseVersionStatus
	Title              string
	Description        string
	LearningObjectives []string
	SourceLanguage     LanguageTag
	Changelog          string
	License            ContentLicense
	Attribution        []ContributorSnapshot
	PublishedAt        time.Time
}

// CourseVersionMetadata is the immutable content metadata shared by a
// published CourseVersion and an Authoring Review snapshot. It deliberately
// excludes lifecycle, attribution, and publication time, which belong to the
// later publishing boundary.
type CourseVersionMetadata struct {
	CourseID           CourseID
	Version            Version
	Title              string
	Description        string
	LearningObjectives []string
	SourceLanguage     LanguageTag
	Changelog          string
	License            ContentLicense
}

func (m CourseVersionMetadata) Validate() error {
	if m.CourseID == "" {
		return errors.New("course identifier is required")
	}
	if !m.Version.Valid() {
		return errors.New("invalid course version")
	}
	if len(strings.TrimSpace(m.Title)) == 0 || len(m.Title) > 240 {
		return errors.New("invalid course version title")
	}
	if len(strings.TrimSpace(m.Description)) == 0 || len(m.Description) > 20000 {
		return errors.New("invalid course version description")
	}
	if len(m.LearningObjectives) == 0 || len(m.LearningObjectives) > 100 {
		return errors.New("course version requires learning objectives")
	}
	for _, objective := range m.LearningObjectives {
		if len(strings.TrimSpace(objective)) == 0 || len(objective) > 1000 {
			return errors.New("invalid learning objective")
		}
	}
	language, err := NormalizeLanguageTag(string(m.SourceLanguage))
	if err != nil || language != m.SourceLanguage {
		return errors.New("invalid source language")
	}
	if len(strings.TrimSpace(m.Changelog)) == 0 || len(m.Changelog) > 20000 {
		return errors.New("course version changelog is required")
	}
	if err := m.License.Validate(); err != nil {
		return err
	}
	return nil
}

func (in CourseVersionInput) Validate() error {
	if err := (CourseVersionMetadata{
		CourseID:           in.CourseID,
		Version:            in.Version,
		Title:              in.Title,
		Description:        in.Description,
		LearningObjectives: in.LearningObjectives,
		SourceLanguage:     in.SourceLanguage,
		Changelog:          in.Changelog,
		License:            in.License,
	}).Validate(); err != nil {
		return err
	}
	if _, err := ParseCourseVersionStatus(string(in.Status)); err != nil {
		return err
	}
	if err := ValidateContributorSnapshots(in.Attribution); err != nil {
		return err
	}
	if in.PublishedAt.IsZero() {
		return errors.New("course version published time is required")
	}
	return nil
}

type CourseVersion struct {
	ID                 CourseVersionID
	CourseID           CourseID
	Version            Version
	Status             CourseVersionStatus
	Title              string
	Description        string
	LearningObjectives []string
	SourceLanguage     LanguageTag
	Changelog          string
	License            ContentLicense
	Attribution        []ContributorSnapshot
	CreatedAt          time.Time
	PublishedAt        time.Time
}

type ModuleInput struct {
	CourseVersionID CourseVersionID
	StableKey       string
	Title           string
	Description     string
	Position        int
}

func (in ModuleInput) Validate() error {
	if in.CourseVersionID == "" {
		return errors.New("course version identifier is required")
	}
	if _, err := NormalizeStructureKey(in.StableKey); err != nil {
		return errors.New("invalid module stable key")
	}
	if len(strings.TrimSpace(in.Title)) == 0 || len(in.Title) > 240 {
		return errors.New("invalid module title")
	}
	if len(in.Description) > 20000 {
		return errors.New("invalid module description")
	}
	if in.Position < 0 || in.Position > 100000 {
		return errors.New("invalid module position")
	}
	return nil
}

type Module struct {
	ID              ModuleID
	CourseVersionID CourseVersionID
	StableKey       string
	Title           string
	Description     string
	Position        int
	CreatedAt       time.Time
}

type LessonInput struct {
	CourseVersionID          CourseVersionID
	ModuleID                 ModuleID
	StableKey                string
	Title                    string
	Description              string
	LearningObjectives       []string
	EstimatedDurationMinutes *int
	Position                 int
	Content                  LessonContent
}

func (in LessonInput) Validate() error {
	if err := in.ValidateMetadata(); err != nil {
		return err
	}
	return in.Content.Validate()
}

// ValidateMetadata supports read projections that deliberately omit block content.
func (in LessonInput) ValidateMetadata() error {
	if in.CourseVersionID == "" || in.ModuleID == "" {
		return errors.New("lesson course version and module identifiers are required")
	}
	if _, err := NormalizeStructureKey(in.StableKey); err != nil {
		return errors.New("invalid lesson stable key")
	}
	if len(strings.TrimSpace(in.Title)) == 0 || len(in.Title) > 240 {
		return errors.New("invalid lesson title")
	}
	if len(strings.TrimSpace(in.Description)) == 0 || len(in.Description) > 20000 {
		return errors.New("invalid lesson description")
	}
	if len(in.LearningObjectives) == 0 || len(in.LearningObjectives) > 100 {
		return errors.New("lesson requires learning objectives")
	}
	for _, objective := range in.LearningObjectives {
		if len(strings.TrimSpace(objective)) == 0 || len(objective) > 1000 {
			return errors.New("invalid lesson learning objective")
		}
	}
	if in.EstimatedDurationMinutes != nil && (*in.EstimatedDurationMinutes < 1 || *in.EstimatedDurationMinutes > 1440) {
		return errors.New("invalid estimated lesson duration")
	}
	if in.Position < 0 || in.Position > 100000 {
		return errors.New("invalid lesson position")
	}
	return nil
}

type Lesson struct {
	ID                       LessonID
	CourseVersionID          CourseVersionID
	ModuleID                 ModuleID
	StableKey                string
	Title                    string
	Description              string
	LearningObjectives       []string
	EstimatedDurationMinutes *int
	Position                 int
	Content                  LessonContent
	CreatedAt                time.Time
}

// LessonPrerequisite is advisory metadata. Its stable target key supports
// cross-version comparison; it never controls whether a learner may open a Lesson.
type LessonPrerequisite struct {
	CourseVersionID       CourseVersionID
	LessonID              LessonID
	PrerequisiteLessonID  LessonID
	PrerequisiteStableKey string
	Position              int
}

type LessonPrerequisiteInput struct {
	CourseVersionID       CourseVersionID
	LessonID              LessonID
	PrerequisiteStableKey string
	Position              int
}

func (in LessonPrerequisiteInput) Validate() error {
	if in.CourseVersionID == "" || in.LessonID == "" {
		return errors.New("prerequisite course version and lesson identifiers are required")
	}
	if _, err := NormalizeStructureKey(in.PrerequisiteStableKey); err != nil {
		return errors.New("invalid prerequisite lesson key")
	}
	if in.Position < 0 || in.Position > 100000 {
		return errors.New("invalid prerequisite position")
	}
	return nil
}
