package translations

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

const (
	testCourseID = courses.CourseID("10000000-0000-4000-8000-000000000001")
	testSourceID = courses.CourseVersionID("20000000-0000-4000-8000-000000000001")
	testCreator  = "30000000-0000-4000-8000-000000000001"
)

func TestSeedTranslationTreePreservesSourceShapeAndLeavesTextUntranslated(t *testing.T) {
	source := translationSource(t, testSourceID, testCourseID, "1.0.0")
	tree, err := SeedTranslationTree(source)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Title != nil || tree.Modules[0].Title != nil || tree.Modules[0].Lessons[0].ContentBlocks[0].Text != nil {
		t.Fatalf("source prose was copied as translation: %#v", tree)
	}
	if tree.Modules[0].SourceStableKey != "module-one" || tree.Modules[0].Lessons[0].SourceStableKey != "lesson-one" || tree.Modules[0].Lessons[0].ContentBlocks[0].SourceBlockKey != "paragraph-one" {
		t.Fatalf("stable source keys not retained: %#v", tree)
	}
	if tree.Assessments[0].Questions[0].Options[1].SourceStableKey != "option-b" || tree.Assessments[0].Questions[0].Type != courses.PublishedQuestionSingleChoice {
		t.Fatalf("assessment structural or correctness boundary lost: %#v", tree.Assessments)
	}
	empty := ""
	tree.Modules[0].Title = &empty
	if err := tree.ValidateAgainstSource(source); err != nil {
		t.Fatalf("intentional empty translation rejected: %v", err)
	}
	tree.Modules[0].Lessons[0].ContentBlocks[0].SourceBlockKey = "invented"
	if err := tree.ValidateAgainstSource(source); !errors.Is(err, ErrInvalidTranslation) {
		t.Fatalf("invented source key error = %v", err)
	}
}

func TestTranslationServiceUsesExactSourceAndCAS(t *testing.T) {
	source := translationSource(t, testSourceID, testCourseID, "1.0.0")
	repo := &translationMemory{next: "40000000-0000-4000-8000-000000000001"}
	service, err := NewService(sourceMemory{source}, repo, func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Create(context.Background(), testSourceID, "es", testCreator)
	if err != nil {
		t.Fatal(err)
	}
	if created.Source.CourseVersionID != testSourceID || created.TargetLanguage != "es" || created.Revision != 1 {
		t.Fatalf("unexpected translation %#v", created)
	}
	if _, err := service.Create(context.Background(), testSourceID, "en", testCreator); !errors.Is(err, ErrInvalidTranslation) {
		t.Fatalf("source and target language were accepted: %v", err)
	}
	value := "Título"
	next := created.Tree
	next.Title = &value
	translatedPrompt := "Pregunta"
	next.Assessments[0].Questions[0].Prompt = &translatedPrompt
	updated, err := service.Update(context.Background(), created.ID, 1, next)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Tree.Title == nil || *updated.Tree.Title != value {
		t.Fatalf("CAS update failed %#v", updated)
	}
	if _, err := service.Update(context.Background(), created.ID, 1, next); !errors.Is(err, ErrRevisionMismatch) {
		t.Fatalf("stale revision error = %v", err)
	}
	publication, err := service.Publish(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if publication.Revision != 2 || publication.Source.Version.String() != "1.0.0" {
		t.Fatalf("publication source binding changed %#v", publication)
	}
	changed := "Changed after publication"
	next = updated.Tree
	next.Description = &changed
	if _, err := service.Update(context.Background(), created.ID, 2, next); err != nil {
		t.Fatal(err)
	}
	if publication.Tree.Description != nil {
		t.Fatalf("published snapshot mutated through draft update: %#v", publication.Tree)
	}
}

func TestTranslationCapabilitiesUseSourceCourseAttributionOnly(t *testing.T) {
	source := translationSource(t, testSourceID, testCourseID, "1.0.0")
	resource := SourceCourseResource(mustSource(t, source))
	author := "30000000-0000-4000-8000-000000000002"
	maintainer := "30000000-0000-4000-8000-000000000003"
	creatorOnly := "30000000-0000-4000-8000-000000000004"
	globalAdminOnly := "30000000-0000-4000-8000-000000000005"
	policy := NewAuthorizationService(authorityMemory{courses.CourseVersion{CourseID: testCourseID, Status: courses.CourseVersionPublished, Attribution: []courses.ContributorSnapshot{{UserID: author, Role: courses.ContributorAuthor}, {UserID: maintainer, Role: courses.ContributorMaintainer}}}})
	for _, capability := range []Capability{CapabilityRead, CapabilityCreate, CapabilityEdit, CapabilityPublish} {
		if err := policy.Authorize(context.Background(), author, capability, resource); err != nil {
			t.Fatalf("AUTHOR %s: %v", capability, err)
		}
		if err := policy.Authorize(context.Background(), maintainer, capability, resource); err != nil {
			t.Fatalf("MAINTAINER %s: %v", capability, err)
		}
	}
	for _, actor := range []string{creatorOnly, globalAdminOnly} {
		if err := policy.Authorize(context.Background(), actor, CapabilityRead, resource); !errors.Is(err, ErrAuthorizationDenied) {
			t.Fatalf("non-attributed actor %s received access: %v", actor, err)
		}
	}
	foreign := resource
	foreign.source.CourseID = "10000000-0000-4000-8000-000000000099"
	if err := policy.Authorize(context.Background(), author, CapabilityEdit, foreign); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("Course A authority reached Course B resource: %v", err)
	}
	// Application operations load the Translation's persisted source before
	// authorizing; a caller cannot substitute a different Course resource.
	repo := &translationMemory{next: "40000000-0000-4000-8000-000000000010"}
	base, _ := NewService(sourceMemory{source}, repo, time.Now)
	app, _ := NewApplicationService(base, policy)
	created, err := app.Create(context.Background(), author, testSourceID, "es")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Workspace(context.Background(), creatorOnly, created.ID); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("creator provenance became permanent authority: %v", err)
	}
	if _, err := app.Update(context.Background(), globalAdminOnly, created.ID, created.Revision, created.Tree); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("global administrator bypass: %v", err)
	}
}

