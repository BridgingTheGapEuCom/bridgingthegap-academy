package platform

import (
	"errors"
	"net/http"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

const maxAuthoringReviewBodyBytes int64 = 32 * 1024

type authoringReviewSubmitRequest struct {
	ExpectedDraftRevision *int64 `json:"expectedDraftRevision"`
}

type authoringReviewDecisionRequest struct {
	ExpectedReviewRevision *int64                `json:"expectedReviewRevision"`
	Message                optionalField[string] `json:"message"`
}

type authoringReviewDTO struct {
	ID                    string                 `json:"id"`
	DraftID               string                 `json:"draftId"`
	DraftRevision         int64                  `json:"draftRevision"`
	SnapshotSchemaVersion int                    `json:"snapshotSchemaVersion"`
	Status                authoring.ReviewStatus `json:"status"`
	ReviewRevision        int64                  `json:"reviewRevision"`
	SubmittedBy           string                 `json:"submittedBy"`
	SubmittedAt           time.Time              `json:"submittedAt"`
	DecidedBy             *string                `json:"decidedBy"`
	DecidedAt             *time.Time             `json:"decidedAt"`
}

type authoringReviewDetailDTO struct {
	Review      authoringReviewDTO                  `json:"review"`
	Snapshot    authoring.ReviewSnapshot            `json:"snapshot"`
	Publication authoringReviewPublicationStatusDTO `json:"publication"`
}

// Submission returns the frozen Review evidence without the optional
// publication-status read. A successful Review creation must not be reported
// as failed because a later, independent publication-fact lookup is down.
type authoringReviewSubmissionDetailDTO struct {
	Review   authoringReviewDTO       `json:"review"`
	Snapshot authoring.ReviewSnapshot `json:"snapshot"`
}

type authoringReviewPublicationStatusDTO struct {
	CanPublish  bool                            `json:"canPublish"`
	Publishable bool                            `json:"publishable"`
	Issues      []publicationValidationIssueDTO `json:"issues"`
	Published   *authoringReviewPublishedDTO    `json:"published"`
}

type authoringReviewPublishedDTO struct {
	CourseID      string    `json:"courseId"`
	CourseVersion string    `json:"courseVersion"`
	PublishedAt   time.Time `json:"publishedAt"`
}

func (a *authHTTP) handleAuthoringReviewSubmit(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	expected, _, err := decodeAuthoringReviewBody(w, r, false)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid review submission")
		return
	}
	cycle, snapshot, err := a.authoringReviews.Submit(r.Context(), actor, draftID, expected)
	if err != nil {
		authoringReviewMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, authoringReviewSubmissionDetailDTO{Review: authoringReview(cycle), Snapshot: snapshot})
}

func (a *authHTTP) handleAuthoringReviewActive(w http.ResponseWriter, r *http.Request) {
	a.handleAuthoringReviewCurrent(w, r, true)
}

func (a *authHTTP) handleAuthoringReviewLatest(w http.ResponseWriter, r *http.Request) {
	a.handleAuthoringReviewCurrent(w, r, false)
}

func (a *authHTTP) handleAuthoringReviewCurrent(w http.ResponseWriter, r *http.Request, active bool) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	var cycle authoring.ReviewCycle
	var err error
	if active {
		cycle, _, err = a.authoringReviews.Active(r.Context(), actor, draftID)
	} else {
		cycle, _, err = a.authoringReviews.Latest(r.Context(), actor, draftID)
	}
	if err != nil {
		authoringReviewProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringReview(cycle))
}

func (a *authHTTP) handleAuthoringReviewHistory(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	cycles, err := a.authoringReviews.History(r.Context(), actor, draftID)
	if err != nil {
		authoringReviewProblem(w, r, err)
		return
	}
	reviews := make([]authoringReviewDTO, 0, len(cycles))
	for _, cycle := range cycles {
		reviews = append(reviews, authoringReview(cycle))
	}
	writeJSON(w, http.StatusOK, struct {
		Reviews []authoringReviewDTO `json:"reviews"`
	}{Reviews: reviews})
}

