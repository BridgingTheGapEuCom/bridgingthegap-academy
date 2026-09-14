package platform

import (
	"errors"
	"net/http"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const authoringIDLength = 36

type authoringDraftDTO struct {
	ID              string                `json:"id"`
	CourseID        string                `json:"course_id"`
	IntendedVersion string                `json:"intended_version"`
	SourceLanguage  string                `json:"source_language"`
	Title           string                `json:"title"`
	Description     string                `json:"description"`
	Objectives      []string              `json:"objectives"`
	Changelog       string                `json:"changelog"`
	License         authoringLicenseDTO   `json:"license"`
	Status          authoring.DraftStatus `json:"status"`
	Revision        int64                 `json:"revision"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}

type authoringLicenseDTO struct {
	Kind        courses.ContentLicenseKind `json:"kind"`
	Identifier  string                     `json:"identifier"`
	DisplayName string                     `json:"display_name"`
	URL         string                     `json:"url"`
	CustomText  string                     `json:"custom_text"`
}

type authoringWorkspaceDTO struct {
	ID             string    `json:"id"`
	DraftID        string    `json:"draft_id"`
	CreatedAt      time.Time `json:"created_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
}

type authoringModuleDTO struct {
	ID          string                      `json:"id"`
	StableKey   string                      `json:"stable_key"`
	Title       string                      `json:"title"`
	Description string                      `json:"description"`
	Position    int                         `json:"position"`
	Revision    int64                       `json:"revision"`
	Lessons     []authoringLessonSummaryDTO `json:"lessons"`
}

type authoringLessonSummaryDTO struct {
	ID                          string   `json:"id"`
	StableKey                   string   `json:"stable_key"`
	Title                       string   `json:"title"`
	Description                 string   `json:"description"`
	Objectives                  []string `json:"objectives"`
	EstimatedDurationMinutes    *int     `json:"estimated_duration_minutes"`
	Position                    int      `json:"position"`
	Revision                    int64    `json:"revision"`
	RecommendedPrerequisiteKeys []string `json:"recommended_prerequisite_keys"`
}

type authoringLessonDTO struct {
	ID                          string                `json:"id"`
	DraftID                     string                `json:"draft_id"`
	ModuleID                    string                `json:"module_id"`
	StableKey                   string                `json:"stable_key"`
	Title                       string                `json:"title"`
	Description                 string                `json:"description"`
	Objectives                  []string              `json:"objectives"`
	EstimatedDurationMinutes    *int                  `json:"estimated_duration_minutes"`
	Position                    int                   `json:"position"`
	Revision                    int64                 `json:"revision"`
	RecommendedPrerequisiteKeys []string              `json:"recommended_prerequisite_keys"`
	Content                     courses.LessonContent `json:"content"`
	CreatedAt                   time.Time             `json:"created_at"`
	UpdatedAt                   time.Time             `json:"updated_at"`
}

func (a *authHTTP) handleAuthoringDraft(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	draft, err := a.authoring.Draft(r.Context(), actor, draftID)
	if err != nil {
		authoringProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringDraft(draft))
}

func (a *authHTTP) handleAuthoringWorkspace(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	workspace, err := a.authoring.Workspace(r.Context(), actor, draftID)
	if err != nil {
		authoringProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringWorkspaceDTO{ID: string(workspace.ID), DraftID: string(workspace.DraftID), CreatedAt: workspace.CreatedAt, LastActivityAt: workspace.LastActivityAt})
}

func (a *authHTTP) handleAuthoringStructure(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	structure, err := a.authoring.Structure(r.Context(), actor, draftID)
	if err != nil {
		authoringProblem(w, r, err)
		return
	}
	modules := make([]authoringModuleDTO, 0, len(structure))
	for _, item := range structure {
		lessons := make([]authoringLessonSummaryDTO, 0, len(item.Lessons))
		for _, lesson := range item.Lessons {
			lessons = append(lessons, authoringLessonSummary(lesson.Lesson, lesson.RecommendedPrerequisiteKeys))
		}
		modules = append(modules, authoringModuleDTO{ID: string(item.Module.ID), StableKey: item.Module.StableKey, Title: item.Module.Title, Description: item.Module.Description, Position: item.Module.Position, Revision: item.Module.Revision, Lessons: lessons})
	}
	writeJSON(w, http.StatusOK, struct {
		Modules []authoringModuleDTO `json:"modules"`
	}{Modules: modules})
}

func (a *authHTTP) handleAuthoringLesson(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	lessonID, ok := authoringLessonID(w, r)
	if !ok {
		return
	}
	lesson, prerequisites, err := a.authoring.Lesson(r.Context(), actor, draftID, lessonID)
	if err != nil {
		authoringProblem(w, r, err)
		return
	}
	keys := make([]string, 0, len(prerequisites))
	for _, prerequisite := range prerequisites {
		keys = append(keys, prerequisite.TargetStableKey)
	}
	writeJSON(w, http.StatusOK, authoringLessonDetail(lesson, keys))
}

func authoringRequest(w http.ResponseWriter, r *http.Request) (authoring.DraftID, identity.AuthenticatedActor, bool) {
	draftID, ok := authoringDraftID(w, r)
	if !ok {
		return "", identity.AuthenticatedActor{}, false
	}
	actor, ok := currentAuthenticatedActor(r.Context())
	if !ok {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return "", identity.AuthenticatedActor{}, false
	}
	return draftID, actor, true
}

func authoringDraftID(w http.ResponseWriter, r *http.Request) (authoring.DraftID, bool) {
	return authoringUUIDParam(w, r, "draftId", "Invalid draft identifier")
}

func authoringLessonID(w http.ResponseWriter, r *http.Request) (authoring.LessonID, bool) {
	id, ok := authoringUUIDParam(w, r, "lessonId", "Invalid lesson identifier")
	return authoring.LessonID(id), ok
}

func authoringUUIDParam(w http.ResponseWriter, r *http.Request, name, title string) (authoring.DraftID, bool) {
	value := chi.URLParam(r, name)
	if len(value) != authoringIDLength {
		problem(w, r, http.StatusBadRequest, title)
		return "", false
	}
	id, err := uuid.Parse(value)
	if err != nil || id.String() != value {
		problem(w, r, http.StatusBadRequest, title)
		return "", false
	}
	return authoring.DraftID(value), true
}

func authoringProblem(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, authoring.ErrNotFound) || errors.Is(err, authoring.ErrAuthorizationDenied) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
}

