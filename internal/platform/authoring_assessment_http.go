package platform

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
)

const maxAuthoringAssessmentBodyBytes int64 = 256 * 1024

// AuthoringAssessmentDTO is deliberately answer-bearing and only used by
// Draft-authoring routes. Future learner routes must use a different DTO.
type AuthoringAssessmentDTO struct {
	AssessmentKey string                           `json:"assessmentKey"`
	Title         string                           `json:"title"`
	Revision      int64                            `json:"revision"`
	Questions     []authoringAssessmentQuestionDTO `json:"questions"`
	CreatedAt     time.Time                        `json:"createdAt"`
	UpdatedAt     time.Time                        `json:"updatedAt"`
}

type authoringAssessmentQuestionDTO struct {
	StableKey         string                           `json:"stableKey"`
	Type              assessments.QuestionType         `json:"type"`
	Prompt            string                           `json:"prompt"`
	Position          int                              `json:"position"`
	Options           []authoringAssessmentOptionDTO   `json:"options"`
	CorrectOptionKeys []string                         `json:"correctOptionKeys"`
	LeftItems         []authoringAssessmentMatchingDTO `json:"leftItems"`
	RightItems        []authoringAssessmentMatchingDTO `json:"rightItems"`
	CorrectPairs      []authoringAssessmentPairDTO     `json:"correctPairs"`
}

type authoringAssessmentOptionDTO struct {
	StableKey string `json:"stableKey"`
	Text      string `json:"text"`
	Position  int    `json:"position"`
}

type authoringAssessmentMatchingDTO struct {
	StableKey string `json:"stableKey"`
	Text      string `json:"text"`
	Position  int    `json:"position"`
}

type authoringAssessmentPairDTO struct {
	LeftKey  string `json:"leftKey"`
	RightKey string `json:"rightKey"`
}

type authoringAssessmentCreateRequest struct {
	Title     string                           `json:"title"`
	Questions []authoringAssessmentQuestionDTO `json:"questions"`
}

type authoringAssessmentUpdateRequest struct {
	ExpectedRevision *int64                           `json:"expectedRevision"`
	Title            string                           `json:"title"`
	Questions        []authoringAssessmentQuestionDTO `json:"questions"`
}

