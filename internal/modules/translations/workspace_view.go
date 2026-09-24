package translations

import (
	"context"
	"errors"
	"strings"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

var ErrSourceIntegrity = errors.New("translation source integrity mismatch")

type FieldState string

const (
	FieldUntranslated FieldState = "UNTRANSLATED"
	FieldTranslated   FieldState = "TRANSLATED"
)

type Completeness struct {
	TotalTranslatableFields int
	TranslatedFields        int
	UntranslatedFields      int
	Complete                bool
}
type TextFieldView struct {
	Source     string
	Translated *string
	State      FieldState
}
type TranslationWorkspaceView struct {
	TranslationID  TranslationID
	Revision       int64
	Lifecycle      TranslationStatus
	Source         SourceCourseVersion
	TargetLanguage courses.LanguageTag
	CreatorUserID  string
	Course         TranslationCourseView
	Modules        []TranslationModuleView
	Assessments    []TranslationAssessmentView
	Completeness   Completeness
}
type TranslationCourseView struct {
	Title              TextFieldView
	Description        TextFieldView
	LearningObjectives []TextFieldView
}
type TranslationModuleView struct {
	SourceStableKey string
	Position        int
	Title           TextFieldView
	Description     TextFieldView
	Lessons         []TranslationLessonView
	Completeness    Completeness
}
type TranslationLessonView struct {
	SourceStableKey        string
	Position               int
	PrerequisiteStableKeys []string
	Title                  TextFieldView
	Description            TextFieldView
	LearningObjectives     []TextFieldView
	Blocks                 []TranslationContentBlockView
	Completeness           Completeness
}

// TranslationContentBlockView is type-aware source context. Asset keys, code,
// table cells, and assessment references are read-only context; Fields are the
// only values which participate in translation completeness.
type TranslationContentBlockView struct {
	SourceBlockKey string
	Type           courses.BlockType
	Position       int
	Fields         map[string]TextFieldView
	AssetKeys      []string
	Code           string
	TableHeaders   []string
	TableRows      [][]string
	AssessmentKey  string
}
type TranslationAssessmentView struct {
	AssessmentKey string
	Questions     []TranslationQuestionView
	Completeness  Completeness
}
type TranslationQuestionView struct {
	SourceStableKey string
	Type            courses.PublishedAssessmentQuestionType
	Position        int
	Prompt          TextFieldView
	Options         []TranslationOptionView
	LeftItems       []TranslationItemView
	RightItems      []TranslationItemView
}
type TranslationOptionView struct {
	SourceStableKey string
	Position        int
	Text            TextFieldView
}
type TranslationItemView struct {
	SourceStableKey string
	Position        int
	Text            TextFieldView
}

// WorkspaceQueryService constructs an authenticated translator view from the
// persisted workspace and precisely its bound immutable source version. It
// never uses Courses latest reads or Authoring state and mutates neither input.
type WorkspaceQueryService struct {
	translations *Service
	authorizer   Authorizer
}

func NewWorkspaceQueryService(translations *Service, authorizer Authorizer) (*WorkspaceQueryService, error) {
	if translations == nil || authorizer == nil {
		return nil, ErrInvalidTranslation
	}
	return &WorkspaceQueryService{translations: translations, authorizer: authorizer}, nil
}
func (s *WorkspaceQueryService) View(ctx context.Context, actorID string, id TranslationID) (TranslationWorkspaceView, error) {
	if s == nil || s.translations == nil || s.authorizer == nil {
		return TranslationWorkspaceView{}, ErrInvalidTranslation
	}
	translation, err := s.translations.Get(ctx, id)
	if err != nil {
		return TranslationWorkspaceView{}, err
	}
	if err := s.authorizer.Authorize(ctx, actorID, CapabilityRead, SourceCourseResource(translation.Source)); err != nil {
		return TranslationWorkspaceView{}, err
	}
	source, err := s.translations.source.GetImmutableCourseVersion(ctx, translation.Source.CourseVersionID)
	if err != nil {
		return TranslationWorkspaceView{}, err
	}
	binding, bindingErr := sourceFromImmutable(source)
	if bindingErr != nil || binding != translation.Source || translation.Tree.ValidateAgainstSource(source) != nil {
		return TranslationWorkspaceView{}, ErrSourceIntegrity
	}
	return buildWorkspaceView(translation, source)
}

func buildWorkspaceView(translation CourseTranslation, source courses.ImmutableCourseVersion) (TranslationWorkspaceView, error) {
	if translation.Tree.ValidateAgainstSource(source) != nil {
		return TranslationWorkspaceView{}, ErrSourceIntegrity
	}
	view := TranslationWorkspaceView{TranslationID: translation.ID, Revision: translation.Revision, Lifecycle: translation.Status, Source: translation.Source, TargetLanguage: translation.TargetLanguage, CreatorUserID: translation.CreatorUserID, Course: TranslationCourseView{Title: field(source.CourseVersion.Title, translation.Tree.Title), Description: field(source.CourseVersion.Description, translation.Tree.Description), LearningObjectives: fields(source.CourseVersion.LearningObjectives, translation.Tree.LearningObjectives)}, Modules: make([]TranslationModuleView, 0, len(source.Modules)), Assessments: make([]TranslationAssessmentView, 0, len(source.AssessmentBindings))}
	for i, module := range source.Modules {
		translated := translation.Tree.Modules[i]
		moduleView := TranslationModuleView{SourceStableKey: module.StableKey, Position: module.Position, Title: field(module.Title, translated.Title), Description: field(module.Description, translated.Description), Lessons: make([]TranslationLessonView, 0, len(module.Lessons))}
		for j, lesson := range module.Lessons {
			tLesson := translated.Lessons[j]
			lessonView := TranslationLessonView{SourceStableKey: lesson.StableKey, Position: lesson.Position, PrerequisiteStableKeys: append([]string(nil), lesson.PrerequisiteStableKeys...), Title: field(lesson.Title, tLesson.Title), Description: field(lesson.Description, tLesson.Description), LearningObjectives: fields(lesson.LearningObjectives, tLesson.LearningObjectives), Blocks: buildBlocks(lesson.Content, tLesson.ContentBlocks)}
			lessonView.Completeness = complete(lessonFields(lessonView))
			moduleView.Lessons = append(moduleView.Lessons, lessonView)
		}
		moduleView.Completeness = combine(moduleFields(moduleView))
		view.Modules = append(view.Modules, moduleView)
	}
	for i, binding := range source.AssessmentBindings {
		assessment := translation.Tree.Assessments[i]
		assessmentView := TranslationAssessmentView{AssessmentKey: binding.AssessmentKey, Questions: make([]TranslationQuestionView, 0, len(binding.Questions))}
		for j, question := range binding.Questions {
			translated := assessment.Questions[j]
			questionView := TranslationQuestionView{SourceStableKey: question.StableKey, Type: question.Type, Position: question.Position, Prompt: field(question.Prompt, translated.Prompt), Options: make([]TranslationOptionView, 0, len(question.Options)), LeftItems: make([]TranslationItemView, 0, len(question.LeftItems)), RightItems: make([]TranslationItemView, 0, len(question.RightItems))}
			for k, option := range question.Options {
				questionView.Options = append(questionView.Options, TranslationOptionView{SourceStableKey: option.StableKey, Position: option.Position, Text: field(option.Text, translated.Options[k].Text)})
			}
			for k, item := range question.LeftItems {
				questionView.LeftItems = append(questionView.LeftItems, TranslationItemView{SourceStableKey: item.StableKey, Position: item.Position, Text: field(item.Text, translated.LeftItems[k].Text)})
			}
			for k, item := range question.RightItems {
				questionView.RightItems = append(questionView.RightItems, TranslationItemView{SourceStableKey: item.StableKey, Position: item.Position, Text: field(item.Text, translated.RightItems[k].Text)})
			}
			assessmentView.Questions = append(assessmentView.Questions, questionView)
		}
		assessmentView.Completeness = complete(assessmentFields(assessmentView))
		view.Assessments = append(view.Assessments, assessmentView)
	}
	all := courseFields(view.Course)
	for _, module := range view.Modules {
		all = append(all, moduleFields(module)...)
	}
	for _, assessment := range view.Assessments {
		all = append(all, assessmentFields(assessment)...)
	}
	view.Completeness = complete(all)
	return view, nil
}
func field(source string, translated *string) TextFieldView {
	state := FieldUntranslated
	if translated != nil {
		state = FieldTranslated
	}
	return TextFieldView{Source: source, Translated: translated, State: state}
}
func fields(source []string, translated []*string) []TextFieldView {
	result := make([]TextFieldView, len(source))
	for i := range source {
		result[i] = field(source[i], translated[i])
	}
	return result
}
func buildBlocks(content courses.LessonContent, translated []TranslatedContentBlock) []TranslationContentBlockView {
	result := make([]TranslationContentBlockView, 0, len(content.Blocks))
	for i, block := range content.Blocks {
		value := TranslationContentBlockView{SourceBlockKey: block.Key, Type: block.Type, Position: i, Fields: map[string]TextFieldView{}}
		t := translated[i]
		switch payload := block.Payload.(type) {
		case courses.TextBlockPayload:
			value.Fields["text"] = field(richText(payload.Content), t.Text)
		case courses.HeadingBlockPayload:
			value.Fields["text"] = field(inlines(payload.Content), t.Text)
		case courses.ImageBlockPayload:
			value.AssetKeys = []string{payload.Asset.AssetKey}
			value.Fields["altText"] = field(payload.AltText, t.AltText)
			value.Fields["caption"] = field(payload.Caption, t.Caption)
		case courses.VideoBlockPayload:
			value.AssetKeys = []string{payload.Asset.AssetKey, payload.CaptionsAsset.AssetKey}
			if payload.TranscriptAsset != nil {
				value.AssetKeys = append(value.AssetKeys, payload.TranscriptAsset.AssetKey)
			}
			value.Fields["title"] = field(payload.Title, t.Title)
			value.Fields["transcript"] = field(payload.Transcript, t.Transcript)
		case courses.AudioBlockPayload:
			value.AssetKeys = []string{payload.Asset.AssetKey}
			if payload.TranscriptAsset != nil {
				value.AssetKeys = append(value.AssetKeys, payload.TranscriptAsset.AssetKey)
			}
			value.Fields["title"] = field(payload.Title, t.Title)
			value.Fields["transcript"] = field(payload.Transcript, t.Transcript)
		case courses.CodeBlockPayload:
			value.Code = payload.Code
			if payload.Title != "" {
				value.Fields["title"] = field(payload.Title, t.Title)
			}
		case courses.QuoteBlockPayload:
			value.Fields["text"] = field(payload.Text, t.Text)
			value.Fields["attribution"] = field(payload.Attribution, t.Attribution)
		case courses.CalloutBlockPayload:
			value.Fields["title"] = field(payload.Title, t.Title)
			value.Fields["text"] = field(richText(payload.Content), t.Text)
		case courses.TableBlockPayload:
			value.Fields["caption"] = field(payload.Caption, t.Caption)
			value.TableHeaders = append([]string(nil), payload.Headers...)
			value.TableRows = cloneRows(payload.Rows)
		case courses.DownloadBlockPayload:
			value.AssetKeys = []string{payload.Asset.AssetKey}
			value.Fields["label"] = field(payload.Label, t.Label)
			value.Fields["description"] = field(payload.Description, t.Description)
		case courses.KnowledgeCheckBlockPayload:
			value.AssessmentKey = payload.AssessmentKey
		}
		result = append(result, value)
	}
	return result
}
func richText(value courses.RichText) string {
	parts := make([]string, 0)
	for _, node := range value.Nodes {
		for _, inline := range node.Content {
			parts = append(parts, inline.Text)
		}
		for _, item := range node.Items {
			for _, inline := range item {
				parts = append(parts, inline.Text)
			}
		}
	}
	return strings.Join(parts, "\n")
}
func inlines(value []courses.RichTextInline) string {
	parts := make([]string, 0, len(value))
	for _, inline := range value {
		parts = append(parts, inline.Text)
	}
	return strings.Join(parts, "")
}
func cloneRows(rows [][]string) [][]string {
	result := make([][]string, len(rows))
	for i := range rows {
		result[i] = append([]string(nil), rows[i]...)
	}
	return result
}
func courseFields(course TranslationCourseView) []TextFieldView {
	return append(append([]TextFieldView{course.Title, course.Description}, course.LearningObjectives...), nil...)
}
func moduleFields(module TranslationModuleView) []TextFieldView {
	result := []TextFieldView{module.Title, module.Description}
	for _, lesson := range module.Lessons {
		result = append(result, lessonFields(lesson)...)
	}
	return result
}
func lessonFields(lesson TranslationLessonView) []TextFieldView {
	result := append([]TextFieldView{lesson.Title, lesson.Description}, lesson.LearningObjectives...)
	for _, block := range lesson.Blocks {
		for _, value := range block.Fields {
			result = append(result, value)
		}
	}
	return result
}
func assessmentFields(assessment TranslationAssessmentView) []TextFieldView {
	var result []TextFieldView
	for _, question := range assessment.Questions {
		result = append(result, question.Prompt)
		for _, option := range question.Options {
			result = append(result, option.Text)
		}
		for _, item := range question.LeftItems {
			result = append(result, item.Text)
		}
		for _, item := range question.RightItems {
			result = append(result, item.Text)
		}
	}
	return result
}
func complete(fields []TextFieldView) Completeness {
	result := Completeness{TotalTranslatableFields: len(fields), Complete: true}
	for _, field := range fields {
		if field.State == FieldTranslated {
			result.TranslatedFields++
		} else {
			result.UntranslatedFields++
			result.Complete = false
		}
	}
	return result
}
func combine(fields []TextFieldView) Completeness { return complete(fields) }