func (a *authHTTP) handleAuthoringReview(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	reviewID, ok := authoringReviewID(w, r)
	if !ok {
		return
	}
	cycle, snapshot, err := a.authoringReviews.Review(r.Context(), actor, draftID, reviewID)
	if err != nil {
		authoringReviewProblem(w, r, err)
		return
	}
	detail, err := a.authoringReviewDetail(r, actor, cycle, snapshot)
	if err != nil {
		authoringReviewProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (a *authHTTP) authoringReviewDetail(r *http.Request, actor identity.AuthenticatedActor, cycle authoring.ReviewCycle, snapshot authoring.ReviewSnapshot) (authoringReviewDetailDTO, error) {
	validation := authoring.NewPublicationValidator().Validate(cycle, &snapshot)
	status := authoring.ReviewPublicationStatus{Publishable: validation.Publishable, Issues: validation.Issues}
	if a.authoringPublicationStatus != nil {
		var err error
		status, err = a.authoringPublicationStatus.Status(r.Context(), actor, cycle, snapshot)
		if err != nil {
			return authoringReviewDetailDTO{}, err
		}
	}
	issues := make([]publicationValidationIssueDTO, 0, len(status.Issues))
	for _, issue := range status.Issues {
		issues = append(issues, publicationValidationIssueDTO{Code: issue.Code, Path: issue.Path, Message: issue.Message})
	}
	publication := authoringReviewPublicationStatusDTO{
		CanPublish: status.CanPublish, Publishable: status.Publishable, Issues: issues,
	}
	if status.Published != nil {
		publication.Published = &authoringReviewPublishedDTO{
			CourseID: string(status.Published.CourseID), CourseVersion: status.Published.CourseVersion.String(), PublishedAt: status.Published.PublishedAt,
		}
	}
	return authoringReviewDetailDTO{Review: authoringReview(cycle), Snapshot: snapshot, Publication: publication}, nil
}

func (a *authHTTP) handleAuthoringReviewApprove(w http.ResponseWriter, r *http.Request) {
	a.handleAuthoringReviewDecision(w, r, authoring.ReviewApproved)
}

func (a *authHTTP) handleAuthoringReviewRequestChanges(w http.ResponseWriter, r *http.Request) {
	a.handleAuthoringReviewDecision(w, r, authoring.ReviewChangesRequested)
}

func (a *authHTTP) handleAuthoringReviewDecision(w http.ResponseWriter, r *http.Request, decision authoring.ReviewStatus) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	reviewID, ok := authoringReviewID(w, r)
	if !ok {
		return
	}
	expected, message, err := decodeAuthoringReviewBody(w, r, true)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid review decision")
		return
	}
	var cycle authoring.ReviewCycle
	if decision == authoring.ReviewApproved {
		cycle, err = a.authoringReviews.Approve(r.Context(), actor, draftID, reviewID, expected, message)
	} else {
		cycle, err = a.authoringReviews.RequestChanges(r.Context(), actor, draftID, reviewID, expected, message)
	}
	if err != nil {
		authoringReviewMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringReview(cycle))
}

func decodeAuthoringReviewBody(w http.ResponseWriter, r *http.Request, decision bool) (int64, string, error) {
	if !decision {
		var input authoringReviewSubmitRequest
		if err := decodeAuthoringBody(w, r, maxAuthoringReviewBodyBytes, &input); err != nil || input.ExpectedDraftRevision == nil || *input.ExpectedDraftRevision < 1 {
			return 0, "", errors.New("invalid review submission")
		}
		return *input.ExpectedDraftRevision, "", nil
	}
	var input authoringReviewDecisionRequest
	if err := decodeAuthoringBody(w, r, maxAuthoringReviewBodyBytes, &input); err != nil || input.ExpectedReviewRevision == nil || *input.ExpectedReviewRevision < 1 || input.Message.null {
		return 0, "", errors.New("invalid review decision")
	}
	if input.Message.set && authoring.ValidateReviewMessage(input.Message.value) != nil {
		return 0, "", errors.New("invalid review message")
	}
	return *input.ExpectedReviewRevision, input.Message.value, nil
}

func authoringReviewID(w http.ResponseWriter, r *http.Request) (authoring.ReviewID, bool) {
	id, ok := authoringUUIDParam(w, r, "reviewId", "Invalid review identifier")
	return authoring.ReviewID(id), ok
}

func authoringReview(cycle authoring.ReviewCycle) authoringReviewDTO {
	var decidedBy *string
	if cycle.DecidedByUserID != "" {
		value := cycle.DecidedByUserID
		decidedBy = &value
	}
	return authoringReviewDTO{
		ID: string(cycle.ID), DraftID: string(cycle.DraftID), DraftRevision: cycle.DraftRevision,
		SnapshotSchemaVersion: cycle.SnapshotSchemaVersion, Status: cycle.Status, ReviewRevision: cycle.Revision,
		SubmittedBy: cycle.SubmittedByUserID, SubmittedAt: cycle.SubmittedAt, DecidedBy: decidedBy, DecidedAt: cycle.DecidedAt,
	}
}

func authoringReviewProblem(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, authoring.ErrNotFound) || errors.Is(err, authoring.ErrReviewNotFound) || errors.Is(err, authoring.ErrAuthorizationDenied) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	problem(w, r, http.StatusInternalServerError, "Authoring review service unavailable")
}

func authoringReviewMutationProblem(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, authoring.ErrNotFound) || errors.Is(err, authoring.ErrReviewNotFound) || errors.Is(err, authoring.ErrAuthorizationDenied) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	if errors.Is(err, authoring.ErrIndependentReviewerRequired) {
		problemCode(w, r, http.StatusConflict, "Independent reviewer required", "independent_reviewer_required")
		return
	}
	if errors.Is(err, authoring.ErrRevisionMismatch) || errors.Is(err, authoring.ErrReviewStale) || errors.Is(err, authoring.ErrReviewInvalidState) || errors.Is(err, authoring.ErrReviewAlreadyExists) || errors.Is(err, authoring.ErrConflict) || errors.Is(err, authoring.ErrInvalidState) {
		problem(w, r, http.StatusConflict, "Review conflict")
		return
	}
	problem(w, r, http.StatusInternalServerError, "Authoring review service unavailable")
}
