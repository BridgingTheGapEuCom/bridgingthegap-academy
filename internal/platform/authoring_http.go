package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
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

type authoringDraftUpdateRequest struct {
	ExpectedRevision *int64                                 `json:"expectedRevision"`
	IntendedVersion  optionalField[string]                  `json:"intendedVersion"`
	SourceLanguage   optionalField[string]                  `json:"sourceLanguage"`
	Title            optionalField[string]                  `json:"title"`
	Description      optionalField[string]                  `json:"description"`
	Objectives       optionalField[[]string]                `json:"objectives"`
	Changelog        optionalField[string]                  `json:"changelog"`
	License          optionalField[authoringLicenseRequest] `json:"license"`
}

type authoringModuleCreateRequest struct {
	ExpectedDraftRevision *int64 `json:"expectedDraftRevision"`
	StableKey             string `json:"stableKey"`
	Title                 string `json:"title"`
	Description           string `json:"description"`
	Position              *int   `json:"position"`
}

type authoringModuleUpdateRequest struct {
	ExpectedModuleRevision *int64                `json:"expectedModuleRevision"`
	Title                  optionalField[string] `json:"title"`
	Description            optionalField[string] `json:"description"`
}

type authoringModuleReorderRequest struct {
	ExpectedDraftRevision *int64   `json:"expectedDraftRevision"`
	ModuleIDs             []string `json:"moduleIds"`
}

type authoringModuleDeleteRequest struct {
	ExpectedDraftRevision  *int64 `json:"expectedDraftRevision"`
	ExpectedModuleRevision *int64 `json:"expectedModuleRevision"`
}

type authoringModuleMutationDTO struct {
	Module        authoringModuleDTO `json:"module"`
	DraftRevision int64              `json:"draftRevision"`
}

// optionalField distinguishes an omitted JSON member from an explicit null.
// Null is rejected for all mutable metadata fields because their domain values
// are required; an empty string or slice remains an explicit domain input.
type optionalField[T any] struct {
	set   bool
	null  bool
	value T
}

func (f *optionalField[T]) UnmarshalJSON(data []byte) error {
	f.set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		f.null = true
		return nil
	}
	return json.Unmarshal(data, &f.value)
}

type authoringLicenseRequest struct {
	Kind        courses.ContentLicenseKind `json:"kind"`
	Identifier  string                     `json:"identifier"`
	DisplayName string                     `json:"display_name"`
	URL         string                     `json:"url"`
	CustomText  string                     `json:"custom_text"`
}

func (l *authoringLicenseRequest) UnmarshalJSON(data []byte) error {
	type fields authoringLicenseRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value fields
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing license JSON")
	}
	*l = authoringLicenseRequest(value)
	return nil
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

func (a *authHTTP) handleAuthoringDraftUpdate(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	expectedRevision, patch, err := decodeAuthoringDraftUpdate(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid draft update")
		return
	}
	if a.authoringMutations == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	draft, err := a.authoringMutations.UpdateDraft(r.Context(), actor, draftID, expectedRevision, patch)
	if err != nil {
		authoringMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringDraft(draft))
}

func (a *authHTTP) handleAuthoringModuleCreate(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	expected, input, err := decodeAuthoringModuleCreate(w, r, draftID)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid module create")
		return
	}
	if a.authoringStructureMutations == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	result, err := a.authoringStructureMutations.CreateModule(r.Context(), actor, draftID, expected, input)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringModuleMutation(result))
}

func (a *authHTTP) handleAuthoringModuleUpdate(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	moduleID, ok := authoringModuleID(w, r)
	if !ok {
		return
	}
	expected, patch, err := decodeAuthoringModuleUpdate(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid module update")
		return
	}
	if a.authoringStructureMutations == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	result, err := a.authoringStructureMutations.UpdateModule(r.Context(), actor, draftID, moduleID, expected, patch)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringModuleMutation(result))
}

func (a *authHTTP) handleAuthoringModuleReorder(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	expected, order, err := decodeAuthoringModuleReorder(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid module order")
		return
	}
	if a.authoringStructureMutations == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	draft, err := a.authoringStructureMutations.ReorderModules(r.Context(), actor, draftID, expected, order)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		DraftRevision int64 `json:"draftRevision"`
	}{DraftRevision: draft.Revision})
}

