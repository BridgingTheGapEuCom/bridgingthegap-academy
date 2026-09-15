package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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

// authoringDraftSummaryDTO is the narrow private discovery contract. Full
// draft metadata remains available only through a draft-scoped read.
type authoringDraftSummaryDTO struct {
	ID              string                `json:"id"`
	Title           string                `json:"title"`
	IntendedVersion string                `json:"intendedVersion"`
	Status          authoring.DraftStatus `json:"status"`
	UpdatedAt       time.Time             `json:"updatedAt"`
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

// authoringActiveMemberDTO is intentionally smaller than the persistence
// membership record: it exposes only the opaque identity reference and role.
type authoringActiveMemberDTO struct {
	UserID string               `json:"userId"`
	Role   authoring.MemberRole `json:"role"`
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
	ExpectedDraftRevision *int64    `json:"expectedDraftRevision"`
	ModuleIDs             *[]string `json:"moduleIds"`
}

type authoringModuleDeleteRequest struct {
	ExpectedDraftRevision  *int64 `json:"expectedDraftRevision"`
	ExpectedModuleRevision *int64 `json:"expectedModuleRevision"`
}

type authoringModuleMutationDTO struct {
	Module        authoringModuleDTO `json:"module"`
	DraftRevision int64              `json:"draftRevision"`
}

type authoringLessonCreateRequest struct {
	ExpectedDraftRevision    *int64   `json:"expectedDraftRevision"`
	StableKey                string   `json:"stableKey"`
	Title                    string   `json:"title"`
	Description              string   `json:"description"`
	Objectives               []string `json:"objectives"`
	EstimatedDurationMinutes *int     `json:"estimatedDurationMinutes"`
	Position                 *int     `json:"position"`
}

type authoringLessonUpdateRequest struct {
	ExpectedLessonRevision   *int64                  `json:"expectedLessonRevision"`
	Title                    optionalField[string]   `json:"title"`
	Description              optionalField[string]   `json:"description"`
	Objectives               optionalField[[]string] `json:"objectives"`
	EstimatedDurationMinutes optionalField[int]      `json:"estimatedDurationMinutes"`
}

type authoringLessonOrderModuleRequest struct {
	ModuleID  string    `json:"moduleId"`
	LessonIDs *[]string `json:"lessonIds"`
}

type authoringLessonReorderRequest struct {
	ExpectedDraftRevision *int64                               `json:"expectedDraftRevision"`
	Modules               *[]authoringLessonOrderModuleRequest `json:"modules"`
}

type authoringLessonPrerequisitesRequest struct {
	ExpectedLessonRevision *int64    `json:"expectedLessonRevision"`
	PrerequisiteLessonKeys *[]string `json:"prerequisiteLessonKeys"`
}

type authoringLessonDeleteRequest struct {
	ExpectedDraftRevision  *int64 `json:"expectedDraftRevision"`
	ExpectedLessonRevision *int64 `json:"expectedLessonRevision"`
}

type authoringLessonContentRequest struct {
	ExpectedLessonRevision *int64                `json:"expectedLessonRevision"`
	Content                courses.LessonContent `json:"content"`
}

type authoringMemberAddRequest struct {
	ExpectedDraftRevision *int64               `json:"expectedDraftRevision"`
	UserID                string               `json:"userId"`
	Role                  authoring.MemberRole `json:"role"`
}

type authoringMemberRoleRequest struct {
	ExpectedDraftRevision *int64               `json:"expectedDraftRevision"`
	Role                  authoring.MemberRole `json:"role"`
}

type authoringMemberRevokeRequest struct {
	ExpectedDraftRevision *int64 `json:"expectedDraftRevision"`
}

type authoringLessonMutationDTO struct {
	ID                       string   `json:"id"`
	DraftID                  string   `json:"draft_id"`
	ModuleID                 string   `json:"module_id"`
	StableKey                string   `json:"stable_key"`
	Title                    string   `json:"title"`
	Description              string   `json:"description"`
	Objectives               []string `json:"objectives"`
	EstimatedDurationMinutes *int     `json:"estimated_duration_minutes"`
	Position                 int      `json:"position"`
	Revision                 int64    `json:"revision"`
	DraftRevision            int64    `json:"draftRevision"`
}

type authoringLessonContentMutationDTO struct {
	Lesson  authoringLessonMutationDTO `json:"lesson"`
	Content courses.LessonContent      `json:"content"`
}

type authoringMemberMutationDTO struct {
	ID            string               `json:"id"`
	UserID        string               `json:"userId"`
	Role          authoring.MemberRole `json:"role"`
	CreatedAt     time.Time            `json:"createdAt"`
	RevokedAt     *time.Time           `json:"revokedAt"`
	DraftRevision int64                `json:"draftRevision"`
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
	if err := validateAuthoringLicenseJSON(data); err != nil {
		return err
	}
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

func (a *authHTTP) handleAuthoringDraftList(w http.ResponseWriter, r *http.Request) {
	actor, ok := currentAuthenticatedActor(r.Context())
	if !ok {
		problem(w, r, http.StatusUnauthorized, "Unauthenticated")
		return
	}
	if a.authoring == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	drafts, err := a.authoring.Drafts(r.Context(), actor)
	if err != nil {
		authoringProblem(w, r, err)
		return
	}
	items := make([]authoringDraftSummaryDTO, 0, len(drafts))
	for _, draft := range drafts {
		items = append(items, authoringDraftSummaryDTO{
			ID: string(draft.ID), Title: draft.Title, IntendedVersion: draft.IntendedVersion.String(),
			Status: draft.Status, UpdatedAt: draft.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, struct {
		Drafts []authoringDraftSummaryDTO `json:"drafts"`
	}{Drafts: items})
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

func (a *authHTTP) handleAuthoringLessonCreate(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	moduleID, ok := authoringModuleID(w, r)
	if !ok {
		return
	}
	expected, input, err := decodeAuthoringLessonCreate(w, r, draftID, moduleID)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid lesson create")
		return
	}
	if a.authoringLessonMutations == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	result, err := a.authoringLessonMutations.CreateLesson(r.Context(), actor, draftID, moduleID, expected, input)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringLessonMutation(result))
}

func (a *authHTTP) handleAuthoringLessonUpdate(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	lessonID, ok := authoringLessonID(w, r)
	if !ok {
		return
	}
	expected, patch, err := decodeAuthoringLessonUpdate(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid lesson update")
		return
	}
	if a.authoringLessonMutations == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	result, err := a.authoringLessonMutations.UpdateLesson(r.Context(), actor, draftID, lessonID, expected, patch)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringLessonMutation(result))
}

func (a *authHTTP) handleAuthoringLessonReorder(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	expected, order, err := decodeAuthoringLessonReorder(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid lesson order")
		return
	}
	if a.authoringLessonMutations == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	draft, err := a.authoringLessonMutations.ReorderLessons(r.Context(), actor, draftID, expected, order)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		DraftRevision int64 `json:"draftRevision"`
	}{DraftRevision: draft.Revision})
}

