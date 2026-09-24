package openbadges

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"net/url"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
)

var (
	ErrInvalidStatusList        = errors.New("invalid Open Badges status list")
	ErrStatusListFull           = errors.New("open badges status list full")
	ErrMissingStatusCertificate = errors.New("status list Certificate missing")
	ErrStatusEncoding           = errors.New("status list encoding failed")
)

// StatusList is an immutable allocation identity; EntryCount is not part of
// its payload. Certificate lifecycle is the sole source of revocation truth.
type StatusList struct {
	ID                string
	Capacity          int
	StatusPurpose     string
	LifecycleRevision int64
}
type StatusFact struct {
	Entry              StatusListEntry
	CertificateStatus  credentials.CertificateStatus
	CertificatePresent bool
}

func ValidID(id string) bool { return uuid(id) }

// BuildEncodedList follows W3C Bitstring Status List v1.0: index zero is the
// most significant bit of the first byte, then GZIP and multibase base64url.
func BuildEncodedList(list StatusList, facts []StatusFact) (string, error) {
	if !uuid(list.ID) || list.Capacity < DefaultStatusListCapacity || list.Capacity%8 != 0 || list.StatusPurpose != RevocationPurpose {
		return "", ErrInvalidStatusList
	}
	raw := make([]byte, list.Capacity/8)
	seen := make(map[int]struct{}, len(facts))
	for _, fact := range facts {
		if !fact.CertificatePresent {
			return "", ErrMissingStatusCertificate
		}
		e := fact.Entry
		if e.Validate() != nil || e.StatusListID != list.ID || e.StatusListIndex >= list.Capacity {
			return "", ErrInvalidStatusList
		}
		if _, exists := seen[e.StatusListIndex]; exists {
			return "", ErrInvalidStatusList
		}
		seen[e.StatusListIndex] = struct{}{}
		switch fact.CertificateStatus {
		case credentials.CertificateActive:
		case credentials.CertificateRevoked:
			raw[e.StatusListIndex/8] |= byte(1 << (7 - e.StatusListIndex%8))
		default:
			return "", ErrInvalidStatusList
		}
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(raw); err != nil {
		return "", errors.Join(ErrStatusEncoding, err)
	}
	if err := writer.Close(); err != nil {
		return "", errors.Join(ErrStatusEncoding, err)
	}
	return "u" + base64.RawURLEncoding.EncodeToString(compressed.Bytes()), nil
}

// UnsignedStatusListCredential is an internal snapshot only. It must be
// cryptographically secured before any public publication.
type UnsignedStatusListCredential struct {
	Context           []string          `json:"@context"`
	ID                string            `json:"id"`
	Type              []string          `json:"type"`
	Issuer            Profile           `json:"issuer"`
	ValidFrom         string            `json:"validFrom"`
	CredentialSubject StatusListSubject `json:"credentialSubject"`
}
type StatusListSubject struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	StatusPurpose string `json:"statusPurpose"`
	EncodedList   string `json:"encodedList"`
}

func (m *Mapper) BuildUnsignedStatusListCredential(list StatusList, facts []StatusFact, issuer credentials.Issuer, validFrom time.Time) (UnsignedStatusListCredential, error) {
	if m == nil || m.base == nil || validFrom.IsZero() {
		return UnsignedStatusListCredential{}, ErrInvalidConfiguration
	}
	p, err := profile(issuer)
	if err != nil {
		return UnsignedStatusListCredential{}, ErrInvalidConfiguration
	}
	encoded, err := BuildEncodedList(list, facts)
	if err != nil {
		return UnsignedStatusListCredential{}, err
	}
	id := m.url("open-badges", "status", "revocation", list.ID)
	u, err := url.Parse(id)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return UnsignedStatusListCredential{}, ErrInvalidConfiguration
	}
	return UnsignedStatusListCredential{Context: []string{VCContext}, ID: id, Type: []string{"VerifiableCredential", "BitstringStatusListCredential"}, Issuer: p, ValidFrom: validFrom.UTC().Format(time.RFC3339), CredentialSubject: StatusListSubject{ID: id + "#list", Type: "BitstringStatusList", StatusPurpose: RevocationPurpose, EncodedList: encoded}}, nil
}
