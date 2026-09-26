// Package credentials owns durable awarded certificates independently of
// Course publication, Authoring, transport, and external credential formats.
package credentials

import (
	"errors"
	"strings"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

var (
	ErrInvalidCertificate  = errors.New("invalid certificate")
	ErrCertificateNotFound = errors.New("certificate not found")
	ErrCertificateConflict = errors.New("certificate conflicts with existing data")
)

type CertificateID string
type CertificateStatus string

const (
	CertificateActive  CertificateStatus = "ACTIVE"
	CertificateRevoked CertificateStatus = "REVOKED"
)

// Issuer is a small, stable description owned by application configuration at
// issuance time. It is intentionally independent of Open Badges issuer types.
type Issuer struct {
	ID   string
	Name string
}

// Achievement is the immutable, learner-meaningful course-completion snapshot.
// Its values are copied from the exact published CourseVersion at issuance.
type Achievement struct {
	CourseTitle   string
	CourseVersion string
	Language      string
	Criteria      string
}

// Certificate is an historical achievement. Recipient, CourseVersion,
// achievement snapshot, issuer, and issuance time never change; revocation is
// the sole lifecycle transition in this foundation.
type Certificate struct {
	ID              CertificateID
	LearnerUserID   string `json:"-"`
	CourseID        string `json:"-"`
	CourseVersionID string `json:"-"`
	Achievement     Achievement
	Issuer          Issuer
	IssuedAt        time.Time
	Status          CertificateStatus
	RevokedAt       *time.Time
}

type CertificateInput struct {
	LearnerUserID   string `json:"-"`
	CourseID        string `json:"-"`
	CourseVersionID string `json:"-"`
	Achievement     Achievement
	Issuer          Issuer
}

// CourseCompletionAchievement freezes the public meaning needed for a later
// certificate/export adapter from an already-published immutable version.
func CourseCompletionAchievement(version courses.ImmutableCourseVersion, criteria string) (Achievement, error) {
	if version.ID == "" || version.CourseVersion.Status != courses.CourseVersionPublished || version.CourseVersion.CourseID == "" {
		return Achievement{}, ErrInvalidCertificate
	}
	achievement := Achievement{
		CourseTitle: version.CourseVersion.Title, CourseVersion: version.CourseVersion.Version.String(),
		Language: string(version.CourseVersion.SourceLanguage), Criteria: criteria,
	}
	return achievement, achievement.Validate()
}

func (issuer Issuer) Validate() error {
	if !validText(issuer.ID, 256) || !validText(issuer.Name, 256) {
		return ErrInvalidCertificate
	}
	return nil
}

func (achievement Achievement) Validate() error {
	if !validText(achievement.CourseTitle, 240) || !validVersion(achievement.CourseVersion) || !validLanguage(achievement.Language) || !validText(achievement.Criteria, 4000) {
		return ErrInvalidCertificate
	}
	return nil
}

func (input CertificateInput) Validate() error {
	if !validUUID(input.LearnerUserID) || !validUUID(input.CourseID) || !validUUID(input.CourseVersionID) || input.Achievement.Validate() != nil || input.Issuer.Validate() != nil {
		return ErrInvalidCertificate
	}
	return nil
}

func (certificate Certificate) Validate() error {
	input := CertificateInput{LearnerUserID: certificate.LearnerUserID, CourseID: certificate.CourseID, CourseVersionID: certificate.CourseVersionID, Achievement: certificate.Achievement, Issuer: certificate.Issuer}
	if !validUUID(string(certificate.ID)) || input.Validate() != nil || certificate.IssuedAt.IsZero() {
		return ErrInvalidCertificate
	}
	switch certificate.Status {
	case CertificateActive:
		if certificate.RevokedAt != nil {
			return ErrInvalidCertificate
		}
	case CertificateRevoked:
		if certificate.RevokedAt == nil || certificate.RevokedAt.IsZero() || certificate.RevokedAt.Before(certificate.IssuedAt) {
			return ErrInvalidCertificate
		}
	default:
		return ErrInvalidCertificate
	}
	return nil
}

func (certificate Certificate) Revoke(at time.Time) (Certificate, error) {
	if certificate.Validate() != nil || at.IsZero() {
		return Certificate{}, ErrInvalidCertificate
	}
	if certificate.Status == CertificateRevoked {
		return certificate, nil
	}
	if at.Before(certificate.IssuedAt) {
		return Certificate{}, ErrInvalidCertificate
	}
	revoked := certificate
	at = at.UTC()
	revoked.Status, revoked.RevokedAt = CertificateRevoked, &at
	return revoked, revoked.Validate()
}

func validText(value string, maximum int) bool {
	return value == strings.TrimSpace(value) && value != "" && len(value) <= maximum
}
func validVersion(value string) bool { _, err := courses.ParseVersion(value); return err == nil }
func validLanguage(value string) bool {
	_, err := courses.NormalizeLanguageTag(value)
	return err == nil && value != ""
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') && (character < 'A' || character > 'F') {
			return false
		}
	}
	return true
}
