package platform

import (
	"errors"
	"net/http"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxLearnerAttemptBodyBytes int64 = 128 * 1024

// learnerAttemptDTO is a private learner projection. It carries the
// learner's own answers and aggregate result, never the private answer key.
type learnerAttemptDTO struct {
	AttemptID     string                      `json:"attemptId"`
	AssessmentKey string                      `json:"assessmentKey"`
	State         assessments.AttemptState    `json:"state"`
	Revision      int64                       `json:"revision"`
	Responses     []learnerAttemptResponseDTO `json:"responses"`
	CreatedAt     time.Time                   `json:"createdAt"`
	UpdatedAt     time.Time                   `json:"updatedAt"`
	SubmittedAt   *time.Time                  `json:"submittedAt"`
	Result        *learnerAttemptResultDTO    `json:"result"`
}

type learnerAttemptResponseDTO struct {
	QuestionKey        string                          `json:"questionKey"`
	Type               assessments.QuestionType        `json:"type"`
	SelectedOptionKey  string                          `json:"selectedOptionKey"`
	SelectedOptionKeys []string                        `json:"selectedOptionKeys"`
	Pairs              []learnerAttemptMatchingPairDTO `json:"pairs"`
}

type learnerAttemptMatchingPairDTO struct {
	LeftItemKey  string `json:"leftItemKey"`
	RightItemKey string `json:"rightItemKey"`
}

type learnerAttemptResultDTO struct {
	CorrectCount int `json:"correctCount"`
	TotalCount   int `json:"totalCount"`
	Percentage   int `json:"percentage"`
}

type learnerAttemptUpdateRequest struct {
	ExpectedRevision *int64                      `json:"expectedRevision"`
	Responses        []learnerAttemptResponseDTO `json:"responses"`
}

type learnerAttemptSubmitRequest struct {
	ExpectedRevision *int64 `json:"expectedRevision"`
}

func (h *authHTTP) handleLearnerAttemptCreate(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	courseID, version, assessmentKey, ok := learnerAttemptContext(w, r)
	if !ok {
		return
	}
	attempt, err := h.learnerAttempts.Start(r.Context(), string(actor.UserID()), courseID, version, assessmentKey)
	if err != nil {
		learnerAttemptProblem(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/learner/assessment-attempts/"+string(attempt.ID))
	writeJSON(w, http.StatusCreated, learnerAttemptDetail(attempt))
}

func (h *authHTTP) handleLearnerAttemptGet(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	id, ok := learnerAttemptID(w, r)
	if !ok {
		return
	}
	attempt, err := h.learnerAttempts.Get(r.Context(), string(actor.UserID()), id)
	if err != nil {
		learnerAttemptProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, learnerAttemptDetail(attempt))
}

func (h *authHTTP) handleLearnerAttemptUpdate(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	id, ok := learnerAttemptID(w, r)
	if !ok {
		return
	}
	var input learnerAttemptUpdateRequest
	if err := decodeAuthoringBody(w, r, maxLearnerAttemptBodyBytes, &input); err != nil || input.ExpectedRevision == nil || *input.ExpectedRevision < 1 {
		problemCode(w, r, http.StatusBadRequest, "Invalid attempt responses", "invalid_attempt_response")
		return
	}
	attempt, err := h.learnerAttempts.Update(r.Context(), string(actor.UserID()), id, *input.ExpectedRevision, learnerAttemptResponses(input.Responses))
	if err != nil {
		learnerAttemptProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, learnerAttemptDetail(attempt))
}

func (h *authHTTP) handleLearnerAttemptSubmit(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	id, ok := learnerAttemptID(w, r)
	if !ok {
		return
	}
	var input learnerAttemptSubmitRequest
	if err := decodeAuthoringBody(w, r, maxLearnerAttemptBodyBytes, &input); err != nil || input.ExpectedRevision == nil || *input.ExpectedRevision < 1 {
		problemCode(w, r, http.StatusBadRequest, "Invalid attempt submission", "invalid_attempt_submission")
		return
	}
	attempt, err := h.learnerAttempts.Submit(r.Context(), string(actor.UserID()), id, *input.ExpectedRevision)
	if err != nil {
		learnerAttemptProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, learnerAttemptDetail(attempt))
}

func learnerActor(w http.ResponseWriter, r *http.Request) (identity.AuthenticatedActor, bool) {
	actor, ok := currentAuthenticatedActor(r.Context())
	if !ok {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
	}
	return actor, ok
}

func learnerAttemptContext(w http.ResponseWriter, r *http.Request) (courses.CourseID, courses.Version, assessments.AssessmentID, bool) {
	rawCourseID := chi.URLParam(r, "courseId")
	parsedCourseID, err := uuid.Parse(rawCourseID)
	if err != nil || parsedCourseID.String() != rawCourseID {
		problem(w, r, http.StatusBadRequest, "Invalid course ID")
		return "", courses.Version{}, "", false
	}
	version, err := courses.ParseVersion(chi.URLParam(r, "version"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid course version")
		return "", courses.Version{}, "", false
	}
	assessmentKey, err := assessments.ParseAssessmentID(chi.URLParam(r, "assessmentKey"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid assessment identifier")
		return "", courses.Version{}, "", false
	}
	return courses.CourseID(parsedCourseID.String()), version, assessmentKey, true
}

func learnerAttemptID(w http.ResponseWriter, r *http.Request) (assessments.AttemptID, bool) {
	id, err := assessments.ParseAttemptID(chi.URLParam(r, "attemptId"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid attempt identifier")
		return "", false
	}
	return id, true
}

func learnerAttemptResponses(source []learnerAttemptResponseDTO) []assessments.AttemptResponse {
	result := make([]assessments.AttemptResponse, 0, len(source))
	for _, response := range source {
		pairs := make([]assessments.AttemptMatchingPair, 0, len(response.Pairs))
		for _, pair := range response.Pairs {
			pairs = append(pairs, assessments.AttemptMatchingPair{LeftItemKey: pair.LeftItemKey, RightItemKey: pair.RightItemKey})
		}
		result = append(result, assessments.AttemptResponse{QuestionKey: response.QuestionKey, Type: response.Type, SelectedOptionKey: response.SelectedOptionKey, SelectedOptionKeys: append([]string(nil), response.SelectedOptionKeys...), Pairs: pairs})
	}
	return result
}

func learnerAttemptDetail(attempt assessments.AssessmentAttempt) learnerAttemptDTO {
	responses := make([]learnerAttemptResponseDTO, 0, len(attempt.Responses))
	for _, response := range attempt.Responses {
		pairs := make([]learnerAttemptMatchingPairDTO, 0, len(response.Pairs))
		for _, pair := range response.Pairs {
			pairs = append(pairs, learnerAttemptMatchingPairDTO{LeftItemKey: pair.LeftItemKey, RightItemKey: pair.RightItemKey})
		}
		responses = append(responses, learnerAttemptResponseDTO{QuestionKey: response.QuestionKey, Type: response.Type, SelectedOptionKey: response.SelectedOptionKey, SelectedOptionKeys: nonNilStrings(response.SelectedOptionKeys), Pairs: pairs})
	}
	var result *learnerAttemptResultDTO
	if attempt.Result != nil {
		result = &learnerAttemptResultDTO{CorrectCount: attempt.Result.CorrectCount, TotalCount: attempt.Result.TotalCount, Percentage: attempt.Result.CorrectCount * 100 / attempt.Result.TotalCount}
	}
	return learnerAttemptDTO{AttemptID: string(attempt.ID), AssessmentKey: string(attempt.AssessmentKey), State: attempt.State, Revision: attempt.Revision, Responses: responses, CreatedAt: attempt.CreatedAt, UpdatedAt: attempt.UpdatedAt, SubmittedAt: attempt.SubmittedAt, Result: result}
}

func learnerAttemptProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, assessments.ErrAttemptNotFound), errors.Is(err, assessments.ErrAttemptUnavailable), errors.Is(err, courses.ErrNotFound):
		problem(w, r, http.StatusNotFound, "Not found")
	case errors.Is(err, assessments.ErrInvalidAttempt):
		problemCode(w, r, http.StatusBadRequest, "Invalid attempt responses", "invalid_attempt_response")
	case errors.Is(err, assessments.ErrRevisionMismatch):
		problemCode(w, r, http.StatusConflict, "Attempt revision conflict", "attempt_revision_conflict")
	case errors.Is(err, assessments.ErrAttemptImmutable):
		problemCode(w, r, http.StatusConflict, "Attempt already submitted", "attempt_already_submitted")
	default:
		problem(w, r, http.StatusInternalServerError, "Assessment attempt unavailable")
	}
}
