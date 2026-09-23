package platform

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Certificate DTOs deliberately project only frozen achievement and issuer
// facts. The public projection has no recipient field at all.
type certificateAchievementDTO struct {
	CourseID string `json:"courseId"`
	Title    string `json:"title"`
	Version  string `json:"version"`
	Language string `json:"language"`
	Criteria string `json:"criteria"`
}
type certificateIssuerDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type learnerCertificateDTO struct {
	CertificateID    string                        `json:"certificateId"`
	Status           credentials.CertificateStatus `json:"status"`
	IssuedAt         time.Time                     `json:"issuedAt"`
	RevokedAt        *time.Time                    `json:"revokedAt"`
	Achievement      certificateAchievementDTO     `json:"achievement"`
	Issuer           certificateIssuerDTO          `json:"issuer"`
	VerificationPath string                        `json:"verificationPath"`
}
type publicCertificateDTO struct {
	CertificateID string                        `json:"certificateId"`
	Status        credentials.CertificateStatus `json:"status"`
	IssuedAt      time.Time                     `json:"issuedAt"`
	RevokedAt     *time.Time                    `json:"revokedAt"`
	Achievement   certificateAchievementDTO     `json:"achievement"`
	Issuer        certificateIssuerDTO          `json:"issuer"`
}

func (h *authHTTP) handleCertificateIssue(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	courseID, version, ok := certificateCourseContext(w, r)
	if !ok {
		return
	}
	// The route is bodyless. A nonempty body can only be an attempt to control
	// immutable, server-owned credential facts.
	if body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1)); err != nil || len(body) != 0 {
		problem(w, r, http.StatusBadRequest, "Invalid certificate request")
		return
	}
	result, err := h.certificateIssuance.Issue(r.Context(), string(actor.UserID()), courseID, version)
	if err != nil {
		certificateProblem(w, r, err)
		return
	}
	if result.Certificate == nil {
		problemCode(w, r, http.StatusConflict, "Course completion is required before a certificate can be issued", "certificate_not_eligible")
		return
	}
	status := http.StatusCreated
	if result.Existing {
		status = http.StatusOK
	}
	w.Header().Set("Location", "/api/learner/certificates/"+string(result.Certificate.ID))
	writeJSON(w, status, learnerCertificateDetail(*result.Certificate))
}
func (h *authHTTP) handleLearnerCertificateGet(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	id, ok := certificateID(w, r)
	if !ok {
		return
	}
	c, err := h.certificates.GetByID(r.Context(), id)
	if err != nil || c.LearnerUserID != string(actor.UserID()) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	writeJSON(w, http.StatusOK, learnerCertificateDetail(c))
}
func (h *authHTTP) handlePublicCertificate(w http.ResponseWriter, r *http.Request) {
	id, ok := certificateID(w, r)
	if !ok {
		return
	}
	c, err := h.certificates.GetByID(r.Context(), id)
	if err != nil {
		certificateProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, publicCertificateDetail(c))
}
func certificateCourseContext(w http.ResponseWriter, r *http.Request) (courses.CourseID, courses.Version, bool) {
	raw := chi.URLParam(r, "courseId")
	parsed, err := uuid.Parse(raw)
	if err != nil || parsed.String() != raw {
		problem(w, r, http.StatusBadRequest, "Invalid course ID")
		return "", courses.Version{}, false
	}
	version, err := courses.ParseVersion(chi.URLParam(r, "version"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid course version")
		return "", courses.Version{}, false
	}
	return courses.CourseID(raw), version, true
}
func certificateID(w http.ResponseWriter, r *http.Request) (credentials.CertificateID, bool) {
	raw := chi.URLParam(r, "certificateId")
	parsed, err := uuid.Parse(raw)
	if err != nil || parsed.String() != raw {
		problem(w, r, http.StatusNotFound, "Not found")
		return "", false
	}
	return credentials.CertificateID(raw), true
}
func learnerCertificateDetail(c credentials.Certificate) learnerCertificateDTO {
	return learnerCertificateDTO{CertificateID: string(c.ID), Status: c.Status, IssuedAt: c.IssuedAt, RevokedAt: c.RevokedAt, Achievement: certificateAchievementDTO{CourseID: c.CourseID, Title: c.Achievement.CourseTitle, Version: c.Achievement.CourseVersion, Language: c.Achievement.Language, Criteria: c.Achievement.Criteria}, Issuer: certificateIssuerDTO{ID: c.Issuer.ID, Name: c.Issuer.Name}, VerificationPath: "/verify/certificates/" + string(c.ID)}
}
func publicCertificateDetail(c credentials.Certificate) publicCertificateDTO {
	return publicCertificateDTO{CertificateID: string(c.ID), Status: c.Status, IssuedAt: c.IssuedAt, RevokedAt: c.RevokedAt, Achievement: certificateAchievementDTO{CourseID: c.CourseID, Title: c.Achievement.CourseTitle, Version: c.Achievement.CourseVersion, Language: c.Achievement.Language, Criteria: c.Achievement.Criteria}, Issuer: certificateIssuerDTO{ID: c.Issuer.ID, Name: c.Issuer.Name}}
}
func certificateProblem(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, credentials.ErrCertificateNotFound) || errors.Is(err, courses.ErrNotFound) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	problem(w, r, http.StatusInternalServerError, "Certificate unavailable")
}
