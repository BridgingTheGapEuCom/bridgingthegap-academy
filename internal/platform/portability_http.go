package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/portability"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const portabilityPreviewTTL = 15 * time.Minute

type portabilityExport interface {
	Write(context.Context, courses.CourseID, courses.Version, io.Writer) error
}
type portabilityImport interface {
	Import(context.Context, *portability.ValidatedCoursePackage) (portability.ImportResult, error)
}
type portabilityPackageReader interface {
	Read(context.Context, io.Reader) (*portability.ValidatedCoursePackage, error)
}
type portabilityCourseReader interface {
	GetImmutableCourseVersionByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error)
}

type portabilityPreviewSession struct {
	owner        identity.UserID
	ownerSession identity.SessionID
	packageValue *portability.ValidatedCoursePackage
	expiresAt    time.Time
}
type portabilityPreviewStore struct {
	mu       sync.Mutex
	now      func() time.Time
	sessions map[string]portabilityPreviewSession
}

func newPortabilityPreviewStore(now func() time.Time) *portabilityPreviewStore {
	if now == nil {
		now = time.Now
	}
	return &portabilityPreviewStore{now: now, sessions: map[string]portabilityPreviewSession{}}
}
func (s *portabilityPreviewStore) create(owner identity.UserID, session identity.SessionID, p *portability.ValidatedCoursePackage) (string, portability.ImportPreview, error) {
	if s == nil || owner == "" || session == "" || p == nil {
		return "", portability.ImportPreview{}, errors.New("invalid preview")
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", portability.ImportPreview{}, err
	}
	token := hex.EncodeToString(raw[:])
	preview := p.Preview()
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purge(now)
	s.sessions[token] = portabilityPreviewSession{owner: owner, ownerSession: session, packageValue: p, expiresAt: now.Add(portabilityPreviewTTL)}
	return token, preview, nil
}
func (s *portabilityPreviewStore) get(owner identity.UserID, session identity.SessionID, token string) (*portability.ValidatedCoursePackage, bool) {
	if s == nil || owner == "" || session == "" || len(token) != 64 {
		return nil, false
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purge(now)
	x, ok := s.sessions[token]
	if !ok || x.owner != owner || x.ownerSession != session {
		return nil, false
	}
	return x.packageValue, true
}
func (s *portabilityPreviewStore) delete(token string) {
	if s != nil {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
	}
}
func (s *portabilityPreviewStore) purge(now time.Time) {
	for token, x := range s.sessions {
		if !x.expiresAt.After(now) {
			delete(s.sessions, token)
		}
	}
}

type portabilityPreviewDTO struct {
	Token   string                     `json:"previewToken"`
	Preview portabilityPreviewDTOValue `json:"preview"`
}
type portabilityPreviewDTOValue struct {
	Format               string                    `json:"format"`
	FormatVersion        int                       `json:"formatVersion"`
	Title                string                    `json:"title"`
	Version              string                    `json:"version"`
	Language             string                    `json:"language"`
	License              portability.License       `json:"license"`
	Attribution          []portability.Attribution `json:"attribution"`
	ModuleCount          int                       `json:"moduleCount"`
	LessonCount          int                       `json:"lessonCount"`
	AssessmentCount      int                       `json:"assessmentCount"`
	AssetCount           int                       `json:"assetCount"`
	AssetBytes           int64                     `json:"assetBytes"`
	TranslationLanguages []string                  `json:"translationLanguages"`
}
type portabilityImportDTO struct {
	ImportID             string                        `json:"importId"`
	CourseID             string                        `json:"courseId"`
	CourseVersionID      string                        `json:"courseVersionId"`
	SemVer               string                        `json:"semVer"`
	Status               portability.ImportDisposition `json:"status"`
	TranslationLanguages []string                      `json:"translationLanguages"`
}

func previewDTO(token string, p portability.ImportPreview) portabilityPreviewDTO {
	return portabilityPreviewDTO{Token: token, Preview: portabilityPreviewDTOValue{p.Format, p.FormatVersion, p.Title, p.Version, p.Language, p.License, p.Attribution, p.ModuleCount, p.LessonCount, p.AssessmentCount, p.AssetCount, p.AssetBytes, p.TranslationLanguages}}
}
func importDTO(r portability.ImportResult) portabilityImportDTO {
	return portabilityImportDTO{r.ImportID, string(r.CourseID), string(r.CourseVersionID), r.Version.String(), r.Disposition, r.TranslationLanguages}
}

func (h *authHTTP) handlePortabilityPreview(w http.ResponseWriter, r *http.Request) {
	actor, ok := currentAuthenticatedActor(r.Context())
	if !ok || h.portabilityReader == nil || h.portabilityPreviews == nil {
		problem(w, r, http.StatusInternalServerError, "Import unavailable")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/zip" {
		problemCode(w, r, http.StatusBadRequest, "Invalid package upload", "invalid_archive")
		return
	}
	limit := portability.DefaultLimits().MaxCompressedBytes
	pkg, err := h.portabilityReader.Read(r.Context(), http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		portabilityProblem(w, r, err)
		return
	}
	token, preview, err := h.portabilityPreviews.create(actor.UserID(), actor.SessionID(), pkg)
	if err != nil {
		problem(w, r, http.StatusInternalServerError, "Import unavailable")
		return
	}
	writeJSON(w, http.StatusOK, previewDTO(token, preview))
}
func (h *authHTTP) handlePortabilityExecute(w http.ResponseWriter, r *http.Request) {
	actor, ok := currentAuthenticatedActor(r.Context())
	if !ok || h.portabilityImporter == nil || h.portabilityPreviews == nil {
		problem(w, r, http.StatusInternalServerError, "Import unavailable")
		return
	}
	if body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1)); err != nil || len(body) != 0 {
		problem(w, r, http.StatusBadRequest, "Invalid import request")
		return
	}
	token := chi.URLParam(r, "previewToken")
	pkg, found := h.portabilityPreviews.get(actor.UserID(), actor.SessionID(), token)
	if !found {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	result, err := h.portabilityImporter.Import(r.Context(), pkg)
	if err != nil {
		if errors.Is(err, portability.ErrImportSourceVersionConflict) {
			h.portabilityPreviews.delete(token)
			problemCode(w, r, http.StatusConflict, "Package source version conflicts with an existing import", "package_source_version_conflict")
			return
		}
		if errors.Is(err, portability.ErrImportLocalCourseVersionConflict) {
			h.portabilityPreviews.delete(token)
			problemCode(w, r, http.StatusConflict, "Package version conflicts with an existing Course", "local_course_version_conflict")
			return
		}
		portabilityProblem(w, r, err)
		return
	}
	h.portabilityPreviews.delete(token)
	writeJSON(w, http.StatusOK, importDTO(result))
}
func (h *authHTTP) handleCoursePackageExport(w http.ResponseWriter, r *http.Request) {
	if h.portabilityExporter == nil || h.portabilityCourses == nil {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	actor, ok := currentAuthenticatedActor(r.Context())
	if !ok {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return
	}
	raw := chi.URLParam(r, "courseId")
	if id, err := uuid.Parse(raw); err != nil || id.String() != raw {
		problem(w, r, http.StatusBadRequest, "Invalid course ID")
		return
	}
	version, err := courses.ParseVersion(chi.URLParam(r, "version"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid course version")
		return
	}
	v, err := h.portabilityCourses.GetImmutableCourseVersionByCourseAndVersion(r.Context(), courses.CourseID(raw), version)
	if err != nil {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	allowed := false
	for _, contributor := range v.CourseVersion.Attribution {
		if contributor.UserID == string(actor.UserID()) && (contributor.Role == courses.ContributorAuthor || contributor.Role == courses.ContributorMaintainer) {
			allowed = true
			break
		}
	}
	if !allowed {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	tmp, err := os.CreateTemp("", "btg-course-export-*.zip")
	if err != nil {
		problem(w, r, http.StatusInternalServerError, "Export unavailable")
		return
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := h.portabilityExporter.Write(r.Context(), courses.CourseID(raw), version, tmp); err != nil {
		_ = tmp.Close()
		portabilityExportProblem(w, r, err)
		return
	}
	if err := tmp.Close(); err != nil {
		problem(w, r, http.StatusInternalServerError, "Export unavailable")
		return
	}
	file, err := os.Open(tmpName)
	if err != nil {
		problem(w, r, http.StatusInternalServerError, "Export unavailable")
		return
	}
	defer func() { _ = file.Close() }()
	filename := safeExportFilename(v.CourseVersion.Title, version.String())
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	if _, err := io.Copy(w, file); err != nil {
		return
	}
}
func safeExportFilename(title, version string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	title = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, title)
	title = strings.Trim(title, "-")
	if title == "" {
		title = "course"
	}
	return title + "-" + version + ".btg-course.zip"
}
func portabilityProblem(w http.ResponseWriter, r *http.Request, err error) {
	var p *portability.PackageError
	if errors.As(err, &p) {
		problemCode(w, r, http.StatusUnprocessableEntity, "Invalid course package", string(p.Code))
		return
	}
	if errors.Is(err, portability.ErrImportStorageFailed) || errors.Is(err, portability.ErrImportPersistenceFailed) || errors.Is(err, portability.ErrImportCleanupFailed) {
		problemCode(w, r, http.StatusServiceUnavailable, "Import unavailable", "import_persistence_failed")
		return
	}
	problem(w, r, http.StatusInternalServerError, "Import unavailable")
}

func portabilityExportProblem(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, portability.ErrExportNotFound) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	problem(w, r, http.StatusInternalServerError, "Export unavailable")
}