func TestWorkspaceViewPairsExactSourceAndOverridesWithoutCorrectnessLeak(t *testing.T) {
	source := viewSource(t)
	tree, err := SeedTranslationTree(source)
	if err != nil {
		t.Fatal(err)
	}
	title := "Título"
	tree.Title = &title
	empty := ""
	tree.Modules[0].Lessons[0].ContentBlocks[1].Text = &empty // HEADING: intentional empty is translated.
	prompt := "Pregunta"
	tree.Assessments[0].Questions[0].Prompt = &prompt
	option := "Opción A"
	tree.Assessments[0].Questions[0].Options[0].Text = &option
	translation := CourseTranslation{ID: "40000000-0000-4000-8000-000000000011", Source: mustSource(t, source), TargetLanguage: "es", CreatorUserID: testCreator, Status: TranslationDraft, Revision: 2, Tree: tree, CreatedAt: time.Now().Add(-time.Minute), UpdatedAt: time.Now()}
	view, err := buildWorkspaceView(translation, source)
	if err != nil {
		t.Fatal(err)
	}
	if view.Course.Title.Source != "Source title" || view.Course.Title.Translated == nil || *view.Course.Title.Translated != title || view.Course.Title.State != FieldTranslated {
		t.Fatalf("course source/translation pair %#v", view.Course.Title)
	}
	heading := view.Modules[0].Lessons[0].Blocks[1].Fields["text"]
	if heading.Translated == nil || *heading.Translated != "" || heading.State != FieldTranslated {
		t.Fatalf("empty override was collapsed: %#v", heading)
	}
	if view.Modules[0].Lessons[0].Blocks[2].Code != "fmt.Println(\"source\")" || len(view.Modules[0].Lessons[0].Blocks[6].Fields) != 0 || view.Modules[0].Lessons[0].Blocks[7].AssessmentKey == "" {
		t.Fatalf("non-translatable block context incorrect: %#v", view.Modules[0].Lessons[0].Blocks)
	}
	if view.Assessments[0].Questions[0].Prompt.Translated == nil || *view.Assessments[0].Questions[0].Prompt.Translated != prompt || view.Assessments[0].Questions[0].Options[0].Text.Translated == nil {
		t.Fatalf("assessment translated values omitted: %#v", view.Assessments)
	}
	encoded, _ := json.Marshal(view)
	for _, secret := range []string{"CorrectOptionKeys", "correctOptionKeys", "CorrectPairs", "correctPairs"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("translator view leaked grading data %q: %s", secret, encoded)
		}
	}
	if view.Completeness.TotalTranslatableFields == 0 || view.Completeness.TranslatedFields < 4 || view.Completeness.UntranslatedFields == 0 || view.Completeness.Complete {
		t.Fatalf("unexpected completeness %#v", view.Completeness)
	}
	// A zero-field tree is deterministically complete rather than dividing by zero.
	if zero := complete(nil); !zero.Complete || zero.TotalTranslatableFields != 0 || zero.TranslatedFields != 0 || zero.UntranslatedFields != 0 {
		t.Fatalf("zero completeness %#v", zero)
	}
}