func (a *authHTTP) handleAuthoringLessonPrerequisites(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	lessonID, ok := authoringLessonID(w, r)
	if !ok {
		return
	}
	expected, keys, err := decodeAuthoringLessonPrerequisites(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid lesson prerequisites")
		return
	}
	if a.authoringLessonMutations == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	result, err := a.authoringLessonMutations.ReplacePrerequisites(r.Context(), actor, draftID, lessonID, expected, keys)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringLessonMutation(result))
}

func (a *authHTTP) handleAuthoringLessonContent(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	lessonID, ok := authoringLessonID(w, r)
	if !ok {
		return
	}
	expected, content, err := decodeAuthoringLessonContent(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid lesson content")
		return
	}
	if a.authoringLessonContent == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	result, err := a.authoringLessonContent.ReplaceContent(r.Context(), actor, draftID, lessonID, expected, content)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringLessonContentMutationDTO{Lesson: authoringLessonMutation(result), Content: result.Lesson.Content})
}

func (a *authHTTP) handleAuthoringMemberAdd(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	expected, userID, role, err := decodeAuthoringMemberAdd(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid membership add")
		return
	}
	if a.authoringMemberships == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	result, err := a.authoringMemberships.AddMember(r.Context(), actor, draftID, expected, userID, role)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringMemberMutation(result))
}

