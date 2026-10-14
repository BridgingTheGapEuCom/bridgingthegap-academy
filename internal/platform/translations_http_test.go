package platform

import (
	"encoding/json"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
)

func TestTranslationWorkspaceResponseUsesThePublishedContract(t *testing.T) {
	version, err := courses.ParseVersion("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	view := translations.TranslationWorkspaceView{
		TranslationID: "10000000-0000-4000-8000-000000000001",
		Revision:      1,
		Lifecycle:     translations.TranslationDraft,
		Source: translations.SourceCourseVersion{
			CourseID:        "20000000-0000-4000-8000-000000000001",
			CourseVersionID: "30000000-0000-4000-8000-000000000001",
			Version:         version,
			Language:        "en",
		},
		TargetLanguage: "es",
		Modules: []translations.TranslationModuleView{{
			SourceStableKey: "module-one",
			Lessons: []translations.TranslationLessonView{{
				SourceStableKey: "lesson-one",
				Blocks: []translations.TranslationContentBlockView{{
					SourceBlockKey: "block-one", Type: courses.BlockText, Fields: map[string]translations.TextFieldView{}, AssetKeys: []string{}, TableHeaders: []string{}, TableRows: [][]string{},
				}},
			}},
		}},
		Assessments: []translations.TranslationAssessmentView{},
	}

	body, err := json.Marshal(translationWorkspace(view))
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	source := response["source"].(map[string]any)
	if source["version"] != "1.2.3" {
		t.Fatalf("source version must be a SemVer string, got %#v", source["version"])
	}
	block := response["modules"].([]any)[0].(map[string]any)["lessons"].([]any)[0].(map[string]any)["blocks"].([]any)[0].(map[string]any)
	if block["sourceBlockKey"] != "block-one" || block["position"] != float64(0) {
		t.Fatalf("block identity must use the OpenAPI field names: %#v", block)
	}
}
