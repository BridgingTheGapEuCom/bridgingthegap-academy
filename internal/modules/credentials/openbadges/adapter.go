// Package openbadges maps the Academy's standards-neutral Certificate to an
// unsigned Open Badges 3.0 / VC 2.0 JSON-LD document. It deliberately owns no
// signing, key, proof, transport, or persistence concerns.
package openbadges

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
)

const VCContext = "https://www.w3.org/ns/credentials/v2"
const OBContext = "https://purl.imsglobal.org/spec/ob/v3p0/context-3.0.3.json"

var (
	ErrInvalidConfiguration = errors.New("invalid Open Badges configuration")
	ErrInvalidCertificate   = errors.New("certificate cannot be mapped to Open Badges")
	ErrRevokedCertificate   = errors.New("revoked certificates require a portable credential status service")
)

type Config struct {
	PublicBaseURL string
	SubjectSalt   []byte
}
type Credential struct {
	Context           []string           `json:"@context"`
	ID                string             `json:"id"`
	Type              []string           `json:"type"`
	Issuer            Profile            `json:"issuer"`
	ValidFrom         string             `json:"validFrom"`
	CredentialSubject AchievementSubject `json:"credentialSubject"`
}
type Profile struct {
	ID   string   `json:"id"`
	Type []string `json:"type"`
	Name string   `json:"name"`
}
type AchievementSubject struct {
	ID          string      `json:"id"`
	Type        []string    `json:"type"`
	Achievement Achievement `json:"achievement"`
}
type Achievement struct {
	ID          string   `json:"id"`
	Type        []string `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Criteria    Criteria `json:"criteria"`
	Creator     Profile  `json:"creator"`
	InLanguage  string   `json:"inLanguage"`
}
type Criteria struct {
	Narrative string `json:"narrative"`
}
type Mapper struct {
	base *url.URL
	salt []byte
}

func NewMapper(config Config) (*Mapper, error) {
	u, err := url.Parse(config.PublicBaseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || len(config.SubjectSalt) < 32 {
		return nil, ErrInvalidConfiguration
	}
	return &Mapper{base: u, salt: append([]byte(nil), config.SubjectSalt...)}, nil
}
func (m *Mapper) Map(c credentials.Certificate) (Credential, error) {
	if m == nil || m.base == nil || len(m.salt) < 32 {
		return Credential{}, ErrInvalidConfiguration
	}
	if c.Validate() != nil {
		return Credential{}, ErrInvalidCertificate
	}
	if c.Status == credentials.CertificateRevoked {
		return Credential{}, ErrRevokedCertificate
	}
	issuer, err := profile(c.Issuer)
	if err != nil {
		return Credential{}, ErrInvalidCertificate
	}
	return Credential{Context: []string{VCContext, OBContext}, ID: m.url("certificates", string(c.ID)), Type: []string{"VerifiableCredential", "OpenBadgeCredential"}, Issuer: issuer, ValidFrom: c.IssuedAt.UTC().Format("2006-01-02T15:04:05Z"), CredentialSubject: AchievementSubject{ID: m.subject(c.LearnerUserID), Type: []string{"AchievementSubject"}, Achievement: Achievement{ID: m.url("achievements", "course-versions", c.CourseVersionID), Type: []string{"Achievement"}, Name: c.Achievement.CourseTitle, Description: "Certificate awarded for completing " + c.Achievement.CourseTitle + " version " + c.Achievement.CourseVersion + ".", Criteria: Criteria{Narrative: c.Achievement.Criteria}, Creator: issuer, InLanguage: c.Achievement.Language}}}, nil
}
func (m *Mapper) url(parts ...string) string {
	u := *m.base
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.Join(parts, "/")
	return u.String()
}
func (m *Mapper) subject(learner string) string {
	h := hmac.New(sha256.New, m.salt)
	h.Write([]byte("btg-openbadges-subject-v1\x00"))
	h.Write([]byte(learner))
	return "urn:btg:subject:" + hex.EncodeToString(h.Sum(nil))
}
func profile(i credentials.Issuer) (Profile, error) {
	u, err := url.Parse(i.ID)
	if err != nil || u.Scheme != "https" || u.Host == "" || i.Validate() != nil {
		return Profile{}, ErrInvalidCertificate
	}
	return Profile{ID: i.ID, Type: []string{"Profile"}, Name: i.Name}, nil
}