func (a *authHTTP) handleAuthoringMemberRole(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	userID, ok := authoringUserID(w, r)
	if !ok {
		return
	}
	expected, role, err := decodeAuthoringMemberRole(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid membership role")
		return
	}
	if a.authoringMemberships == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	result, err := a.authoringMemberships.ChangeRole(r.Context(), actor, draftID, expected, userID, role)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringMemberMutation(result))
}

func (a *authHTTP) handleAuthoringMemberRevoke(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	userID, ok := authoringUserID(w, r)
	if !ok {
		return
	}
	expected, err := decodeAuthoringMemberRevoke(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid membership revoke")
		return
	}
	if a.authoringMemberships == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	result, err := a.authoringMemberships.RevokeMember(r.Context(), actor, draftID, expected, userID)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringMemberMutation(result))
}

func (a *authHTTP) handleAuthoringLessonDelete(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	lessonID, ok := authoringLessonID(w, r)
	if !ok {
		return
	}
	expectedDraft, expectedLesson, err := decodeAuthoringLessonDelete(w, r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid lesson delete")
		return
	}
	if a.authoringLessonMutations == nil {
		problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
		return
	}
	draft, err := a.authoringLessonMutations.DeleteLesson(r.Context(), actor, draftID, lessonID, expectedDraft, expectedLesson)
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

func (a *authHTTP) handleAuthoringMembers(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	members, err := a.authoring.Members(r.Context(), actor, draftID)
	if err != nil {
		authoringProblem(w, r, err)
		return
	}
	response := make([]authoringActiveMemberDTO, 0, len(members))
	for _, member := range members {
		response = append(response, authoringActiveMemberDTO{UserID: member.UserID, Role: member.Role})
	}
	writeJSON(w, http.StatusOK, struct {
		Members []authoringActiveMemberDTO `json:"members"`
	}{Members: response})
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

func authoringUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := authoringUUIDParam(w, r, "userId", "Invalid user identifier")
	return string(id), ok
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
		problem(w, r, http.StatusBadRequest, "Invalid authoring structure")
		return
	}
	problem(w, r, http.StatusInternalServerError, "Authoring service unavailable")
}

