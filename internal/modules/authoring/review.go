package authoring

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type ReviewID string
type ReviewStatus string
type ReviewEventType string

const (
	// ReviewEditing is the derived workflow state when no submitted cycle is
	// active. Persisted cycles begin at IN_REVIEW because only they have a frozen snapshot.
	ReviewEditing          ReviewStatus = "EDITING"
	ReviewInReview         ReviewStatus = "IN_REVIEW"
	ReviewApproved         ReviewStatus = "APPROVED"
	ReviewChangesRequested ReviewStatus = "CHANGES_REQUESTED"

	ReviewSubmittedEvent        ReviewEventType = "SUBMITTED"
	ReviewApprovedEvent         ReviewEventType = "APPROVED"
	ReviewChangesRequestedEvent ReviewEventType = "CHANGES_REQUESTED"
)

var (
	ErrReviewNotFound      = errors.New("review not found")
	ErrReviewStale         = errors.New("review revision mismatch")
	ErrReviewInvalidState  = errors.New("invalid review transition")
	ErrReviewAlreadyExists = errors.New("review already exists for draft revision")
	ErrReviewSnapshot      = errors.New("invalid review snapshot")
)

func (s ReviewStatus) ValidCycleStatus() bool {
	return s == ReviewInReview || s == ReviewApproved || s == ReviewChangesRequested
}

type ReviewCycle struct {
	ID                    ReviewID
	DraftID               DraftID
	DraftRevision         int64
	SnapshotSchemaVersion int
	Status                ReviewStatus
	Revision              int64
	SubmittedByUserID     string
	SubmittedAt           time.Time
	DecidedByUserID       string
	DecidedAt             *time.Time
}

func (r ReviewCycle) CanDecide(expected int64) error {
	if r.Status != ReviewInReview {
		return ErrReviewInvalidState
	}
	if expected != r.Revision {
		return ErrReviewStale
	}
	return nil
}

type ReviewEvent struct {
	ID          string
	ReviewID    ReviewID
	Type        ReviewEventType
	ActorUserID string
	Message     string
	CreatedAt   time.Time
}

// ReviewSnapshot is immutable canonical source material for later review and
// publishing. IDs are provenance only; stable keys remain publication identity.
type ReviewSnapshot struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Draft         ReviewSnapshotDraft    `json:"draft"`
	Modules       []ReviewSnapshotModule `json:"modules"`
	// Assessments contains answer-bearing definitions frozen with the Review.
	// It is deliberately absent from the Authoring Review HTTP representation.
	Assessments []ReviewSnapshotAssessment `json:"-"`
}

type ReviewSnapshotAssessment struct {
	AssessmentKey string
	Questions     []assessments.Question
}

type ReviewSnapshotDraft struct {
	ID              DraftID                `json:"id"`
	Revision        int64                  `json:"revision"`
	CourseID        courses.CourseID       `json:"courseId"`
	IntendedVersion string                 `json:"intendedVersion"`
	SourceLanguage  courses.LanguageTag    `json:"sourceLanguage"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	Objectives      []string               `json:"objectives"`
	Changelog       string                 `json:"changelog"`
	License         courses.ContentLicense `json:"license"`
}

type ReviewSnapshotModule struct {
	ID          ModuleID               `json:"id"`
	StableKey   string                 `json:"stableKey"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Position    int                    `json:"position"`
	Lessons     []ReviewSnapshotLesson `json:"lessons"`
}

type ReviewSnapshotLesson struct {
	ID                       LessonID              `json:"id"`
	StableKey                string                `json:"stableKey"`
	Title                    string                `json:"title"`
	Description              string                `json:"description"`
	Objectives               []string              `json:"objectives"`
	EstimatedDurationMinutes *int                  `json:"estimatedDurationMinutes"`
	Position                 int                   `json:"position"`
	PrerequisiteStableKeys   []string              `json:"prerequisiteStableKeys"`
	Content                  courses.LessonContent `json:"content"`
}