type authoringAssessmentSummaryDTO struct {
	AssessmentKey string    `json:"assessmentKey"`
	Title         string    `json:"title"`
	QuestionCount int       `json:"questionCount"`
	Revision      int64     `json:"revision"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type authoringAssessmentListDTO struct {
	Items  []authoringAssessmentSummaryDTO `json:"items"`
	Limit  int                             `json:"limit"`
	Offset int                             `json:"offset"`
	Total  int                             `json:"total"`
}

func (h *authHTTP) handleAuthoringAssessmentCreate(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	definition, err := decodeAuthoringAssessmentCreate(w, r)
	if err != nil {
		problemCode(w, r, http.StatusBadRequest, "Invalid assessment request", "invalid_assessment")
		return
	}
	if h.authoringAssessments == nil {
		problem(w, r, http.StatusInternalServerError, "Assessment service unavailable")
		return
	}
	assessment, err := h.authoringAssessments.Create(r.Context(), actor, draftID, definition)
	if err != nil {
		authoringAssessmentProblem(w, r, err)
		return
	}
	w.Header().Set("Location", r.URL.Path+"/"+string(assessment.ID))
	writeJSON(w, http.StatusCreated, authoringAssessmentDetail(assessment))
}

func (h *authHTTP) handleAuthoringAssessmentList(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	limit, offset, err := authoringAssessmentListQuery(r)
	if err != nil {
		problemCode(w, r, http.StatusBadRequest, "Invalid assessment query", "invalid_assessment_query")
		return
	}
	if h.authoringAssessments == nil {
		problem(w, r, http.StatusInternalServerError, "Assessment service unavailable")
		return
	}
	page, err := h.authoringAssessments.List(r.Context(), actor, draftID, limit, offset)
	if err != nil {
		authoringAssessmentProblem(w, r, err)
		return
	}
	items := make([]authoringAssessmentSummaryDTO, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, authoringAssessmentSummaryDTO{
			AssessmentKey: string(item.ID), Title: item.Title, QuestionCount: item.QuestionCount, Revision: item.Revision, UpdatedAt: item.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, authoringAssessmentListDTO{Items: items, Limit: page.Limit, Offset: page.Offset, Total: page.Total})
}

func (h *authHTTP) handleAuthoringAssessmentGet(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	id, ok := authoringAssessmentID(w, r)
	if !ok {
		return
	}
	if h.authoringAssessments == nil {
		problem(w, r, http.StatusInternalServerError, "Assessment service unavailable")
		return
	}
	assessment, err := h.authoringAssessments.Get(r.Context(), actor, draftID, id)
	if err != nil {
		authoringAssessmentProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringAssessmentDetail(assessment))
}

func (h *authHTTP) handleAuthoringAssessmentUpdate(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	id, ok := authoringAssessmentID(w, r)
	if !ok {
		return
	}
	expectedRevision, definition, err := decodeAuthoringAssessmentUpdate(w, r)
	if err != nil {
		problemCode(w, r, http.StatusBadRequest, "Invalid assessment request", "invalid_assessment")
		return
	}
	if h.authoringAssessments == nil {
		problem(w, r, http.StatusInternalServerError, "Assessment service unavailable")
		return
	}
	assessment, err := h.authoringAssessments.Update(r.Context(), actor, draftID, id, expectedRevision, definition)
	if err != nil {
		authoringAssessmentProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authoringAssessmentDetail(assessment))
}

func authoringAssessmentID(w http.ResponseWriter, r *http.Request) (assessments.AssessmentID, bool) {
	value, ok := authoringUUIDParam(w, r, "assessmentId", "Invalid assessment identifier")
	if !ok {
		return "", false
	}
	id, err := assessments.ParseAssessmentID(string(value))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid assessment identifier")
		return "", false
	}
	return id, true
}

func decodeAuthoringAssessmentCreate(w http.ResponseWriter, r *http.Request) (authoring.AssessmentDefinition, error) {
	var input authoringAssessmentCreateRequest
	if err := decodeAuthoringBody(w, r, maxAuthoringAssessmentBodyBytes, &input); err != nil {
		return authoring.AssessmentDefinition{}, err
	}
	return authoringAssessmentDefinition(input.Title, input.Questions), nil
}

func decodeAuthoringAssessmentUpdate(w http.ResponseWriter, r *http.Request) (int64, authoring.AssessmentDefinition, error) {
	var input authoringAssessmentUpdateRequest
	if err := decodeAuthoringBody(w, r, maxAuthoringAssessmentBodyBytes, &input); err != nil {
		return 0, authoring.AssessmentDefinition{}, err
	}
	if input.ExpectedRevision == nil || *input.ExpectedRevision < 1 {
		return 0, authoring.AssessmentDefinition{}, errors.New("expected revision required")
	}
	return *input.ExpectedRevision, authoringAssessmentDefinition(input.Title, input.Questions), nil
}

func authoringAssessmentDefinition(title string, questions []authoringAssessmentQuestionDTO) authoring.AssessmentDefinition {
	definition := authoring.AssessmentDefinition{Title: title, Questions: make([]assessments.Question, 0, len(questions))}
	for _, question := range questions {
		definition.Questions = append(definition.Questions, assessments.Question{
			StableKey: question.StableKey, Type: question.Type, Prompt: question.Prompt, Position: question.Position,
			Options: authoringAssessmentOptions(question.Options), CorrectOptionKeys: append([]string(nil), question.CorrectOptionKeys...),
			LeftItems: authoringAssessmentItems(question.LeftItems), RightItems: authoringAssessmentItems(question.RightItems), CorrectPairs: authoringAssessmentPairs(question.CorrectPairs),
		})
	}
	return definition
}

func authoringAssessmentOptions(options []authoringAssessmentOptionDTO) []assessments.ChoiceOption {
	items := make([]assessments.ChoiceOption, 0, len(options))
	for _, option := range options {
		items = append(items, assessments.ChoiceOption{StableKey: option.StableKey, Text: option.Text, Position: option.Position})
	}
	return items
}

func authoringAssessmentItems(source []authoringAssessmentMatchingDTO) []assessments.MatchingItem {
	items := make([]assessments.MatchingItem, 0, len(source))
	for _, item := range source {
		items = append(items, assessments.MatchingItem{StableKey: item.StableKey, Text: item.Text, Position: item.Position})
	}
	return items
}

func authoringAssessmentPairs(source []authoringAssessmentPairDTO) []assessments.MatchingPair {
	pairs := make([]assessments.MatchingPair, 0, len(source))
	for _, pair := range source {
		pairs = append(pairs, assessments.MatchingPair{LeftKey: pair.LeftKey, RightKey: pair.RightKey})
	}
	return pairs
}

func authoringAssessmentDetail(assessment assessments.Assessment) AuthoringAssessmentDTO {
	questions := make([]authoringAssessmentQuestionDTO, 0, len(assessment.Questions))
	for _, question := range assessment.Questions {
		questions = append(questions, authoringAssessmentQuestionDTO{
			StableKey: question.StableKey, Type: question.Type, Prompt: question.Prompt, Position: question.Position,
			Options: assessmentOptionsDTO(question.Options), CorrectOptionKeys: nonNilStrings(question.CorrectOptionKeys),
			LeftItems: assessmentItemsDTO(question.LeftItems), RightItems: assessmentItemsDTO(question.RightItems), CorrectPairs: assessmentPairsDTO(question.CorrectPairs),
		})
	}
	return AuthoringAssessmentDTO{AssessmentKey: string(assessment.ID), Title: assessment.Title, Revision: assessment.Revision, Questions: questions, CreatedAt: assessment.CreatedAt, UpdatedAt: assessment.UpdatedAt}
}

func assessmentOptionsDTO(source []assessments.ChoiceOption) []authoringAssessmentOptionDTO {
	items := make([]authoringAssessmentOptionDTO, 0, len(source))
	for _, item := range source {
		items = append(items, authoringAssessmentOptionDTO{StableKey: item.StableKey, Text: item.Text, Position: item.Position})
	}
	return items
}

func assessmentItemsDTO(source []assessments.MatchingItem) []authoringAssessmentMatchingDTO {
	items := make([]authoringAssessmentMatchingDTO, 0, len(source))
	for _, item := range source {
		items = append(items, authoringAssessmentMatchingDTO{StableKey: item.StableKey, Text: item.Text, Position: item.Position})
	}
	return items
}

func assessmentPairsDTO(source []assessments.MatchingPair) []authoringAssessmentPairDTO {
	pairs := make([]authoringAssessmentPairDTO, 0, len(source))
	for _, pair := range source {
		pairs = append(pairs, authoringAssessmentPairDTO{LeftKey: pair.LeftKey, RightKey: pair.RightKey})
	}
	return pairs
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return append([]string(nil), values...)
}

func authoringAssessmentListQuery(r *http.Request) (int, int, error) {
	values := r.URL.Query()
	limit, offset := 0, 0
	for _, item := range []struct {
		name string
		to   *int
		min  int
	}{{"limit", &limit, 1}, {"offset", &offset, 0}} {
		raw, present := values[item.name]
		if !present {
			continue
		}
		if len(raw) != 1 || raw[0] == "" {
			return 0, 0, errors.New("invalid query")
		}
		parsed, err := strconv.Atoi(raw[0])
		if err != nil || parsed < item.min {
			return 0, 0, errors.New("invalid query")
		}
		*item.to = parsed
	}
	return limit, offset, nil
}

func authoringAssessmentProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authoring.ErrNotFound), errors.Is(err, authoring.ErrAuthorizationDenied), errors.Is(err, assessments.ErrAssessmentNotFound):
		problem(w, r, http.StatusNotFound, "Not found")
	case errors.Is(err, assessments.ErrRevisionMismatch):
		problemCode(w, r, http.StatusConflict, "Assessment revision conflict", "assessment_revision_conflict")
	case errors.Is(err, assessments.ErrInvalidAssessment):
		problemCode(w, r, http.StatusBadRequest, "Invalid assessment definition", "invalid_assessment")
	default:
		problem(w, r, http.StatusInternalServerError, "Assessment service unavailable")
	}
}
