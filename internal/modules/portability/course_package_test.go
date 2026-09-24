package portability

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
)

func TestExporterWritesPortableExactPublishedPackage(t *testing.T) {
	source := packageSource(t, "1.0.0")
	storage := packageStorage{objects: map[assets.StorageObjectID][]byte{assets.StorageObjectID(source.AssetBindings[0].StorageObjectID): []byte("image bytes")}}
	translationService, err := translations.NewService(packageSourceRepo{source}, packageTranslationRepo{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	exporter, err := NewExporter(packageSourceRepo{source}, translationService, storage, func() time.Time { return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := exporter.Write(context.Background(), source.CourseVersion.CourseID, source.CourseVersion.Version, &archive); err != nil {
		t.Fatal(err)
	}
	files := unzip(t, archive.Bytes())
	var manifest Manifest
	if err := json.Unmarshal(files[manifestPath], &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Format != Format || manifest.FormatVersion != FormatVersion || manifest.Course.Version != "1.0.0" || manifest.Course.License.Identifier != "CC-BY-4.0" {
		t.Fatalf("manifest = %#v", manifest)
	}
	if len(manifest.Assets) != 1 || manifest.Assets[0].Path != "assets/70000000-0000-4000-8000-000000000001/content" || strings.Contains(string(files[manifestPath]), source.AssetBindings[0].StorageObjectID) {
		t.Fatalf("asset metadata leaked storage identity: %s", files[manifestPath])
	}
	if got := string(files[manifest.Assets[0].Path]); got != "image bytes" {
		t.Fatalf("asset bytes = %q", got)
	}
	var assessments AssessmentsPayload
	if err := json.Unmarshal(files[assessmentsPath], &assessments); err != nil {
		t.Fatal(err)
	}
	if got := assessments.Assessments[0].Questions[0].CorrectOptionKeys; len(got) != 1 || got[0] != "option-b" {
		t.Fatalf("correct answer was not preserved: %#v", assessments)
	}
	for _, forbidden := range []string{"draft-private-id", "learner-id", source.AssetBindings[0].StorageObjectID} {
		if strings.Contains(archive.String(), forbidden) {
			t.Fatalf("package leaked %q", forbidden)
		}
	}
}

func TestExporterRejectsCorruptFrozenAsset(t *testing.T) {
	source := packageSource(t, "1.0.0")
	storage := packageStorage{objects: map[assets.StorageObjectID][]byte{assets.StorageObjectID(source.AssetBindings[0].StorageObjectID): []byte("corrupt")}}
	service, _ := translations.NewService(packageSourceRepo{source}, packageTranslationRepo{}, time.Now)
	exporter, _ := NewExporter(packageSourceRepo{source}, service, storage, time.Now)
	if err := exporter.Write(context.Background(), source.CourseVersion.CourseID, source.CourseVersion.Version, io.Discard); err != ErrExportIntegrity {
		t.Fatalf("corrupt asset error = %v", err)
	}
}

func TestExporterIsExactVersionAndSemanticallyDeterministic(t *testing.T) {
	one := packageSource(t, "1.0.0")
	two := packageSource(t, "1.1.0")
	two.CourseVersion.Title = "Version 1.1"
	storage := packageStorage{objects: map[assets.StorageObjectID][]byte{assets.StorageObjectID(one.AssetBindings[0].StorageObjectID): []byte("image bytes")}}
	service, _ := translations.NewService(packageSourceRepo{one}, packageTranslationRepo{}, time.Now)
	exporter, _ := NewExporter(packageSourceRepo{one}, service, storage, func() time.Time { return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC) })
	var first, second bytes.Buffer
	for _, dst := range []*bytes.Buffer{&first, &second} {
		if err := exporter.Write(context.Background(), one.CourseVersion.CourseID, one.CourseVersion.Version, dst); err != nil {
			t.Fatal(err)
		}
	}
	firstFiles, secondFiles := unzip(t, first.Bytes()), unzip(t, second.Bytes())
	if !bytes.Equal(firstFiles[coursePath], secondFiles[coursePath]) || !bytes.Equal(firstFiles[checksumsPath], secondFiles[checksumsPath]) {
		t.Fatal("same immutable export was not deterministic")
	}
	if strings.Contains(string(firstFiles[coursePath]), two.CourseVersion.Title) {
		t.Fatal("newer course version leaked into exact export")
	}
}

type packageSourceRepo struct {
	source courses.ImmutableCourseVersion
}

func (r packageSourceRepo) GetImmutableCourseVersion(context.Context, courses.CourseVersionID) (courses.ImmutableCourseVersion, error) {
	return r.source, nil
}
func (r packageSourceRepo) GetImmutableCourseVersionByCourseAndVersion(_ context.Context, id courses.CourseID, v courses.Version) (courses.ImmutableCourseVersion, error) {
	if id != r.source.CourseVersion.CourseID || v != r.source.CourseVersion.Version {
		return courses.ImmutableCourseVersion{}, courses.ErrNotFound
	}
	return r.source, nil
}

type packageTranslationRepo struct{}

func (packageTranslationRepo) Create(context.Context, translations.CreateInput) (translations.CourseTranslation, error) {
	return translations.CourseTranslation{}, translations.ErrTranslationNotFound
}
func (packageTranslationRepo) Get(context.Context, translations.TranslationID) (translations.CourseTranslation, error) {
	return translations.CourseTranslation{}, translations.ErrTranslationNotFound
}
func (packageTranslationRepo) GetBySourceVersionAndLanguage(context.Context, courses.CourseVersionID, courses.LanguageTag) (translations.CourseTranslation, error) {
	return translations.CourseTranslation{}, translations.ErrTranslationNotFound
}
func (packageTranslationRepo) UpdateTree(context.Context, translations.TranslationID, int64, translations.TranslationTree, time.Time) (translations.CourseTranslation, error) {
	return translations.CourseTranslation{}, translations.ErrTranslationNotFound
}
func (packageTranslationRepo) Publish(context.Context, translations.TranslationID, translations.TranslationPublication, time.Time) (translations.TranslationPublication, error) {
	return translations.TranslationPublication{}, translations.ErrTranslationNotFound
}
func (packageTranslationRepo) GetLatestPublication(context.Context, courses.CourseVersionID, courses.LanguageTag) (translations.TranslationPublication, error) {
	return translations.TranslationPublication{}, translations.ErrTranslationNotFound
}
func (packageTranslationRepo) ListLanguages(context.Context, courses.CourseVersionID) ([]courses.LanguageTag, error) {
	return []courses.LanguageTag{}, nil
}

type packageStorage struct {
	objects map[assets.StorageObjectID][]byte
}

func (s packageStorage) Put(context.Context, io.Reader) (assets.StoredBinary, error) {
	return assets.StoredBinary{}, assets.ErrBinaryStorage
}
func (s packageStorage) Open(_ context.Context, id assets.StorageObjectID) (io.ReadCloser, error) {
	b, ok := s.objects[id]
	if !ok {
		return nil, assets.ErrStorageObjectMissing
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (packageStorage) DiscardUncommitted(context.Context, assets.StorageObjectID) error { return nil }

func packageSource(t *testing.T, versionText string) courses.ImmutableCourseVersion {
	t.Helper()
	version, err := courses.ParseVersion(versionText)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("image bytes")
	sum := sha256.Sum256(raw)
	return courses.ImmutableCourseVersion{ID: "20000000-0000-4000-8000-000000000001", CourseVersion: courses.CourseVersionInput{CourseID: "10000000-0000-4000-8000-000000000001", Version: version, Status: courses.CourseVersionPublished, Title: "Portable course", Description: "Portable description", LearningObjectives: []string{"Objective"}, SourceLanguage: "en", Changelog: "Initial release", License: courses.ContentLicense{Kind: courses.ContentLicenseStandard, Identifier: "CC-BY-4.0", DisplayName: "CC BY 4.0", URL: "https://creativecommons.org/licenses/by/4.0/"}, Attribution: []courses.ContributorSnapshot{{DisplayName: "Public Author", Role: courses.ContributorAuthor, Order: 0}}, PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, Modules: []courses.ImmutableCourseVersionModule{{StableKey: "module-one", Title: "Module", Description: "", Position: 0, Lessons: []courses.ImmutableCourseVersionLesson{{StableKey: "lesson-one", Title: "Lesson", Description: "", LearningObjectives: []string{"Lesson objective"}, Position: 0, PrerequisiteStableKeys: []string{}, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "image", Type: courses.BlockImage, Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: "70000000-0000-4000-8000-000000000001"}, AltText: "Image", Caption: "Caption"}}, {Key: "check", Type: courses.BlockKnowledgeCheck, Payload: courses.KnowledgeCheckBlockPayload{AssessmentKey: "60000000-0000-4000-8000-000000000001"}}}}}}}}, AssetBindings: []courses.PublishedAssetBinding{{AssetKey: "70000000-0000-4000-8000-000000000001", StorageObjectID: "80000000-0000-4000-8000-000000000001", OriginalFilename: "image.png", MediaType: "image/png", ByteSize: int64(len(raw)), SHA256Digest: hex.EncodeToString(sum[:])}}, AssessmentBindings: []courses.PublishedAssessmentBinding{{AssessmentKey: "60000000-0000-4000-8000-000000000001", Questions: []courses.PublishedAssessmentQuestion{{StableKey: "question-one", Type: courses.PublishedQuestionSingleChoice, Prompt: "Prompt", Position: 0, Options: []courses.PublishedAssessmentOption{{StableKey: "option-a", Text: "A", Position: 0}, {StableKey: "option-b", Text: "B", Position: 1}}, CorrectOptionKeys: []string{"option-b"}}}}}}
}
func unzip(t *testing.T, b []byte) map[string][]byte {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range r.File {
		in, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(in)
		if err := in.Close(); err != nil {
			t.Fatal(err)
		}
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = body
	}
	return out
}
