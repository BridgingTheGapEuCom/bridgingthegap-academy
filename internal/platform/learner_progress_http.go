package platform

import (
	"errors"
	"net/http"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/progress"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type learnerProgressDTO struct {
	Revision            int64     `json:"revision"`
	CompletedLessonKeys []string  `json:"completedLessonKeys"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type learnerProgressCompleteRequest struct {
	ExpectedRevision *int64 `json:"expectedRevision"`
}

func (h *authHTTP) handleLearnerProgressGet(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	courseID, version, ok := learnerProgressContext(w, r)
	if !ok {
		return
	}
	value, err := h.learnerProgress.Get(r.Context(), string(actor.UserID()), courseID, version)
	if err != nil {
		learnerProgressProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, learnerProgressDetail(value))
}

func (h *authHTTP) handleLearnerProgressComplete(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	courseID, version, ok := learnerProgressContext(w, r)
	if !ok {
		return
	}
	lessonKey, err := courses.NormalizeStructureKey(chi.URLParam(r, "lessonKey"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid lesson identifier")
		return
	}
	var input learnerProgressCompleteRequest
	if err := decodeAuthoringBody(w, r, 1024, &input); err != nil || input.ExpectedRevision == nil || *input.ExpectedRevision < 1 {
		problemCode(w, r, http.StatusBadRequest, "Invalid progress update", "invalid_progress_update")
		return
	}
	value, err := h.learnerProgress.CompleteLesson(r.Context(), string(actor.UserID()), courseID, version, lessonKey, *input.ExpectedRevision)
	if err != nil {
		learnerProgressProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, learnerProgressDetail(value))
}

func learnerProgressContext(w http.ResponseWriter, r *http.Request) (courses.CourseID, courses.Version, bool) {
	rawCourseID := chi.URLParam(r, "courseId")
	parsedCourseID, err := uuid.Parse(rawCourseID)
	if err != nil || parsedCourseID.String() != rawCourseID {
		problem(w, r, http.StatusBadRequest, "Invalid course ID")
		return "", courses.Version{}, false
	}
	version, err := courses.ParseVersion(chi.URLParam(r, "version"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid course version")
		return "", courses.Version{}, false
	}
	return courses.CourseID(rawCourseID), version, true
}

func learnerProgressDetail(value progress.CourseProgress) learnerProgressDTO {
	return learnerProgressDTO{Revision: value.Revision, CompletedLessonKeys: nonNilStrings(value.CompletedLessonKeys), UpdatedAt: value.UpdatedAt}
}

func learnerProgressProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, progress.ErrProgressNotFound):
		problem(w, r, http.StatusNotFound, "Not found")
	case errors.Is(err, progress.ErrInvalidProgress):
		problemCode(w, r, http.StatusBadRequest, "Invalid progress update", "invalid_progress_update")
	case errors.Is(err, progress.ErrRevisionMismatch):
		problemCode(w, r, http.StatusConflict, "Progress revision conflict", "progress_revision_conflict")
	default:
		problem(w, r, http.StatusInternalServerError, "Course progress unavailable")
	}
}