func NewReviewSnapshot(draft CourseDraft, modules []DraftModule, lessons []DraftLesson, prerequisites []Prerequisite) (ReviewSnapshot, error) {
	byModule := make(map[ModuleID][]ReviewSnapshotLesson, len(modules))
	keys := make(map[LessonID][]string)
	lessonKeysByID := make(map[LessonID]string, len(lessons))
	for _, lesson := range lessons {
		if lesson.ID == "" || lessonKeysByID[lesson.ID] != "" {
			return ReviewSnapshot{}, ErrReviewSnapshot
		}
		lessonKeysByID[lesson.ID] = lesson.StableKey
	}
	for _, prerequisite := range prerequisites {
		if lessonKeysByID[prerequisite.LessonID] == "" || lessonKeysByID[prerequisite.TargetLessonID] != prerequisite.TargetStableKey || prerequisite.Position != len(keys[prerequisite.LessonID]) {
			return ReviewSnapshot{}, ErrReviewSnapshot
		}
		keys[prerequisite.LessonID] = append(keys[prerequisite.LessonID], prerequisite.TargetStableKey)
	}
	for _, lesson := range lessons {
		content, err := cloneLessonContent(lesson.Content)
		if err != nil {
			return ReviewSnapshot{}, ErrReviewSnapshot
		}
		prerequisiteKeys := append([]string{}, keys[lesson.ID]...)
		byModule[lesson.ModuleID] = append(byModule[lesson.ModuleID], ReviewSnapshotLesson{
			ID:                       lesson.ID,
			StableKey:                lesson.StableKey,
			Title:                    lesson.Title,
			Description:              lesson.Description,
			Objectives:               append([]string{}, lesson.LearningObjectives...),
			EstimatedDurationMinutes: cloneInt(lesson.EstimatedDurationMinutes),
			Position:                 lesson.Position,
			PrerequisiteStableKeys:   prerequisiteKeys,
			Content:                  content,
		})
	}
	snapshot := ReviewSnapshot{SchemaVersion: 1, Draft: ReviewSnapshotDraft{
		ID: draft.ID, Revision: draft.Revision, CourseID: draft.Metadata.CourseID,
		IntendedVersion: draft.Metadata.IntendedVersion.String(), SourceLanguage: draft.Metadata.SourceLanguage,
		Title: draft.Metadata.Title, Description: draft.Metadata.Description,
		Objectives: append([]string{}, draft.Metadata.LearningObjectives...), Changelog: draft.Metadata.Changelog,
		License: draft.Metadata.License,
	}, Modules: make([]ReviewSnapshotModule, 0, len(modules)), Assessments: []ReviewSnapshotAssessment{}}
	includedLessons := 0
	for _, module := range modules {
		moduleLessons := byModule[module.ID]
		if moduleLessons == nil {
			moduleLessons = []ReviewSnapshotLesson{}
		}
		includedLessons += len(moduleLessons)
		snapshot.Modules = append(snapshot.Modules, ReviewSnapshotModule{ID: module.ID, StableKey: module.StableKey, Title: module.Title, Description: module.Description, Position: module.Position, Lessons: moduleLessons})
	}
	if includedLessons != len(lessons) {
		return ReviewSnapshot{}, ErrReviewSnapshot
	}
	if err := snapshot.Validate(); err != nil {
		return ReviewSnapshot{}, err
	}
	return snapshot, nil
}

func (s ReviewSnapshot) Validate() error {
	if s.SchemaVersion != 1 || s.Draft.ID == "" || s.Draft.Revision < 1 || s.Modules == nil {
		return ErrReviewSnapshot
	}
	version, err := courses.ParseVersion(s.Draft.IntendedVersion)
	if err != nil {
		return ErrReviewSnapshot
	}
	metadata := DraftMetadata{CourseID: s.Draft.CourseID, IntendedVersion: version, SourceLanguage: s.Draft.SourceLanguage, Title: s.Draft.Title, Description: s.Draft.Description, LearningObjectives: s.Draft.Objectives, Changelog: s.Draft.Changelog, License: s.Draft.License}
	if metadata.Validate() != nil || len(s.Modules) > MaxModulesPerDraft {
		return ErrReviewSnapshot
	}
	moduleIDs, moduleKeys := map[ModuleID]bool{}, map[string]bool{}
	lessonIDs, lessonKeys := map[LessonID]bool{}, map[string]bool{}
	lessonCount := 0
	for modulePosition, module := range s.Modules {
		if module.ID == "" || module.Position != modulePosition || moduleIDs[module.ID] || moduleKeys[module.StableKey] || module.Lessons == nil {
			return ErrReviewSnapshot
		}
		if (ModuleInput{DraftID: s.Draft.ID, StableKey: module.StableKey, Title: module.Title, Description: module.Description, Position: module.Position}).Validate() != nil {
			return ErrReviewSnapshot
		}
		moduleIDs[module.ID] = true
		moduleKeys[module.StableKey] = true
		for lessonPosition, lesson := range module.Lessons {
			lessonCount++
			if lesson.ID == "" || lesson.Position != lessonPosition || lessonIDs[lesson.ID] || lessonKeys[lesson.StableKey] || lesson.PrerequisiteStableKeys == nil {
				return ErrReviewSnapshot
			}
			input := LessonInput{DraftID: s.Draft.ID, ModuleID: module.ID, StableKey: lesson.StableKey, Title: lesson.Title, Description: lesson.Description, LearningObjectives: lesson.Objectives, EstimatedDurationMinutes: lesson.EstimatedDurationMinutes, Position: lesson.Position, Content: lesson.Content}
			if input.Validate() != nil || ValidatePrerequisiteKeys(lesson.StableKey, lesson.PrerequisiteStableKeys) != nil {
				return ErrReviewSnapshot
			}
			lessonIDs[lesson.ID] = true
			lessonKeys[lesson.StableKey] = true
		}
	}
	if lessonCount > MaxLessonsPerDraft {
		return ErrReviewSnapshot
	}
	for _, module := range s.Modules {
		for _, lesson := range module.Lessons {
			for _, key := range lesson.PrerequisiteStableKeys {
				if !lessonKeys[key] {
					return ErrReviewSnapshot
				}
			}
		}
	}
	referencedAssessments := make(map[string]struct{})
	for _, module := range s.Modules {
		for _, lesson := range module.Lessons {
			for _, block := range lesson.Content.Blocks {
				if payload, ok := block.Payload.(courses.KnowledgeCheckBlockPayload); ok {
					referencedAssessments[payload.AssessmentKey] = struct{}{}
				}
			}
		}
	}
	previousKey := ""
	for _, frozen := range s.Assessments {
		if _, err := assessments.ParseAssessmentID(frozen.AssessmentKey); err != nil || frozen.Questions == nil || previousKey != "" && frozen.AssessmentKey <= previousKey {
			return ErrReviewSnapshot
		}
		if _, referenced := referencedAssessments[frozen.AssessmentKey]; !referenced {
			return ErrReviewSnapshot
		}
		if err := (assessments.AssessmentUpdate{Title: "Frozen assessment", Questions: frozen.Questions}).Validate(); err != nil {
			return ErrReviewSnapshot
		}
		previousKey = frozen.AssessmentKey
	}
	return nil
}

