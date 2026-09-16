package platform

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

const maxAuthoringPublicationBodyBytes int64 = 16 * 1024

type authoringPublicationRequest struct {
	ExpectedReviewRevision *int64 `json:"expectedReviewRevision"`
}

type authoringPublicationDTO struct {
	ReviewID        string    `json:"reviewId"`
	ReviewRevision  int64     `json:"reviewRevision"`
	CourseID        string    `json:"courseId"`
	CourseVersion   string    `json:"courseVersion"`
	CourseVersionID string    `json:"courseVersionId"`
	PublishedAt     time.Time `json:"publishedAt"`
}

type publicationValidationIssueDTO struct {
	Code    authoring.PublicationValidationCode `json:"code"`
	Path    string                              `json:"path"`
	Message string                              `json:"message"`
}

func (a *authHTTP) handleAuthoringReviewPublish(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	reviewID, ok := authoringReviewID(w, r)
	if !ok {
		return
	}
	var input authoringPublicationRequest
	if err := decodeAuthoringBody(w, r, maxAuthoringPublicationBodyBytes, &input); err != nil || input.ExpectedReviewRevision == nil || *input.ExpectedReviewRevision < 1 {
		problem(w, r, http.StatusBadRequest, "Invalid publication request")
		return
	}
	result, err := a.authoringPublications.Publish(r.Context(), actor, draftID, reviewID, *input.ExpectedReviewRevision)
	if err != nil {
		authoringPublicationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringPublicationDTO{
		ReviewID: string(result.ReviewID), ReviewRevision: result.ReviewRevision,
		CourseID: string(result.CourseID), CourseVersion: result.CourseVersion.String(),
		CourseVersionID: string(result.CourseVersionID), PublishedAt: result.PublishedAt,
	})
}

func authoringPublicationProblem(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, authoring.ErrNotFound) || errors.Is(err, authoring.ErrReviewNotFound) || errors.Is(err, authoring.ErrAuthorizationDenied) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	if errors.Is(err, authoring.ErrReviewStale) {
		problemCode(w, r, http.StatusConflict, "Review revision conflict", "review_revision_conflict")
		return
	}
	if errors.Is(err, authoring.ErrReviewInvalidState) {
		problemCode(w, r, http.StatusConflict, "Review is not approved", string(authoring.PublicationIssueReviewNotApproved))
		return
	}
	var validation *authoring.PublicationValidationFailure
	if errors.As(err, &validation) {
		publicationValidationProblem(w, r, validation.Result.Issues)
		return
	}
	if errors.Is(err, courses.ErrCourseVersionAlreadyExists) {
		problemCode(w, r, http.StatusConflict, "Course version conflict", "course_version_already_exists")
		return
	}
	if errors.Is(err, authoring.ErrPublicationConflict) || errors.Is(err, authoring.ErrPublicationProvenanceMismatch) {
		problemCode(w, r, http.StatusConflict, "Publication conflict", "publication_conflict")
		return
	}
	problem(w, r, http.StatusInternalServerError, "Publication service unavailable")
}

func publicationValidationProblem(w http.ResponseWriter, r *http.Request, issues []authoring.PublicationValidationIssue) {
	publicIssues := make([]publicationValidationIssueDTO, 0, len(issues))
	for _, issue := range issues {
		publicIssues = append(publicIssues, publicationValidationIssueDTO{Code: issue.Code, Path: issue.Path, Message: issue.Message})
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "about:blank", "title": "Publication validation failed",
		"status": http.StatusUnprocessableEntity, "instance": strings.TrimSpace(r.URL.Path),
		"request_id": r.Context().Value(requestIDKey{}), "code": "publication_validation_failed",
		"issues": publicIssues,
	})
}
