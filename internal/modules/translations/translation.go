package translations

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/google/uuid"
)

var (
	ErrInvalidTranslation  = errors.New("invalid course translation")
	ErrTranslationNotFound = errors.New("course translation not found")
	ErrTranslationConflict = errors.New("course translation conflicts with existing data")
	ErrRevisionMismatch    = errors.New("translation revision mismatch")
)

type TranslationID string
type TranslationPublicationID string
type TranslationStatus string

const (
	TranslationDraft     TranslationStatus = "DRAFT"
	TranslationPublished TranslationStatus = "PUBLISHED"
)

// SourceCourseVersion is frozen when a workspace is created. The version ID is
// the authoritative binding; the remaining facts make historical reads and
// later source-lag detection independent of latest-version lookup.
type SourceCourseVersion struct {
	CourseID        courses.CourseID
	CourseVersionID courses.CourseVersionID
	Version         courses.Version
	Language        courses.LanguageTag
}

// CourseTranslation is a mutable derived workspace. The Tree contains only
// structural source references and optional human translations; source content
// and grading rules remain in the immutable Courses aggregate.
type CourseTranslation struct {
	ID             TranslationID
	Source         SourceCourseVersion
	TargetLanguage courses.LanguageTag
	CreatorUserID  string `json:"-"`
	Status         TranslationStatus
	Revision       int64
	Tree           TranslationTree
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TranslationPublication is an immutable snapshot of one workspace revision.
// It deliberately has no SemVer: its meaning is "translation of source X",
// never an independently-versioned Course.
type TranslationPublication struct {
	ID             TranslationPublicationID
	TranslationID  TranslationID
	Source         SourceCourseVersion
	TargetLanguage courses.LanguageTag
	Revision       int64
	Tree           TranslationTree
	PublishedAt    time.Time
}

// TranslationTree mirrors source ordering and stable keys. Nil text pointers
// mean untranslated; a non-nil pointer to "" is an intentional empty value.
// No source display text is copied into this structure, so an untranslated
// field can never be mistaken for a completed translation.
type TranslationTree struct {
	Title              *string
	Description        *string
	LearningObjectives []*string
	Modules            []TranslatedModule
	Assessments        []TranslatedAssessment
}

type TranslatedModule struct {
	SourceStableKey string
	Position        int
	Title           *string
	Description     *string
	Lessons         []TranslatedLesson
}

type TranslatedLesson struct {
	SourceStableKey    string
	Position           int
	Title              *string
	Description        *string
	LearningObjectives []*string
	ContentBlocks      []TranslatedContentBlock
}

// TranslatedContentBlock preserves the source block key and type. Only fields
// which are human-facing for the corresponding source block may be populated.
// In particular, CODE.Code and all asset references remain source-owned.
type TranslatedContentBlock struct {
	SourceBlockKey string
	Type           courses.BlockType
	Text           *string
	Title          *string
	AltText        *string
	Caption        *string
	Attribution    *string
	Transcript     *string
	Label          *string
	Description    *string
}

type TranslatedAssessment struct {
	AssessmentKey string
	Questions     []TranslatedAssessmentQuestion
}
type TranslatedAssessmentQuestion struct {
	SourceStableKey string
	Type            courses.PublishedAssessmentQuestionType
	Position        int
	Prompt          *string
	Options         []TranslatedAssessmentOption
	LeftItems       []TranslatedAssessmentItem
	RightItems      []TranslatedAssessmentItem
}
type TranslatedAssessmentOption struct {
	SourceStableKey string
	Position        int
	Text            *string
}
type TranslatedAssessmentItem struct {
	SourceStableKey string
	Position        int
	Text            *string
}

func (s SourceCourseVersion) Validate() error {
	if !validUUID(string(s.CourseID)) || !validUUID(string(s.CourseVersionID)) || !s.Version.Valid() || s.Language == "" {
		return ErrInvalidTranslation
	}
	lang, err := courses.NormalizeLanguageTag(string(s.Language))
	if err != nil || lang != s.Language {
		return ErrInvalidTranslation
	}
	return nil
}

func (t CourseTranslation) Validate() error {
	if !validUUID(string(t.ID)) || t.Source.Validate() != nil || t.TargetLanguage == "" || t.TargetLanguage == t.Source.Language || !validUUID(t.CreatorUserID) || t.Revision < 1 || t.CreatedAt.IsZero() || t.UpdatedAt.IsZero() || t.UpdatedAt.Before(t.CreatedAt) {
		return ErrInvalidTranslation
	}
	lang, err := courses.NormalizeLanguageTag(string(t.TargetLanguage))
	if err != nil || lang != t.TargetLanguage {
		return ErrInvalidTranslation
	}
	if t.Status != TranslationDraft && t.Status != TranslationPublished {
		return ErrInvalidTranslation
	}
	return t.Tree.validateOptionalValues()
}

func (p TranslationPublication) Validate() error {
	if !validUUID(string(p.ID)) || !validUUID(string(p.TranslationID)) || p.Source.Validate() != nil || p.TargetLanguage == "" || p.TargetLanguage == p.Source.Language || p.Revision < 1 || p.PublishedAt.IsZero() {
		return ErrInvalidTranslation
	}
	if normalized, err := courses.NormalizeLanguageTag(string(p.TargetLanguage)); err != nil || normalized != p.TargetLanguage {
		return ErrInvalidTranslation
	}
	return p.Tree.validateOptionalValues()
}

func (tree TranslationTree) validateOptionalValues() error {
	if err := validOptional(tree.Title, 240); err != nil || validOptional(tree.Description, 10000) != nil {
		return ErrInvalidTranslation
	}
	for _, value := range tree.LearningObjectives {
		if validOptional(value, 4000) != nil {
			return ErrInvalidTranslation
		}
	}
	for _, module := range tree.Modules {
		if !validKey(module.SourceStableKey) || module.Position < 0 || validOptional(module.Title, 240) != nil || validOptional(module.Description, 10000) != nil {
			return ErrInvalidTranslation
		}
		for _, lesson := range module.Lessons {
			if !validKey(lesson.SourceStableKey) || lesson.Position < 0 || validOptional(lesson.Title, 240) != nil || validOptional(lesson.Description, 10000) != nil {
				return ErrInvalidTranslation
			}
			for _, objective := range lesson.LearningObjectives {
				if validOptional(objective, 4000) != nil {
					return ErrInvalidTranslation
				}
			}
			for _, block := range lesson.ContentBlocks {
				if !validKey(block.SourceBlockKey) || block.Type == "" || !validBlockFields(block) {
					return ErrInvalidTranslation
				}
			}
		}
	}
	for _, assessment := range tree.Assessments {
		if !validUUID(assessment.AssessmentKey) {
			return ErrInvalidTranslation
		}
		for _, question := range assessment.Questions {
			if !validAssessmentKey(question.SourceStableKey) || question.Type == "" || question.Position < 0 || validOptional(question.Prompt, 10000) != nil {
				return ErrInvalidTranslation
			}
			for _, option := range question.Options {
				if !validAssessmentKey(option.SourceStableKey) || option.Position < 0 || validOptional(option.Text, 4000) != nil {
					return ErrInvalidTranslation
				}
			}
			for _, item := range append(append([]TranslatedAssessmentItem{}, question.LeftItems...), question.RightItems...) {
				if !validAssessmentKey(item.SourceStableKey) || item.Position < 0 || validOptional(item.Text, 4000) != nil {
					return ErrInvalidTranslation
				}
			}
		}
	}
	return nil
}

// ValidateForPersistence checks the self-contained shape of a stored tree.
// Full source-key/ordering validation is deliberately performed by Service,
// which owns the immutable Courses read required for that comparison.
func (tree TranslationTree) ValidateForPersistence() error { return tree.validateOptionalValues() }

func validBlockFields(block TranslatedContentBlock) bool {
	values := []*string{block.Text, block.Title, block.AltText, block.Caption, block.Attribution, block.Transcript, block.Label, block.Description}
	for _, value := range values {
		if validOptional(value, courses.MaxBlockTextCharacters) != nil {
			return false
		}
	}
	allowed := func(text, title, alt, caption, attribution, transcript, label, description bool) bool {
		return (block.Text == nil || text) && (block.Title == nil || title) &&
			(block.AltText == nil || alt) && (block.Caption == nil || caption) &&
			(block.Attribution == nil || attribution) && (block.Transcript == nil || transcript) &&
			(block.Label == nil || label) && (block.Description == nil || description)
	}
	switch block.Type {
	case courses.BlockText, courses.BlockHeading:
		return allowed(true, false, false, false, false, false, false, false)
	case courses.BlockImage:
		return allowed(false, false, true, true, false, false, false, false)
	case courses.BlockVideo, courses.BlockAudio:
		return allowed(false, true, false, false, false, true, false, false)
	case courses.BlockCode:
		return allowed(false, true, false, false, false, false, false, false)
	case courses.BlockQuote:
		return allowed(true, false, false, false, true, false, false, false)
	case courses.BlockCallout:
		return allowed(true, true, false, false, false, false, false, false)
	case courses.BlockTable:
		return allowed(false, false, false, true, false, false, false, false)
	case courses.BlockDownload:
		return allowed(false, false, false, false, false, false, true, true)
	case courses.BlockKnowledgeCheck, courses.BlockDivider:
		return allowed(false, false, false, false, false, false, false, false)
	default:
		return false
	}
}
func validOptional(value *string, maximum int) error {
	if value != nil && (!utf8.ValidString(*value) || len(*value) > maximum) {
		return ErrInvalidTranslation
	}
	return nil
}
func validKey(value string) bool {
	normalized, err := courses.NormalizeStructureKey(value)
	return err == nil && normalized == value
}
func validAssessmentKey(value string) bool {
	return value != "" && len(value) <= 160 && strings.TrimSpace(value) == value
}
func validUUID(value string) bool { _, err := uuid.Parse(value); return err == nil }

// SeedTranslationTree derives a zero-content translation tree from exactly one
// immutable published source. It never copies source prose into translated
// fields and never consults Authoring or latest CourseVersion state.
func SeedTranslationTree(source courses.ImmutableCourseVersion) (TranslationTree, error) {
	if source.ID == "" || source.CourseVersion.Status != courses.CourseVersionPublished || source.CourseVersion.CourseID == "" || !source.CourseVersion.Version.Valid() || source.CourseVersion.SourceLanguage == "" {
		return TranslationTree{}, ErrInvalidTranslation
	}
	tree := TranslationTree{LearningObjectives: make([]*string, len(source.CourseVersion.LearningObjectives)), Modules: make([]TranslatedModule, 0, len(source.Modules)), Assessments: make([]TranslatedAssessment, 0, len(source.AssessmentBindings))}
	for modulePosition, module := range source.Modules {
		if module.Position != modulePosition || !validKey(module.StableKey) {
			return TranslationTree{}, ErrInvalidTranslation
		}
		translatedModule := TranslatedModule{SourceStableKey: module.StableKey, Position: module.Position, Lessons: make([]TranslatedLesson, 0, len(module.Lessons))}
		for lessonPosition, lesson := range module.Lessons {
			if lesson.Position != lessonPosition || !validKey(lesson.StableKey) || lesson.Content.Validate() != nil {
				return TranslationTree{}, ErrInvalidTranslation
			}
			translatedLesson := TranslatedLesson{SourceStableKey: lesson.StableKey, Position: lesson.Position, LearningObjectives: make([]*string, len(lesson.LearningObjectives)), ContentBlocks: make([]TranslatedContentBlock, 0, len(lesson.Content.Blocks))}
			for _, block := range lesson.Content.Blocks {
				translatedLesson.ContentBlocks = append(translatedLesson.ContentBlocks, TranslatedContentBlock{SourceBlockKey: block.Key, Type: block.Type})
			}
			translatedModule.Lessons = append(translatedModule.Lessons, translatedLesson)
		}
		tree.Modules = append(tree.Modules, translatedModule)
	}
	for _, binding := range source.AssessmentBindings {
		if binding.Validate() != nil {
			return TranslationTree{}, ErrInvalidTranslation
		}
		assessment := TranslatedAssessment{AssessmentKey: binding.AssessmentKey, Questions: make([]TranslatedAssessmentQuestion, 0, len(binding.Questions))}
		for _, question := range binding.Questions {
			translatedQuestion := TranslatedAssessmentQuestion{SourceStableKey: question.StableKey, Type: question.Type, Position: question.Position, Options: make([]TranslatedAssessmentOption, 0, len(question.Options)), LeftItems: make([]TranslatedAssessmentItem, 0, len(question.LeftItems)), RightItems: make([]TranslatedAssessmentItem, 0, len(question.RightItems))}
			for _, option := range question.Options {
				translatedQuestion.Options = append(translatedQuestion.Options, TranslatedAssessmentOption{SourceStableKey: option.StableKey, Position: option.Position})
			}
			for _, item := range question.LeftItems {
				translatedQuestion.LeftItems = append(translatedQuestion.LeftItems, TranslatedAssessmentItem{SourceStableKey: item.StableKey, Position: item.Position})
			}
			for _, item := range question.RightItems {
				translatedQuestion.RightItems = append(translatedQuestion.RightItems, TranslatedAssessmentItem{SourceStableKey: item.StableKey, Position: item.Position})
			}
			assessment.Questions = append(assessment.Questions, translatedQuestion)
		}
		tree.Assessments = append(tree.Assessments, assessment)
	}
	return tree, tree.validateOptionalValues()
}

// ValidateAgainstSource rejects added, removed, reordered, or retargeted
// nodes. Assessment correctness is deliberately absent from the tree, so it
// always remains the immutable source binding's responsibility.
func (tree TranslationTree) ValidateAgainstSource(source courses.ImmutableCourseVersion) error {
	seed, err := SeedTranslationTree(source)
	if err != nil {
		return ErrInvalidTranslation
	}
	if err := tree.validateOptionalValues(); err != nil {
		return err
	}
	if len(tree.LearningObjectives) != len(seed.LearningObjectives) || len(tree.Modules) != len(seed.Modules) || len(tree.Assessments) != len(seed.Assessments) {
		return ErrInvalidTranslation
	}
	for i := range tree.Modules {
		got, want := tree.Modules[i], seed.Modules[i]
		if got.SourceStableKey != want.SourceStableKey || got.Position != want.Position || len(got.Lessons) != len(want.Lessons) {
			return ErrInvalidTranslation
		}
		for j := range got.Lessons {
			lesson, expected := got.Lessons[j], want.Lessons[j]
			if lesson.SourceStableKey != expected.SourceStableKey || lesson.Position != expected.Position || len(lesson.LearningObjectives) != len(expected.LearningObjectives) || len(lesson.ContentBlocks) != len(expected.ContentBlocks) {
				return ErrInvalidTranslation
			}
			for k := range lesson.ContentBlocks {
				if lesson.ContentBlocks[k].SourceBlockKey != expected.ContentBlocks[k].SourceBlockKey || lesson.ContentBlocks[k].Type != expected.ContentBlocks[k].Type {
					return ErrInvalidTranslation
				}
			}
		}
	}
	for i := range tree.Assessments {
		got, want := tree.Assessments[i], seed.Assessments[i]
		if got.AssessmentKey != want.AssessmentKey || len(got.Questions) != len(want.Questions) {
			return ErrInvalidTranslation
		}
		for j := range got.Questions {
			q, expected := got.Questions[j], want.Questions[j]
			if q.SourceStableKey != expected.SourceStableKey || q.Type != expected.Type || q.Position != expected.Position || !sameOptionShape(q.Options, expected.Options) || !sameItemShape(q.LeftItems, expected.LeftItems) || !sameItemShape(q.RightItems, expected.RightItems) {
				return ErrInvalidTranslation
			}
		}
	}
	return nil
}
func sameOptionShape(left, right []TranslatedAssessmentOption) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].SourceStableKey != right[i].SourceStableKey || left[i].Position != right[i].Position {
			return false
		}
	}
	return true
}
func sameItemShape(left, right []TranslatedAssessmentItem) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].SourceStableKey != right[i].SourceStableKey || left[i].Position != right[i].Position {
			return false
		}
	}
	return true
}

func sourceFromImmutable(source courses.ImmutableCourseVersion) (SourceCourseVersion, error) {
	result := SourceCourseVersion{CourseID: source.CourseVersion.CourseID, CourseVersionID: source.ID, Version: source.CourseVersion.Version, Language: source.CourseVersion.SourceLanguage}
	return result, result.Validate()
}
func cloneTree(tree TranslationTree) (TranslationTree, error) {
	encoded, err := json.Marshal(tree)
	if err != nil {
		return TranslationTree{}, ErrInvalidTranslation
	}
	var clone TranslationTree
	if err := json.Unmarshal(encoded, &clone); err != nil || clone.validateOptionalValues() != nil {
		return TranslationTree{}, ErrInvalidTranslation
	}
	return clone, nil
}