func authoringDraft(draft authoring.CourseDraft) authoringDraftDTO {
	return authoringDraftDTO{ID: string(draft.ID), CourseID: string(draft.Metadata.CourseID), IntendedVersion: draft.Metadata.IntendedVersion.String(), SourceLanguage: string(draft.Metadata.SourceLanguage), Title: draft.Metadata.Title, Description: draft.Metadata.Description, Objectives: draft.Metadata.LearningObjectives, Changelog: draft.Metadata.Changelog, License: authoringLicenseDTO{Kind: draft.Metadata.License.Kind, Identifier: draft.Metadata.License.Identifier, DisplayName: draft.Metadata.License.DisplayName, URL: draft.Metadata.License.URL, CustomText: draft.Metadata.License.CustomText}, Status: draft.Status, Revision: draft.Revision, CreatedAt: draft.CreatedAt, UpdatedAt: draft.UpdatedAt}
}

func authoringLessonSummary(lesson authoring.DraftLesson, prerequisites []string) authoringLessonSummaryDTO {
	return authoringLessonSummaryDTO{ID: string(lesson.ID), StableKey: lesson.StableKey, Title: lesson.Title, Description: lesson.Description, Objectives: lesson.LearningObjectives, EstimatedDurationMinutes: lesson.EstimatedDurationMinutes, Position: lesson.Position, Revision: lesson.Revision, RecommendedPrerequisiteKeys: prerequisites}
}

func authoringLessonDetail(lesson authoring.DraftLesson, prerequisites []string) authoringLessonDTO {
	return authoringLessonDTO{ID: string(lesson.ID), DraftID: string(lesson.DraftID), ModuleID: string(lesson.ModuleID), StableKey: lesson.StableKey, Title: lesson.Title, Description: lesson.Description, Objectives: lesson.LearningObjectives, EstimatedDurationMinutes: lesson.EstimatedDurationMinutes, Position: lesson.Position, Revision: lesson.Revision, RecommendedPrerequisiteKeys: prerequisites, Content: lesson.Content, CreatedAt: lesson.CreatedAt, UpdatedAt: lesson.UpdatedAt}
}
