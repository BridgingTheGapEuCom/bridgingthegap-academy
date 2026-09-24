package portability

import (
	"context"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

var (
	ErrImportSourceVersionConflict      = errors.New("package source version conflicts with an existing import")
	ErrImportLocalCourseVersionConflict = errors.New("package version conflicts with an existing local course version")
	ErrImportStorageFailed              = errors.New("package asset storage failed")
	ErrImportPersistenceFailed          = errors.New("package import persistence failed")
	ErrImportCleanupFailed              = errors.New("package import storage cleanup failed")
	ErrImportUnavailable                = errors.New("package import persistence unavailable")
)

type ImportDisposition string

const (
	ImportCreated  ImportDisposition = "IMPORTED"
	ImportReplayed ImportDisposition = "REPLAYED"
)

type ImportResult struct {
	ImportID             string
	CourseID             courses.CourseID
	CourseVersionID      courses.CourseVersionID
	Version              courses.Version
	Disposition          ImportDisposition
	TranslationLanguages []string
}

// ImportRepository is the sole persistence port. Its Import method must commit
// the course mapping, immutable CourseVersion, asset bindings, translations,
// and provenance record as one logical transaction. The importer cannot accept
// raw package DTOs because importPlan is package-private.
type ImportRepository interface {
	Import(context.Context, importPlan) (ImportResult, error)
}
type importPlan struct {
	fingerprint  string
	manifest     Manifest
	course       CoursePayload
	assessments  AssessmentsPayload
	translations []TranslationPayload
	assets       []validatedAsset
	importedAt   time.Time
}
type Importer struct {
	repository ImportRepository
	now        func() time.Time
}

func NewImporter(repository ImportRepository, now func() time.Time) (*Importer, error) {
	if repository == nil {
		return nil, ErrImportUnavailable
	}
	if now == nil {
		now = time.Now
	}
	return &Importer{repository, now}, nil
}

// Import accepts only the package-private validated value produced by Reader.
// A raw ZIP cannot cross this boundary. Duplicate and collision handling are
// delegated to a transactionally-safe persistence implementation.
func (i *Importer) Import(ctx context.Context, pkg *ValidatedCoursePackage) (ImportResult, error) {
	if i == nil || i.repository == nil || pkg == nil || pkg.digest == "" {
		return ImportResult{}, ErrImportUnavailable
	}
	return i.repository.Import(ctx, importPlan{fingerprint: pkg.digest, manifest: pkg.manifest, course: pkg.course, assessments: pkg.assessments, translations: pkg.translations, assets: pkg.assets, importedAt: i.now().UTC()})
}