func cloneLessonContent(content courses.LessonContent) (courses.LessonContent, error) {
	encoded, err := courses.MarshalLessonContent(content)
	if err != nil {
		return courses.LessonContent{}, err
	}
	return courses.ParseLessonContent(encoded)
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func ValidateReviewActor(actor string) error {
	if strings.TrimSpace(actor) == "" {
		return errors.New("review actor required")
	}
	return nil
}

func ValidateReviewMessage(message string) error {
	if len(message) > 20000 {
		return errors.New("review message too long")
	}
	return nil
}

type ReviewRepository interface {
	SubmitReview(context.Context, DraftID, int64, string) (ReviewCycle, ReviewSnapshot, error)
	GetReview(context.Context, ReviewID) (ReviewCycle, ReviewSnapshot, error)
	GetReviewForDraft(context.Context, DraftID, ReviewID) (ReviewCycle, ReviewSnapshot, error)
	LatestReview(context.Context, DraftID) (ReviewCycle, ReviewSnapshot, error)
	ActiveReview(context.Context, DraftID) (ReviewCycle, ReviewSnapshot, error)
	ListReviewHistory(context.Context, DraftID) ([]ReviewCycle, error)
	DecideReview(context.Context, ReviewID, int64, ReviewStatus, string, string) (ReviewCycle, error)
	DecideReviewForDraft(context.Context, DraftID, ReviewID, int64, ReviewStatus, string, string) (ReviewCycle, error)
	ListReviewEvents(context.Context, ReviewID) ([]ReviewEvent, error)
	ApprovedReviewForRevision(context.Context, DraftID, int64) (ReviewCycle, ReviewSnapshot, error)
}

type ReviewService struct{ reviews ReviewRepository }

func NewReviewService(reviews ReviewRepository) *ReviewService {
	return &ReviewService{reviews: reviews}
}
func (s *ReviewService) SubmitForReview(ctx context.Context, draft DraftID, expected int64, actor string) (ReviewCycle, ReviewSnapshot, error) {
	if s == nil || s.reviews == nil || draft == "" || expected < 1 || ValidateReviewActor(actor) != nil {
		return ReviewCycle{}, ReviewSnapshot{}, ErrReviewSnapshot
	}
	return s.reviews.SubmitReview(ctx, draft, expected, actor)
}
func (s *ReviewService) ApproveReview(ctx context.Context, review ReviewID, expected int64, actor, message string) (ReviewCycle, error) {
	return s.decide(ctx, review, expected, ReviewApproved, actor, message)
}
func (s *ReviewService) RequestChanges(ctx context.Context, review ReviewID, expected int64, actor, message string) (ReviewCycle, error) {
	return s.decide(ctx, review, expected, ReviewChangesRequested, actor, message)
}
func (s *ReviewService) decide(ctx context.Context, review ReviewID, expected int64, status ReviewStatus, actor, message string) (ReviewCycle, error) {
	if s == nil || s.reviews == nil || review == "" || expected < 1 || ValidateReviewActor(actor) != nil || ValidateReviewMessage(message) != nil {
		return ReviewCycle{}, ErrReviewInvalidState
	}
	return s.reviews.DecideReview(ctx, review, expected, status, actor, message)
}

// MarshalReviewSnapshot validates the versioned canonical document before it
// crosses the persistence boundary.
func MarshalReviewSnapshot(snapshot ReviewSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(reviewSnapshotDocumentFrom(snapshot))
	if err != nil {
		return nil, ErrReviewSnapshot
	}
	return encoded, nil
}

// UnmarshalReviewSnapshot reconstructs private frozen Assessment definitions
// from persistence without adding answer keys to the Review HTTP projection.
func UnmarshalReviewSnapshot(encoded []byte) (ReviewSnapshot, error) {
	var document reviewSnapshotDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		return ReviewSnapshot{}, ErrReviewSnapshot
	}
	snapshot := document.reviewSnapshot()
	if err := snapshot.Validate(); err != nil {
		return ReviewSnapshot{}, err
	}
	return snapshot, nil
}

