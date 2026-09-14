package courses

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type CourseID string
type CourseVersionID string

var (
	slugPattern       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	versionPattern    = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)
	languagePattern   = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)
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

// NormalizeLanguageTag accepts a deliberately small BCP 47-compatible profile
// and stores its conventional casing (for example, en and en-GB).
func NormalizeLanguageTag(value string) (LanguageTag, error) {
	value = strings.TrimSpace(value)
	if len(value) > 64 || !languagePattern.MatchString(value) {
		return "", errors.New("invalid source language")
	}
	parts := strings.Split(value, "-")
	parts[0] = strings.ToLower(parts[0])
	for i := 1; i < len(parts); i++ {
		switch {
		case len(parts[i]) == 4 && isASCIIAlpha(parts[i]):
			parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
		case (len(parts[i]) == 2 && isASCIIAlpha(parts[i])) || (len(parts[i]) == 3 && isDigits(parts[i])):
			parts[i] = strings.ToUpper(parts[i])
		default:
			parts[i] = strings.ToLower(parts[i])
		}
	}
	return LanguageTag(strings.Join(parts, "-")), nil
}

func isASCIIAlpha(value string) bool {
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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

func (in CourseVersionInput) Validate() error {
	if in.CourseID == "" {
		return errors.New("course identifier is required")
	}
	if !in.Version.Valid() {
		return errors.New("invalid course version")
	}
	if _, err := ParseCourseVersionStatus(string(in.Status)); err != nil {
		return err
	}
	if len(strings.TrimSpace(in.Title)) == 0 || len(in.Title) > 240 {
		return errors.New("invalid course version title")
	}
	if len(strings.TrimSpace(in.Description)) == 0 || len(in.Description) > 20000 {
		return errors.New("invalid course version description")
	}
	if len(in.LearningObjectives) == 0 || len(in.LearningObjectives) > 100 {
		return errors.New("course version requires learning objectives")
	}
	for _, objective := range in.LearningObjectives {
		if len(strings.TrimSpace(objective)) == 0 || len(objective) > 1000 {
			return errors.New("invalid learning objective")
		}
	}
	if _, err := NormalizeLanguageTag(string(in.SourceLanguage)); err != nil || string(in.SourceLanguage) == "" {
		return errors.New("invalid source language")
	}
	if len(strings.TrimSpace(in.Changelog)) == 0 || len(in.Changelog) > 20000 {
		return errors.New("course version changelog is required")
	}
	if err := in.License.Validate(); err != nil {
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
