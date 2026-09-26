package platform

import (
	"errors"
	"net/http"
	"strings"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func writeVC(w http.ResponseWriter, body []byte, cache string) {
	w.Header().Set("Content-Type", "application/vc")
	w.Header().Set("Cache-Control", cache)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
func (h *authHTTP) handleSignedOpenBadge(w http.ResponseWriter, r *http.Request) {
	id, ok := certificateID(w, r)
	if !ok {
		return
	}
	doc, err := h.badgePublication.Credential(r.Context(), id)
	if errors.Is(err, credentials.ErrCertificateNotFound) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	if err != nil {
		problem(w, r, http.StatusServiceUnavailable, "Signed credential unavailable")
		return
	}
	writeVC(w, doc, "public, max-age=31536000, immutable")
}
func (h *authHTTP) handleSignedStatusList(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "listId")
	parsed, err := uuid.Parse(listID)
	if err != nil || parsed.String() != listID {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	doc, err := h.badgePublication.Status(r.Context(), listID)
	if errors.Is(err, openbadges.ErrStatusEntryNotFound) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	if err != nil {
		problem(w, r, http.StatusServiceUnavailable, "Status list unavailable")
		return
	}
	writeVC(w, doc, "no-store")
}
func (h *authHTTP) handleOpenBadgesIssuer(w http.ResponseWriter, r *http.Request) {
	doc, err := h.badgePublication.Controller(r.Context())
	if err != nil {
		problem(w, r, http.StatusServiceUnavailable, "Issuer unavailable")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, doc)
}
func (h *authHTTP) handleOpenBadgesAchievement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "versionId")
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	version, err := h.achievementVersions.GetImmutableCourseVersion(r.Context(), courses.CourseVersionID(id))
	if err != nil || version.CourseVersion.Status != courses.CourseVersionPublished {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	// The exact published version, rather than latest Course metadata, supplies
	// this public resource. It contains no learner identity.
	w.Header().Set("Cache-Control", "public, max-age=86400")
	writeJSON(w, http.StatusOK, map[string]any{"@context": []string{"https://purl.imsglobal.org/spec/ob/v3p0/context-3.0.3.json"}, "id": strings.TrimRight(h.badgePublicOrigin, "/") + "/achievements/course-versions/" + id, "type": "Achievement", "name": version.CourseVersion.Title, "description": "Certificate awarded for completing " + version.CourseVersion.Title + " version " + version.CourseVersion.Version.String() + ".", "creator": map[string]any{"id": h.badgeIssuer.ID, "type": []string{"Profile"}, "name": h.badgeIssuer.Name}, "version": version.CourseVersion.Version.String(), "inLanguage": version.CourseVersion.SourceLanguage, "criteria": map[string]string{"narrative": credentials.CourseCompletionCriteriaText}})
}