func (a *authHTTP) handleAuthoringModuleDelete(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	moduleID, ok := authoringModuleID(w, r)
	if !ok {
		return
	}
	expectedDraft, expectedModule, err := decodeAuthoringModuleDelete(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid module delete")
		return
	}
	if a.authoringStructureMutations == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	draft, err := a.authoringStructureMutations.DeleteModule(r.Context(), actor, draftID, moduleID, expectedDraft, expectedModule)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		DraftRevision int64 `json:"draftRevision"`
	}{DraftRevision: draft.Revision})
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

func authoringModuleID(w http.ResponseWriter, r *http.Request) (authoring.ModuleID, bool) {
	id, ok := authoringUUIDParam(w, r, "moduleId", "Invalid module identifier")
	return authoring.ModuleID(id), ok
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

func authoringMutationProblem(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, authoring.ErrNotFound) || errors.Is(err, authoring.ErrAuthorizationDenied) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	if errors.Is(err, authoring.ErrRevisionMismatch) || errors.Is(err, authoring.ErrInvalidState) || errors.Is(err, authoring.ErrConflict) {
		problem(w, r, http.StatusConflict, "Draft revision conflict")
		return
	}
	if errors.Is(err, authoring.ErrInvalidPatch) {
		problem(w, r, http.StatusBadRequest, "Invalid draft update")
		return
	}
	problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
}

func authoringStructureMutationProblem(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, authoring.ErrNotFound) || errors.Is(err, authoring.ErrAuthorizationDenied) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	if errors.Is(err, authoring.ErrRevisionMismatch) || errors.Is(err, authoring.ErrInvalidState) || errors.Is(err, authoring.ErrConflict) {
		problem(w, r, http.StatusConflict, "Draft revision conflict")
		return
	}
	if errors.Is(err, authoring.ErrInvalidStructure) {
		problem(w, r, http.StatusBadRequest, "Invalid module structure")
		return
	}
	problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
}

func decodeAuthoringDraftUpdate(w http.ResponseWriter, r *http.Request) (int64, authoring.DraftMetadataPatch, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return 0, authoring.DraftMetadataPatch{}, errors.New("invalid content type")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAuthoringDraftMetadataBodyBytes))
	decoder.DisallowUnknownFields()
	var input authoringDraftUpdateRequest
	if err := decoder.Decode(&input); err != nil {
		return 0, authoring.DraftMetadataPatch{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return 0, authoring.DraftMetadataPatch{}, errors.New("trailing JSON")
	}
	if input.ExpectedRevision == nil || *input.ExpectedRevision < 1 {
		return 0, authoring.DraftMetadataPatch{}, errors.New("missing expected revision")
	}
	patch := authoring.DraftMetadataPatch{}
	if input.IntendedVersion.set {
		if input.IntendedVersion.null {
			return 0, authoring.DraftMetadataPatch{}, errors.New("null intended version")
		}
		version, err := courses.ParseVersion(input.IntendedVersion.value)
		if err != nil {
			return 0, authoring.DraftMetadataPatch{}, err
		}
		patch.IntendedVersion = &version
	}
	if input.SourceLanguage.set {
		if input.SourceLanguage.null {
			return 0, authoring.DraftMetadataPatch{}, errors.New("null source language")
		}
		language := courses.LanguageTag(input.SourceLanguage.value)
		patch.SourceLanguage = &language
	}
	if input.Title.set {
		if input.Title.null {
			return 0, authoring.DraftMetadataPatch{}, errors.New("null title")
		}
		patch.Title = &input.Title.value
	}
	if input.Description.set {
		if input.Description.null {
			return 0, authoring.DraftMetadataPatch{}, errors.New("null description")
		}
		patch.Description = &input.Description.value
	}
	if input.Objectives.set {
		if input.Objectives.null {
			return 0, authoring.DraftMetadataPatch{}, errors.New("null objectives")
		}
		patch.LearningObjectives = &input.Objectives.value
	}
	if input.Changelog.set {
		if input.Changelog.null {
			return 0, authoring.DraftMetadataPatch{}, errors.New("null changelog")
		}
		patch.Changelog = &input.Changelog.value
	}
	if input.License.set {
		if input.License.null {
			return 0, authoring.DraftMetadataPatch{}, errors.New("null license")
		}
		license := courses.ContentLicense{Kind: input.License.value.Kind, Identifier: input.License.value.Identifier, DisplayName: input.License.value.DisplayName, URL: input.License.value.URL, CustomText: input.License.value.CustomText}
		patch.License = &license
	}
	if patch.Empty() {
		return 0, authoring.DraftMetadataPatch{}, errors.New("empty patch")
	}
	return *input.ExpectedRevision, patch, nil
}