func TestWorkspaceQueryUsesBoundVersionAndFailsOnIntegrityMismatch(t *testing.T) {
	v1 := translationSource(t, testSourceID, testCourseID, "1.0.0")
	v1.CourseVersion.Title = "Version one"
	tree, _ := SeedTranslationTree(v1)
	translation := CourseTranslation{ID: "40000000-0000-4000-8000-000000000012", Source: mustSource(t, v1), TargetLanguage: "es", CreatorUserID: testCreator, Status: TranslationDraft, Revision: 1, Tree: tree, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	repo := &translationMemory{item: translation}
	base, _ := NewService(sourceMemory{v1}, repo, time.Now)
	author := testCreator
	policy := NewAuthorizationService(authorityMemory{courses.CourseVersion{CourseID: testCourseID, Status: courses.CourseVersionPublished, Attribution: []courses.ContributorSnapshot{{UserID: author, Role: courses.ContributorAuthor}}}})
	query, _ := NewWorkspaceQueryService(base, policy)
	view, err := query.View(context.Background(), author, translation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Course.Title.Source != "Version one" {
		t.Fatalf("query did not use exact bound source: %#v", view.Source)
	}
	repo.item.Tree.Modules[0].Lessons[0].ContentBlocks[0].SourceBlockKey = "invented"
	if _, err := query.View(context.Background(), author, translation.ID); !errors.Is(err, ErrSourceIntegrity) {
		t.Fatalf("integrity mismatch error = %v", err)
	}
}

type authorityMemory []courses.CourseVersion

func (a authorityMemory) ListPublishedCourseVersions(context.Context) ([]courses.CourseVersion, error) {
	return a, nil
}
func mustSource(t *testing.T, source courses.ImmutableCourseVersion) SourceCourseVersion {
	t.Helper()
	binding, err := sourceFromImmutable(source)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func viewSource(t *testing.T) courses.ImmutableCourseVersion {
	source := translationSource(t, testSourceID, testCourseID, "1.0.0")
	asset := "70000000-0000-4000-8000-000000000001"
	assessment := "60000000-0000-4000-8000-000000000001"
	source.Modules[0].Lessons[0].Content.Blocks = []courses.Block{
		{Key: "text", Type: courses.BlockText, Payload: courses.TextBlockPayload{Content: rich("Source text")}},
		{Key: "heading", Type: courses.BlockHeading, Payload: courses.HeadingBlockPayload{Level: 2, Content: []courses.RichTextInline{{Type: "text", Text: "Source heading"}}}},
		{Key: "code", Type: courses.BlockCode, Payload: courses.CodeBlockPayload{Code: "fmt.Println(\"source\")", Title: "Code title"}},
		{Key: "quote", Type: courses.BlockQuote, Payload: courses.QuoteBlockPayload{Text: "Source quote", Attribution: "Author"}},
		{Key: "callout", Type: courses.BlockCallout, Payload: courses.CalloutBlockPayload{Kind: "INFO", Title: "Note", Content: rich("Callout text")}},
		{Key: "image", Type: courses.BlockImage, Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: asset}, AltText: "Source alt", Caption: "Source caption"}},
		{Key: "divider", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}},
		{Key: "check", Type: courses.BlockKnowledgeCheck, Payload: courses.KnowledgeCheckBlockPayload{AssessmentKey: assessment}},
	}
	return source
}
func rich(value string) courses.RichText {
	return courses.RichText{Nodes: []courses.RichTextNode{{Type: "paragraph", Content: []courses.RichTextInline{{Type: "text", Text: value}}}}}
}

type sourceMemory struct {
	source courses.ImmutableCourseVersion
}

func (m sourceMemory) GetImmutableCourseVersion(_ context.Context, id courses.CourseVersionID) (courses.ImmutableCourseVersion, error) {
	if id != m.source.ID {
		return courses.ImmutableCourseVersion{}, courses.ErrNotFound
	}
	return m.source, nil
}

