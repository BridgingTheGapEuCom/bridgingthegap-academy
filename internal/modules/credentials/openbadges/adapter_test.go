package openbadges

import (
	"encoding/json"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"strings"
	"testing"
	"time"
)

func certificate() credentials.Certificate {
	at := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	return credentials.Certificate{ID: "11111111-1111-4111-8111-111111111111", LearnerUserID: "22222222-2222-4222-8222-222222222222", CourseID: "33333333-3333-4333-8333-333333333333", CourseVersionID: "44444444-4444-4444-8444-444444444444", Achievement: credentials.Achievement{CourseTitle: "Course one", CourseVersion: "1.0.0", Language: "en", Criteria: "Complete every published lesson."}, Issuer: credentials.Issuer{ID: "https://academy.example/issuer", Name: "Academy"}, IssuedAt: at, Status: credentials.CertificateActive}
}
func TestMapsFrozenCertificateDeterministicallyWithoutProofOrLearnerID(t *testing.T) {
	m, err := NewMapper(Config{PublicBaseURL: "https://academy.example", SubjectSalt: []byte("01234567890123456789012345678901")})
	if err != nil {
		t.Fatal(err)
	}
	c := certificate()
	first, err := m.Map(c)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := m.Map(c)
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) || first.Context[0] != VCContext || first.Context[1] != OBContext || first.ID != "https://academy.example/certificates/"+string(c.ID) || first.CredentialSubject.Achievement.ID != "https://academy.example/achievements/course-versions/"+c.CourseVersionID || first.ValidFrom != "2025-01-02T03:04:05Z" || strings.Contains(string(a), c.LearnerUserID) || strings.Contains(string(a), "proof") || strings.Contains(string(a), "email") {
		t.Fatalf("bad mapping: %s", a)
	}
}
func TestRejectsRevokedAndInvalidConfiguration(t *testing.T) {
	c := certificate()
	at := time.Now().UTC()
	c.Status = credentials.CertificateRevoked
	c.RevokedAt = &at
	m, _ := NewMapper(Config{PublicBaseURL: "https://academy.example", SubjectSalt: []byte("01234567890123456789012345678901")})
	if _, err := m.Map(c); err != ErrRevokedCertificate {
		t.Fatalf("err=%v", err)
	}
	if _, err := NewMapper(Config{PublicBaseURL: "http://academy.example", SubjectSalt: []byte("short")}); err != ErrInvalidConfiguration {
		t.Fatalf("config=%v", err)
	}
}