type reviewSnapshotDocument struct {
	SchemaVersion int                                `json:"schemaVersion"`
	Draft         ReviewSnapshotDraft                `json:"draft"`
	Modules       []ReviewSnapshotModule             `json:"modules"`
	Assessments   []reviewSnapshotAssessmentDocument `json:"assessments"`
}

type reviewSnapshotAssessmentDocument struct {
	AssessmentKey string                           `json:"assessmentKey"`
	Questions     []reviewSnapshotQuestionDocument `json:"questions"`
}

type reviewSnapshotQuestionDocument struct {
	StableKey         string                     `json:"stableKey"`
	Type              assessments.QuestionType   `json:"type"`
	Prompt            string                     `json:"prompt"`
	Position          int                        `json:"position"`
	Options           []assessments.ChoiceOption `json:"options,omitempty"`
	CorrectOptionKeys []string                   `json:"correctOptionKeys,omitempty"`
	LeftItems         []assessments.MatchingItem `json:"leftItems,omitempty"`
	RightItems        []assessments.MatchingItem `json:"rightItems,omitempty"`
	CorrectPairs      []assessments.MatchingPair `json:"correctPairs,omitempty"`
}

func reviewSnapshotDocumentFrom(snapshot ReviewSnapshot) reviewSnapshotDocument {
	document := reviewSnapshotDocument{SchemaVersion: snapshot.SchemaVersion, Draft: snapshot.Draft, Modules: snapshot.Modules, Assessments: make([]reviewSnapshotAssessmentDocument, 0, len(snapshot.Assessments))}
	for _, frozen := range snapshot.Assessments {
		item := reviewSnapshotAssessmentDocument{AssessmentKey: frozen.AssessmentKey, Questions: make([]reviewSnapshotQuestionDocument, 0, len(frozen.Questions))}
		for _, question := range frozen.Questions {
			item.Questions = append(item.Questions, reviewSnapshotQuestionDocument{
				StableKey: question.StableKey, Type: question.Type, Prompt: question.Prompt, Position: question.Position,
				Options: append([]assessments.ChoiceOption(nil), question.Options...), CorrectOptionKeys: append([]string(nil), question.CorrectOptionKeys...),
				LeftItems: append([]assessments.MatchingItem(nil), question.LeftItems...), RightItems: append([]assessments.MatchingItem(nil), question.RightItems...), CorrectPairs: append([]assessments.MatchingPair(nil), question.CorrectPairs...),
			})
		}
		document.Assessments = append(document.Assessments, item)
	}
	return document
}

func (document reviewSnapshotDocument) reviewSnapshot() ReviewSnapshot {
	snapshot := ReviewSnapshot{SchemaVersion: document.SchemaVersion, Draft: document.Draft, Modules: document.Modules, Assessments: make([]ReviewSnapshotAssessment, 0, len(document.Assessments))}
	for _, frozen := range document.Assessments {
		item := ReviewSnapshotAssessment{AssessmentKey: frozen.AssessmentKey, Questions: make([]assessments.Question, 0, len(frozen.Questions))}
		for _, question := range frozen.Questions {
			item.Questions = append(item.Questions, assessments.Question{
				StableKey: question.StableKey, Type: question.Type, Prompt: question.Prompt, Position: question.Position,
				Options: append([]assessments.ChoiceOption(nil), question.Options...), CorrectOptionKeys: append([]string(nil), question.CorrectOptionKeys...),
				LeftItems: append([]assessments.MatchingItem(nil), question.LeftItems...), RightItems: append([]assessments.MatchingItem(nil), question.RightItems...), CorrectPairs: append([]assessments.MatchingPair(nil), question.CorrectPairs...),
			})
		}
		snapshot.Assessments = append(snapshot.Assessments, item)
	}
	return snapshot
}
