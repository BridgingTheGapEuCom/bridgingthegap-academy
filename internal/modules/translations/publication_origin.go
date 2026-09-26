package translations

import (
	"regexp"
	"time"
)

type PublicationOrigin string

const (
	PublicationOriginAuthoring PublicationOrigin = "AUTHORING_PUBLICATION"
	PublicationOriginImported  PublicationOrigin = "IMPORTED_PUBLICATION"
)

type AuthoringPublicationProvenance struct {
	TranslationID TranslationID
	Revision      int64
}

type ImportedPublicationProvenance struct {
	ImportID                      string
	PackageFingerprint            string
	PortableSourceCourseID        string
	PortableSourceCourseVersionID string
	ImportedAt                    time.Time
}

type PublicationProvenance struct {
	Origin    PublicationOrigin
	Authoring *AuthoringPublicationProvenance
	Imported  *ImportedPublicationProvenance
}

var fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (p PublicationProvenance) Validate() error {
	switch p.Origin {
	case PublicationOriginAuthoring:
		if p.Imported != nil || p.Authoring == nil || !validUUID(string(p.Authoring.TranslationID)) || p.Authoring.Revision < 1 {
			return ErrInvalidTranslation
		}
	case PublicationOriginImported:
		if p.Authoring != nil || p.Imported == nil || !validUUID(p.Imported.ImportID) || !fingerprintPattern.MatchString(p.Imported.PackageFingerprint) || p.Imported.PortableSourceCourseID == "" || p.Imported.PortableSourceCourseVersionID == "" || p.Imported.ImportedAt.IsZero() {
			return ErrInvalidTranslation
		}
	default:
		return ErrInvalidTranslation
	}
	return nil
}
