package platform

import (
	"errors"
	"net/http"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxTranslationBodyBytes int64 = 256 * 1024

type translationCreateRequest struct {
	TargetLanguage string `json:"targetLanguage"`
}
type translationPatchRequest struct {
	ExpectedRevision *int64                     `json:"expectedRevision"`
	Changes          []translationChangeRequest `json:"changes"`
}
type translationChangeRequest struct {
	Target        string                `json:"target"`
	Field         string                `json:"field"`
	ModuleKey     string                `json:"moduleKey"`
	LessonKey     string                `json:"lessonKey"`
	BlockKey      string                `json:"blockKey"`
	AssessmentKey string                `json:"assessmentKey"`
	QuestionKey   string                `json:"questionKey"`
	ItemKey       string                `json:"itemKey"`
	Objective     *int                  `json:"objective"`
	Translated    optionalField[string] `json:"translated"`
}
type translationPublishRequest struct {
	ExpectedRevision *int64 `json:"expectedRevision"`
}
type translationSummaryDTO struct {
	TranslationID  string                         `json:"translationId"`
	TargetLanguage string                         `json:"targetLanguage"`
	Lifecycle      translations.TranslationStatus `json:"lifecycle"`
	Revision       int64                          `json:"revision"`
	Completeness   translations.Completeness      `json:"completeness"`
}
type translationPublicationDTO struct {
	PublicationID         string    `json:"publicationId"`
	TranslationID         string    `json:"translationId"`
	Revision              int64     `json:"revision"`
	TargetLanguage        string    `json:"targetLanguage"`
	SourceCourseID        string    `json:"sourceCourseId"`
	SourceCourseVersionID string    `json:"sourceCourseVersionId"`
	SourceVersion         string    `json:"sourceVersion"`
	PublishedAt           time.Time `json:"publishedAt"`
}

func (a *authHTTP) handleTranslationCreate(w http.ResponseWriter, r *http.Request) {
	actor, courseID, version, ok := translationRouteContext(w, r)
	if !ok {
		return
	}
	var in translationCreateRequest
	if err := decodeAuthoringBody(w, r, maxTranslationBodyBytes, &in); err != nil {
		problemCode(w, r, http.StatusBadRequest, "Invalid translation request", "invalid_translation_request")
		return
	}
	target, err := courses.NormalizeLanguageTag(in.TargetLanguage)
	if err != nil {
		problemCode(w, r, http.StatusBadRequest, "Invalid translation request", "invalid_translation_request")
		return
	}
	translation, err := a.translations.CreateForCourseVersion(r.Context(), string(actor.UserID()), courseID, version, target)
	if err != nil {
		translationProblem(w, r, err, true)
		return
	}
	view, viewErr := a.translationWorkspace.View(r.Context(), string(actor.UserID()), translation.ID)
	if viewErr != nil {
		translationProblem(w, r, viewErr, false)
		return
	}
	w.Header().Set("Location", "/api/translations/"+string(translation.ID))
	writeJSON(w, http.StatusCreated, translationSummaryWithCompleteness(translation, view.Completeness))
}
func (a *authHTTP) handleTranslationList(w http.ResponseWriter, r *http.Request) {
	actor, courseID, version, ok := translationRouteContext(w, r)
	if !ok {
		return
	}
	values, err := a.translations.ListForCourseVersion(r.Context(), string(actor.UserID()), courseID, version)
	if err != nil {
		translationProblem(w, r, err, true)
		return
	}
	result := make([]translationSummaryDTO, 0, len(values))
	for _, value := range values {
		view, viewErr := a.translationWorkspace.View(r.Context(), string(actor.UserID()), value.ID)
		if viewErr != nil {
			translationProblem(w, r, viewErr, true)
			return
		}
		result = append(result, translationSummaryWithCompleteness(value, view.Completeness))
	}
	writeJSON(w, http.StatusOK, struct {
		Translations []translationSummaryDTO `json:"translations"`
	}{Translations: result})
}
func (a *authHTTP) handleTranslationWorkspace(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := translationActorID(w, r)
	if !ok {
		return
	}
	view, err := a.translationWorkspace.View(r.Context(), string(actor.UserID()), id)
	if err != nil {
		translationProblem(w, r, err, false)
		return
	}
	writeJSON(w, http.StatusOK, view)
}
func (a *authHTTP) handleTranslationPatch(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := translationActorID(w, r)
	if !ok {
		return
	}
	var in translationPatchRequest
	if err := decodeAuthoringBody(w, r, maxTranslationBodyBytes, &in); err != nil || in.ExpectedRevision == nil || *in.ExpectedRevision < 1 || len(in.Changes) == 0 {
		problemCode(w, r, http.StatusBadRequest, "Invalid translation update", "invalid_translation_update")
		return
	}
	changes := make([]translations.TextChange, 0, len(in.Changes))
	for _, c := range in.Changes {
		if !c.Translated.set || ((c.Target == "COURSE" || c.Target == "LESSON") && c.Field == "objective" && c.Objective == nil) {
			problemCode(w, r, http.StatusBadRequest, "Invalid translation update", "invalid_translation_update")
			return
		}
		objective := 0
		if c.Objective != nil {
			objective = *c.Objective
		}
		var value *string
		if !c.Translated.null {
			v := c.Translated.value
			value = &v
		}
		changes = append(changes, translations.TextChange{Target: c.Target, Field: c.Field, ModuleKey: c.ModuleKey, LessonKey: c.LessonKey, BlockKey: c.BlockKey, AssessmentKey: c.AssessmentKey, QuestionKey: c.QuestionKey, ItemKey: c.ItemKey, Objective: objective, Translated: value})
	}
	value, err := a.translations.Patch(r.Context(), string(actor.UserID()), id, *in.ExpectedRevision, changes)
	if err != nil {
		translationProblem(w, r, err, false)
		return
	}
	view, viewErr := a.translationWorkspace.View(r.Context(), string(actor.UserID()), value.ID)
	if viewErr != nil {
		translationProblem(w, r, viewErr, false)
		return
	}
	writeJSON(w, http.StatusOK, translationSummaryWithCompleteness(value, view.Completeness))
}
func (a *authHTTP) handleTranslationPublish(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := translationActorID(w, r)
	if !ok {
		return
	}
	var in translationPublishRequest
	if err := decodeAuthoringBody(w, r, maxTranslationBodyBytes, &in); err != nil || in.ExpectedRevision == nil || *in.ExpectedRevision < 1 {
		problemCode(w, r, http.StatusBadRequest, "Invalid translation publication", "invalid_translation_publication")
		return
	}
	value, err := a.translations.PublishExpected(r.Context(), string(actor.UserID()), id, *in.ExpectedRevision)
	if err != nil {
		translationProblem(w, r, err, false)
		return
	}
	writeJSON(w, http.StatusCreated, translationPublication(value))
}
func translationRouteContext(w http.ResponseWriter, r *http.Request) (identity.AuthenticatedActor, courses.CourseID, courses.Version, bool) {
	actor, ok := currentAuthenticatedActor(r.Context())
	if !ok {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return identity.AuthenticatedActor{}, "", courses.Version{}, false
	}
	raw := chi.URLParam(r, "courseId")
	id, err := uuid.Parse(raw)
	if err != nil || id.String() != raw {
		problem(w, r, http.StatusBadRequest, "Invalid course ID")
		return identity.AuthenticatedActor{}, "", courses.Version{}, false
	}
	version, err := courses.ParseVersion(chi.URLParam(r, "version"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid course version")
		return identity.AuthenticatedActor{}, "", courses.Version{}, false
	}
	return actor, courses.CourseID(raw), version, true
}
func translationActorID(w http.ResponseWriter, r *http.Request) (identity.AuthenticatedActor, translations.TranslationID, bool) {
	actor, ok := currentAuthenticatedActor(r.Context())
	if !ok {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return identity.AuthenticatedActor{}, "", false
	}
	raw := chi.URLParam(r, "translationId")
	id, err := uuid.Parse(raw)
	if err != nil || id.String() != raw {
		problem(w, r, http.StatusBadRequest, "Invalid translation identifier")
		return identity.AuthenticatedActor{}, "", false
	}
	return actor, translations.TranslationID(raw), true
}
func translationSummaryWithCompleteness(value translations.CourseTranslation, completeness translations.Completeness) translationSummaryDTO {
	return translationSummaryDTO{TranslationID: string(value.ID), TargetLanguage: string(value.TargetLanguage), Lifecycle: value.Status, Revision: value.Revision, Completeness: completeness}
}
func translationPublication(value translations.TranslationPublication) translationPublicationDTO {
	return translationPublicationDTO{PublicationID: string(value.ID), TranslationID: string(value.TranslationID), Revision: value.Revision, TargetLanguage: string(value.TargetLanguage), SourceCourseID: string(value.Source.CourseID), SourceCourseVersionID: string(value.Source.CourseVersionID), SourceVersion: value.Source.Version.String(), PublishedAt: value.PublishedAt}
}
func translationProblem(w http.ResponseWriter, r *http.Request, err error, sourceRoute bool) {
	switch {
	case errors.Is(err, translations.ErrTranslationNotFound), errors.Is(err, translations.ErrAuthorizationDenied), errors.Is(err, courses.ErrNotFound):
		problem(w, r, http.StatusNotFound, "Not found")
	case errors.Is(err, translations.ErrRevisionMismatch):
		problemCode(w, r, http.StatusConflict, "Translation revision conflict", "translation_revision_conflict")
	case errors.Is(err, translations.ErrTranslationConflict):
		problemCode(w, r, http.StatusConflict, "Translation already exists", "translation_already_exists")
	case errors.Is(err, translations.ErrTranslationIncomplete):
		problemCode(w, r, http.StatusConflict, "Translation is incomplete", "translation_incomplete")
	case errors.Is(err, translations.ErrInvalidTranslation):
		problemCode(w, r, http.StatusBadRequest, "Invalid translation request", "invalid_translation_request")
	case errors.Is(err, translations.ErrSourceIntegrity):
		problemCode(w, r, http.StatusConflict, "Translation workspace unavailable", "translation_source_integrity")
	default:
		_ = sourceRoute
		problem(w, r, http.StatusInternalServerError, "Translation service unavailable")
	}
}
