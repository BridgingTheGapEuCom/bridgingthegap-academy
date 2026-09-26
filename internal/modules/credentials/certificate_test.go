package credentials

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

const (
	testCertificateID = "10000000-0000-4000-8000-000000000001"
	testLearnerID     = "20000000-0000-4000-8000-000000000001"
	testCourseID      = "30000000-0000-4000-8000-000000000001"
	testVersionID     = "40000000-0000-4000-8000-000000000001"
)

func validCertificate(now time.Time) Certificate {
	return Certificate{
		ID: testCertificateID, LearnerUserID: testLearnerID, CourseID: testCourseID, CourseVersionID: testVersionID,
		Achievement: Achievement{CourseTitle: "Event-driven integration", CourseVersion: "1.0.0", Language: "en-GB", Criteria: "Completed every lesson in this published CourseVersion."},
		Issuer:      Issuer{ID: "academy.example", Name: "Academy"}, IssuedAt: now, Status: CertificateActive,
	}
}

func TestCertificateValidationAndRevocationPreserveFrozenFacts(t *testing.T) {
	now := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	certificate := validCertificate(now)
	if err := certificate.Validate(); err != nil {
		t.Fatal(err)
	}
	revoked, err := certificate.Revoke(now.Add(time.Hour))
	if err != nil || revoked.Status != CertificateRevoked || revoked.RevokedAt == nil {
		t.Fatalf("revoke = %#v, %v", revoked, err)
	}
	if revoked.ID != certificate.ID || revoked.LearnerUserID != certificate.LearnerUserID || revoked.CourseID != certificate.CourseID || revoked.CourseVersionID != certificate.CourseVersionID || !reflect.DeepEqual(revoked.Achievement, certificate.Achievement) || !reflect.DeepEqual(revoked.Issuer, certificate.Issuer) || !revoked.IssuedAt.Equal(certificate.IssuedAt) {
		t.Fatalf("revocation changed immutable facts: %#v", revoked)
	}
	replayed, err := revoked.Revoke(now.Add(2 * time.Hour))
	if err != nil || !reflect.DeepEqual(replayed, revoked) {
		t.Fatalf("revocation replay = %#v, %v", replayed, err)
	}
}

func TestCertificateRejectsInvalidLifecycleAndFrozenValues(t *testing.T) {
	now := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	for name, change := range map[string]func(*Certificate){
		"invalid id":             func(c *Certificate) { c.ID = "not-a-uuid" },
		"invalid recipient":      func(c *Certificate) { c.LearnerUserID = "not-a-uuid" },
		"invalid course version": func(c *Certificate) { c.CourseVersionID = "not-a-uuid" },
		"empty title":            func(c *Certificate) { c.Achievement.CourseTitle = "" },
		"active revoked":         func(c *Certificate) { at := now; c.RevokedAt = &at },
		"revoked without time":   func(c *Certificate) { c.Status = CertificateRevoked },
		"unsupported state":      func(c *Certificate) { c.Status = "EXPIRED" },
	} {
		t.Run(name, func(t *testing.T) {
			certificate := validCertificate(now)
			change(&certificate)
			if !errors.Is(certificate.Validate(), ErrInvalidCertificate) {
				t.Fatalf("invalid certificate accepted: %#v", certificate)
			}
		})
	}
}

func TestCourseCompletionAchievementFreezesExactPublishedMetadata(t *testing.T) {
	version, err := courses.ParseVersion("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	achievement, err := CourseCompletionAchievement(courses.ImmutableCourseVersion{ID: "10000000-0000-4000-8000-000000000001", CourseVersion: courses.CourseVersionInput{CourseID: "20000000-0000-4000-8000-000000000001", Status: courses.CourseVersionPublished, Title: "Immutable course", Version: version, SourceLanguage: "en-GB"}}, "Completed the published CourseVersion.")
	if err != nil || achievement != (Achievement{CourseTitle: "Immutable course", CourseVersion: "1.2.3", Language: "en-GB", Criteria: "Completed the published CourseVersion."}) {
		t.Fatalf("achievement = %#v, %v", achievement, err)
	}
}