func decodeAuthoringModuleCreate(w http.ResponseWriter, r *http.Request, draftID authoring.DraftID) (int64, authoring.ModuleInput, error) {
	var input authoringModuleCreateRequest
	if err := decodeAuthoringJSON(w, r, &input); err != nil {
		return 0, authoring.ModuleInput{}, err
	}
	if input.ExpectedDraftRevision == nil || *input.ExpectedDraftRevision < 1 || input.Position == nil {
		return 0, authoring.ModuleInput{}, errors.New("missing module create fields")
	}
	module := authoring.ModuleInput{DraftID: draftID, StableKey: input.StableKey, Title: input.Title, Description: input.Description, Position: *input.Position}
	if err := module.Validate(); err != nil {
		return 0, authoring.ModuleInput{}, err
	}
	return *input.ExpectedDraftRevision, module, nil
}

func decodeAuthoringModuleUpdate(w http.ResponseWriter, r *http.Request) (int64, authoring.DraftModulePatch, error) {
	var input authoringModuleUpdateRequest
	if err := decodeAuthoringJSON(w, r, &input); err != nil {
		return 0, authoring.DraftModulePatch{}, err
	}
	if input.ExpectedModuleRevision == nil || *input.ExpectedModuleRevision < 1 {
		return 0, authoring.DraftModulePatch{}, errors.New("missing expected module revision")
	}
	patch := authoring.DraftModulePatch{}
	if input.Title.set {
		if input.Title.null {
			return 0, authoring.DraftModulePatch{}, errors.New("null title")
		}
		patch.Title = &input.Title.value
	}
	if input.Description.set {
		if input.Description.null {
			return 0, authoring.DraftModulePatch{}, errors.New("null description")
		}
		patch.Description = &input.Description.value
	}
	if patch.Empty() {
		return 0, authoring.DraftModulePatch{}, errors.New("empty module patch")
	}
	return *input.ExpectedModuleRevision, patch, nil
}

func decodeAuthoringModuleReorder(w http.ResponseWriter, r *http.Request) (int64, []authoring.ModuleID, error) {
	var input authoringModuleReorderRequest
	if err := decodeAuthoringJSON(w, r, &input); err != nil {
		return 0, nil, err
	}
	if input.ExpectedDraftRevision == nil || *input.ExpectedDraftRevision < 1 {
		return 0, nil, errors.New("missing expected draft revision")
	}
	order := make([]authoring.ModuleID, 0, len(input.ModuleIDs))
	for _, raw := range input.ModuleIDs {
		if !validAuthoringUUID(raw) {
			return 0, nil, errors.New("invalid module identifier")
		}
		order = append(order, authoring.ModuleID(raw))
	}
	if err := authoring.ValidateModuleOrder(order); err != nil {
		return 0, nil, err
	}
	return *input.ExpectedDraftRevision, order, nil
}

func decodeAuthoringModuleDelete(w http.ResponseWriter, r *http.Request) (int64, int64, error) {
	var input authoringModuleDeleteRequest
	if err := decodeAuthoringJSON(w, r, &input); err != nil {
		return 0, 0, err
	}
	if input.ExpectedDraftRevision == nil || *input.ExpectedDraftRevision < 1 || input.ExpectedModuleRevision == nil || *input.ExpectedModuleRevision < 1 {
		return 0, 0, errors.New("missing expected revision")
	}
	return *input.ExpectedDraftRevision, *input.ExpectedModuleRevision, nil
}

func decodeAuthoringJSON(w http.ResponseWriter, r *http.Request, value any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errors.New("invalid content type")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAuthoringModuleBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func validAuthoringUUID(value string) bool {
	return len(value) == authoringIDLength && func() bool {
		id, err := uuid.Parse(value)
		return err == nil && id.String() == value
	}()
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

func authoringModuleMutation(result authoring.ModuleMutationResult) authoringModuleMutationDTO {
	module := result.Module
	return authoringModuleMutationDTO{
		Module:        authoringModuleDTO{ID: string(module.ID), StableKey: module.StableKey, Title: module.Title, Description: module.Description, Position: module.Position, Revision: module.Revision, Lessons: []authoringLessonSummaryDTO{}},
		DraftRevision: result.Draft.Revision,
	}
}
