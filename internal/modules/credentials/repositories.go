package credentials

import (
	"context"
	"time"
)

// Repository is a narrow private persistence boundary. It intentionally has
// no search, public verification, export, or deletion operations.
type Repository interface {
	Create(context.Context, CertificateInput, time.Time) (Certificate, error)
	GetByID(context.Context, CertificateID) (Certificate, error)
	GetByLearnerAndCourseVersion(context.Context, string, string) (Certificate, error)
	Revoke(context.Context, CertificateID, time.Time) (Certificate, error)
}