func decodeAuthoringDraftUpdate(w http.ResponseWriter, r *http.Request) (int64, authoring.DraftMetadataPatch, error) {
	var input authoringDraftUpdateRequest
	if err := decodeAuthoringBody(w, r, maxAuthoringDraftMetadataBodyBytes, &input); err != nil {
		return 0, authoring.DraftMetadataPatch{}, err
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
	if input.ExpectedDraftRevision == nil || *input.ExpectedDraftRevision < 1 || input.ModuleIDs == nil {
		return 0, nil, errors.New("missing expected draft revision")
	}
	order := make([]authoring.ModuleID, 0, len(*input.ModuleIDs))
	for _, raw := range *input.ModuleIDs {
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

func decodeAuthoringLessonCreate(w http.ResponseWriter, r *http.Request, draftID authoring.DraftID, moduleID authoring.ModuleID) (int64, authoring.LessonInput, error) {
	var input authoringLessonCreateRequest
	if err := decodeAuthoringLessonJSON(w, r, &input); err != nil {
		return 0, authoring.LessonInput{}, err
	}
	if input.ExpectedDraftRevision == nil || *input.ExpectedDraftRevision < 1 || input.Position == nil {
		return 0, authoring.LessonInput{}, errors.New("missing lesson create fields")
	}
	lesson := authoring.NewDraftLessonInput(draftID, moduleID, input.StableKey, input.Title, input.Description, input.Objectives, input.EstimatedDurationMinutes, *input.Position)
	if err := lesson.Validate(); err != nil {
		return 0, authoring.LessonInput{}, err
	}
	return *input.ExpectedDraftRevision, lesson, nil
}

func decodeAuthoringLessonUpdate(w http.ResponseWriter, r *http.Request) (int64, authoring.DraftLessonPatch, error) {
	var input authoringLessonUpdateRequest
	if err := decodeAuthoringLessonJSON(w, r, &input); err != nil {
		return 0, authoring.DraftLessonPatch{}, err
	}
	if input.ExpectedLessonRevision == nil || *input.ExpectedLessonRevision < 1 {
		return 0, authoring.DraftLessonPatch{}, errors.New("missing expected lesson revision")
	}
	patch := authoring.DraftLessonPatch{}
	if input.Title.set {
		if input.Title.null {
			return 0, authoring.DraftLessonPatch{}, errors.New("null title")
		}
		patch.Title = &input.Title.value
	}
	if input.Description.set {
		if input.Description.null {
			return 0, authoring.DraftLessonPatch{}, errors.New("null description")
		}
		patch.Description = &input.Description.value
	}
	if input.Objectives.set {
		if input.Objectives.null {
			return 0, authoring.DraftLessonPatch{}, errors.New("null objectives")
		}
		patch.LearningObjectives = &input.Objectives.value
	}
	if input.EstimatedDurationMinutes.set {
		patch.EstimatedDurationSet = true
		if !input.EstimatedDurationMinutes.null {
			patch.EstimatedDurationMinutes = &input.EstimatedDurationMinutes.value
		}
	}
	if patch.Empty() {
		return 0, authoring.DraftLessonPatch{}, errors.New("empty lesson patch")
	}
	return *input.ExpectedLessonRevision, patch, nil
}

func decodeAuthoringLessonReorder(w http.ResponseWriter, r *http.Request) (int64, []authoring.ModuleLessonOrder, error) {
	var input authoringLessonReorderRequest
	if err := decodeAuthoringBody(w, r, maxAuthoringLessonOrderBodyBytes, &input); err != nil {
		return 0, nil, err
	}
	if input.ExpectedDraftRevision == nil || *input.ExpectedDraftRevision < 1 || input.Modules == nil {
		return 0, nil, errors.New("missing expected draft revision")
	}
	order := make([]authoring.ModuleLessonOrder, 0, len(*input.Modules))
	for _, module := range *input.Modules {
		if !validAuthoringUUID(module.ModuleID) || module.LessonIDs == nil {
			return 0, nil, errors.New("invalid module identifier")
		}
		item := authoring.ModuleLessonOrder{ModuleID: authoring.ModuleID(module.ModuleID), LessonIDs: make([]authoring.LessonID, 0, len(*module.LessonIDs))}
		for _, lessonID := range *module.LessonIDs {
			if !validAuthoringUUID(lessonID) {
				return 0, nil, errors.New("invalid lesson identifier")
			}
			item.LessonIDs = append(item.LessonIDs, authoring.LessonID(lessonID))
		}
		order = append(order, item)
	}
	if err := authoring.ValidateLessonOrder(order); err != nil {
		return 0, nil, err
	}
	return *input.ExpectedDraftRevision, order, nil
}

func decodeAuthoringLessonPrerequisites(w http.ResponseWriter, r *http.Request) (int64, []string, error) {
	var input authoringLessonPrerequisitesRequest
	if err := decodeAuthoringLessonJSON(w, r, &input); err != nil {
		return 0, nil, err
	}
	if input.ExpectedLessonRevision == nil || *input.ExpectedLessonRevision < 1 || input.PrerequisiteLessonKeys == nil {
		return 0, nil, errors.New("missing prerequisite replacement fields")
	}
	keys := append([]string(nil), (*input.PrerequisiteLessonKeys)...)
	if err := authoring.ValidatePrerequisiteKeys("", keys); err != nil {
		return 0, nil, err
	}
	return *input.ExpectedLessonRevision, keys, nil
}

func decodeAuthoringLessonDelete(w http.ResponseWriter, r *http.Request) (int64, int64, error) {
	var input authoringLessonDeleteRequest
	if err := decodeAuthoringLessonJSON(w, r, &input); err != nil {
		return 0, 0, err
	}
	if input.ExpectedDraftRevision == nil || *input.ExpectedDraftRevision < 1 || input.ExpectedLessonRevision == nil || *input.ExpectedLessonRevision < 1 {
		return 0, 0, errors.New("missing expected revision")
	}
	return *input.ExpectedDraftRevision, *input.ExpectedLessonRevision, nil
}

func decodeAuthoringLessonContent(w http.ResponseWriter, r *http.Request) (int64, courses.LessonContent, error) {
	var input authoringLessonContentRequest
	if err := decodeAuthoringLessonContentJSON(w, r, &input); err != nil {
		return 0, courses.LessonContent{}, err
	}
	if input.ExpectedLessonRevision == nil || *input.ExpectedLessonRevision < 1 {
		return 0, courses.LessonContent{}, errors.New("missing expected lesson revision")
	}
	if err := input.Content.Validate(); err != nil {
		return 0, courses.LessonContent{}, err
	}
	return *input.ExpectedLessonRevision, input.Content, nil
}

func decodeAuthoringLessonContentJSON(w http.ResponseWriter, r *http.Request, value any) error {
	return decodeAuthoringBody(w, r, maxAuthoringLessonContentBodyBytes, value)
}

func decodeAuthoringMemberAdd(w http.ResponseWriter, r *http.Request) (int64, string, authoring.MemberRole, error) {
	var input authoringMemberAddRequest
	if err := decodeAuthoringMembershipJSON(w, r, &input); err != nil {
		return 0, "", "", err
	}
	if input.ExpectedDraftRevision == nil || *input.ExpectedDraftRevision < 1 || !validAuthoringUUID(input.UserID) || !input.Role.Valid() {
		return 0, "", "", errors.New("invalid membership add")
	}
	return *input.ExpectedDraftRevision, input.UserID, input.Role, nil
}

func decodeAuthoringMemberRole(w http.ResponseWriter, r *http.Request) (int64, authoring.MemberRole, error) {
	var input authoringMemberRoleRequest
	if err := decodeAuthoringMembershipJSON(w, r, &input); err != nil {
		return 0, "", err
	}
	if input.ExpectedDraftRevision == nil || *input.ExpectedDraftRevision < 1 || !input.Role.Valid() {
		return 0, "", errors.New("invalid membership role")
	}
	return *input.ExpectedDraftRevision, input.Role, nil
}

func decodeAuthoringMemberRevoke(w http.ResponseWriter, r *http.Request) (int64, error) {
	var input authoringMemberRevokeRequest
	if err := decodeAuthoringMembershipJSON(w, r, &input); err != nil {
		return 0, err
	}
	if input.ExpectedDraftRevision == nil || *input.ExpectedDraftRevision < 1 {
		return 0, errors.New("missing expected draft revision")
	}
	return *input.ExpectedDraftRevision, nil
}

func decodeAuthoringMembershipJSON(w http.ResponseWriter, r *http.Request, value any) error {
	return decodeAuthoringBody(w, r, maxAuthoringMembershipBodyBytes, value)
}

func decodeAuthoringLessonJSON(w http.ResponseWriter, r *http.Request, value any) error {
	return decodeAuthoringBody(w, r, maxAuthoringLessonBodyBytes, value)
}

func decodeAuthoringJSON(w http.ResponseWriter, r *http.Request, value any) error {
	return decodeAuthoringBody(w, r, maxAuthoringModuleBodyBytes, value)
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
	return authoringLessonSummaryDTO{ID: string(lesson.ID), StableKey: lesson.StableKey, Title: lesson.Title, Description: lesson.Description, Objectives: lesson.LearningObjectives, EstimatedDurationMinutes: lesson.EstimatedDurationMinutes, Position: lesson.Position, Revision: lesson.Revision, RecommendedPrerequisiteKeys: nonNilAuthoringKeys(prerequisites)}
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

func authoringLessonMutation(result authoring.LessonMutationResult) authoringLessonMutationDTO {
	lesson := result.Lesson
	return authoringLessonMutationDTO{ID: string(lesson.ID), DraftID: string(lesson.DraftID), ModuleID: string(lesson.ModuleID), StableKey: lesson.StableKey, Title: lesson.Title, Description: lesson.Description, Objectives: lesson.LearningObjectives, EstimatedDurationMinutes: lesson.EstimatedDurationMinutes, Position: lesson.Position, Revision: lesson.Revision, DraftRevision: result.Draft.Revision}
}

func authoringMemberMutation(result authoring.MembershipMutationResult) authoringMemberMutationDTO {
	member := result.Member
	return authoringMemberMutationDTO{ID: member.ID, UserID: member.UserID, Role: member.Role, CreatedAt: member.CreatedAt, RevokedAt: member.RevokedAt, DraftRevision: result.Draft.Revision}
}

func nonNilAuthoringKeys(keys []string) []string {
	if keys == nil {
		return []string{}
	}
	return keys
}
