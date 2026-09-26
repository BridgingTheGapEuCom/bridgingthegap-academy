package courses

import (
	"errors"
	"time"
)

// PublicationOrigin records the truthful creation branch for an immutable
// CourseVersion. It is deliberately independent of learner-facing metadata.
type PublicationOrigin string

const (
	PublicationOriginNative   PublicationOrigin = "NATIVE_PUBLICATION"
	PublicationOriginImported PublicationOrigin = "IMPORTED_PUBLICATION"
)

type ImportedPublicationProvenance struct {
	ImportID                      string
	PackageFingerprint            string
	PortableSourceCourseID        string
	PortableSourceCourseVersionID string
	SourceVersion                 Version
	ImportedAt                    time.Time
}
type PublicationProvenance struct {
	Origin   PublicationOrigin
	Native   *CourseVersionProvenance
	Imported *ImportedPublicationProvenance
}

var ErrInvalidPublicationProvenance = errors.New("invalid course publication provenance")

func (p PublicationProvenance) Validate() error {
	switch p.Origin {
	case PublicationOriginNative:
		if p.Native == nil || p.Imported != nil {
			return ErrInvalidPublicationProvenance
		}
		if p.Native.ReviewID == "" || p.Native.DraftID == "" || p.Native.SubmittedByUserID == "" || p.Native.ApprovedByUserID == "" || p.Native.PublishedByUserID == "" || p.Native.ApprovedAt == nil {
			return ErrInvalidPublicationProvenance
		}
	case PublicationOriginImported:
		if p.Native != nil || p.Imported == nil {
			return ErrInvalidPublicationProvenance
		}
		x := p.Imported
		if x.ImportID == "" || len(x.PackageFingerprint) != 64 || x.PortableSourceCourseID == "" || x.PortableSourceCourseVersionID == "" || !x.SourceVersion.Valid() || x.ImportedAt.IsZero() {
			return ErrInvalidPublicationProvenance
		}
	default:
		return ErrInvalidPublicationProvenance
	}
	return nil
}
