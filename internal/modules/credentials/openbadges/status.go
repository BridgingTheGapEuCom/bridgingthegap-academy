package openbadges

import "errors"

var ErrStatusEntryNotFound = errors.New("open badges status entry not found")

const RevocationPurpose = "revocation"
const DefaultStatusListCapacity = 131072

type StatusListEntry struct {
	CertificateID   string
	StatusListID    string
	StatusListIndex int
	StatusPurpose   string
}

func (e StatusListEntry) Validate() error {
	if !uuid(e.CertificateID) || !uuid(e.StatusListID) || e.StatusListIndex < 0 || e.StatusPurpose != RevocationPurpose {
		return ErrInvalidCertificate
	}
	return nil
}
func uuid(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// StatusReference is prepared for a future signed BitstringStatusListEntry.
// Allocation says where an immutable credential belongs; Certificate lifecycle
// remains the only revocation truth.
type StatusReference struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	StatusPurpose        string `json:"statusPurpose"`
	StatusListIndex      string `json:"statusListIndex"`
	StatusListCredential string `json:"statusListCredential"`
}