type translationMemory struct {
	item        CourseTranslation
	publication TranslationPublication
	next        string
}

func (m *translationMemory) Create(_ context.Context, in CreateInput) (CourseTranslation, error) {
	if m.item.ID != "" {
		return CourseTranslation{}, ErrTranslationConflict
	}
	m.item = CourseTranslation{ID: TranslationID(m.next), Source: in.Source, TargetLanguage: in.TargetLanguage, CreatorUserID: in.CreatorUserID, Status: TranslationDraft, Revision: 1, Tree: in.Tree, CreatedAt: in.CreatedAt, UpdatedAt: in.CreatedAt}
	return m.item, nil
}
func (m *translationMemory) Get(_ context.Context, id TranslationID) (CourseTranslation, error) {
	if m.item.ID != id {
		return CourseTranslation{}, ErrTranslationNotFound
	}
	return m.item, nil
}
func (m *translationMemory) GetBySourceVersionAndLanguage(_ context.Context, id courses.CourseVersionID, l courses.LanguageTag) (CourseTranslation, error) {
	if m.item.Source.CourseVersionID != id || m.item.TargetLanguage != l {
		return CourseTranslation{}, ErrTranslationNotFound
	}
	return m.item, nil
}
func (m *translationMemory) UpdateTree(_ context.Context, id TranslationID, e int64, tree TranslationTree, at time.Time) (CourseTranslation, error) {
	if m.item.ID != id {
		return CourseTranslation{}, ErrTranslationNotFound
	}
	if m.item.Revision != e {
		return CourseTranslation{}, ErrRevisionMismatch
	}
	m.item.Tree = tree
	m.item.Revision++
	m.item.UpdatedAt = at
	return m.item, nil
}
func (m *translationMemory) Publish(_ context.Context, id TranslationID, p TranslationPublication, _ time.Time) (TranslationPublication, error) {
	if m.item.ID != id || m.item.Revision != p.Revision {
		return TranslationPublication{}, ErrRevisionMismatch
	}
	p.ID = "50000000-0000-4000-8000-000000000001"
	m.publication = p
	m.item.Status = TranslationPublished
	return p, nil
}
func (m *translationMemory) GetLatestPublication(_ context.Context, _ courses.CourseVersionID, _ courses.LanguageTag) (TranslationPublication, error) {
	if m.publication.ID == "" {
		return TranslationPublication{}, ErrTranslationNotFound
	}
	return m.publication, nil
}
func (m *translationMemory) ListLanguages(_ context.Context, _ courses.CourseVersionID) ([]courses.LanguageTag, error) {
	return []courses.LanguageTag{"es"}, nil
}

func translationSource(t *testing.T, id courses.CourseVersionID, courseID courses.CourseID, version string) courses.ImmutableCourseVersion {
	t.Helper()
	parsed, err := courses.ParseVersion(version)
	if err != nil {
		t.Fatal(err)
	}
	return courses.ImmutableCourseVersion{ID: id, CourseVersion: courses.CourseVersionInput{CourseID: courseID, Version: parsed, Status: courses.CourseVersionPublished, Title: "Source title", Description: "Source description", LearningObjectives: []string{"Objective"}, SourceLanguage: "en"}, Modules: []courses.ImmutableCourseVersionModule{{StableKey: "module-one", Position: 0, Lessons: []courses.ImmutableCourseVersionLesson{{StableKey: "lesson-one", Position: 0, LearningObjectives: []string{"Lesson objective"}, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "paragraph-one", Type: courses.BlockText, Payload: courses.TextBlockPayload{Content: courses.RichText{Nodes: []courses.RichTextNode{{Type: "paragraph", Content: []courses.RichTextInline{{Type: "text", Text: "Source prose"}}}}}}}}}}}}}, AssessmentBindings: []courses.PublishedAssessmentBinding{{AssessmentKey: "60000000-0000-4000-8000-000000000001", Questions: []courses.PublishedAssessmentQuestion{{StableKey: "question-one", Type: courses.PublishedQuestionSingleChoice, Prompt: "Prompt", Position: 0, Options: []courses.PublishedAssessmentOption{{StableKey: "option-a", Text: "A", Position: 0}, {StableKey: "option-b", Text: "B", Position: 1}}, CorrectOptionKeys: []string{"option-b"}}}}}}
}
