//go:build integration

package platform

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	assetslocal "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/localstorage"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/portability"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
	translationspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPortabilityImport(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	storage, err := assetslocal.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	repository, err := portability.NewPostgresImportRepository(pool, storage, 1024)
	if err != nil {
		t.Fatal(err)
	}
	importer, err := portability.NewImporter(repository, func() time.Time { return time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}

	pkg := importedPackage(t, "1.0.0", "20000000-0000-4000-8000-000000000901", "Imported course")
	first, err := importer.Import(ctx, pkg)
	if err != nil {
		t.Fatal(err)
	}
	if first.Disposition != portability.ImportCreated || first.CourseID == "10000000-0000-4000-8000-000000000001" || first.CourseVersionID == "20000000-0000-4000-8000-000000000901" || len(first.TranslationLanguages) != 1 || first.TranslationLanguages[0] != "es" {
		t.Fatalf("first import = %#v", first)
	}
	replayed, err := importer.Import(ctx, pkg)
	if err != nil || replayed.Disposition != portability.ImportReplayed || replayed.ImportID != first.ImportID || replayed.CourseID != first.CourseID || replayed.CourseVersionID != first.CourseVersionID {
		t.Fatalf("replayed import = %#v, %v", replayed, err)
	}

	courseRepo := coursespostgres.New(pool)
	immutable, err := courseRepo.GetImmutableCourseVersion(ctx, first.CourseVersionID)
	if err != nil || immutable.Publication.Origin != courses.PublicationOriginImported || immutable.Publication.Imported == nil || immutable.Publication.Imported.ImportID != first.ImportID || immutable.Modules[0].StableKey != "module-one" || len(immutable.Modules[0].Lessons[0].PrerequisiteStableKeys) != 0 {
		t.Fatalf("imported immutable CourseVersion = %#v, %v", immutable, err)
	}
	reader := courses.NewPublishedReadService(courseRepo)
	learner, err := reader.Exact(ctx, first.CourseID, first.Version)
	if err != nil || learner.Title != "Imported course" || learner.Assessments[0].Questions[0].Prompt != "Prompt" || learner.Assessments[0].Questions[0].Options[1].Text != "B" {
		t.Fatalf("imported learner course = %#v, %v", learner, err)
	}
	binding, err := courseRepo.GetPublishedAssetBindingByCourseAndVersionAndAssetKey(ctx, first.CourseID, first.Version, "70000000-0000-4000-8000-000000000001")
	if err != nil || binding.StorageObjectID == "80000000-0000-4000-8000-000000000001" {
		t.Fatalf("imported asset binding = %#v, %v", binding, err)
	}
	assetReader, err := storage.Open(ctx, assets.StorageObjectID(binding.StorageObjectID))
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(assetReader)
	closeErr := assetReader.Close()
	if readErr != nil || closeErr != nil || string(body) != "image bytes" {
		t.Fatalf("imported asset bytes=%q read=%v close=%v", body, readErr, closeErr)
	}
	translationService, err := translations.NewService(courseRepo, translationspostgres.New(pool), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	languages, err := translationService.Languages(ctx, first.CourseVersionID)
	if err != nil || len(languages) != 1 || languages[0] != "es" {
		t.Fatalf("imported languages=%#v err=%v", languages, err)
	}
	translatedReader, err := translations.NewLearnerReader(translationService, reader)
	if err != nil {
		t.Fatal(err)
	}
	translated, err := translatedReader.Read(ctx, first.CourseID, first.Version, "es")
	if err != nil || translated.Course.Title != "Curso importado" || translated.Course.Assessments[0].Questions[0].Prompt != "Pregunta" {
		t.Fatalf("imported translated learner Course = %#v, %v", translated, err)
	}

	next := importedPackage(t, "1.1.0", "20000000-0000-4000-8000-000000000902", "Imported course v1.1")
	second, err := importer.Import(ctx, next)
	if err != nil || second.CourseID != first.CourseID || second.CourseVersionID == first.CourseVersionID {
		t.Fatalf("later version import=%#v err=%v", second, err)
	}
	conflicting := importedPackage(t, "1.0.0", "20000000-0000-4000-8000-000000000901", "Conflicting exported Course")
	if _, err := importer.Import(ctx, conflicting); !errors.Is(err, portability.ErrImportSourceVersionConflict) {
		t.Fatalf("source/version conflict error=%v", err)
	}

	failingStorage := newImportedPackageStorage()
	failingRepository, err := portability.NewPostgresImportRepository(pool, failingStorage, 1024)
	if err != nil {
		t.Fatal(err)
	}
	failingImporter, err := portability.NewImporter(failingRepository, func() time.Time { return time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	lateFailure := importedPackageAt(t, "3.0.0", "20000000-0000-4000-8000-000000000904", "Fails after CourseVersion", time.Time{})
	if _, err := failingImporter.Import(ctx, lateFailure); !errors.Is(err, portability.ErrImportPersistenceFailed) {
		t.Fatalf("late translation failure error=%v", err)
	}
	if failingStorage.count() != 0 {
		t.Fatalf("failed import retained %d binary objects", failingStorage.count())
	}
	var failedRecords, failedVersions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM portability.import_record WHERE portable_source_course_version_id='20000000-0000-4000-8000-000000000904'`).Scan(&failedRecords); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM courses.course_version WHERE course_id=$1 AND version='3.0.0'`, first.CourseID).Scan(&failedVersions); err != nil {
		t.Fatal(err)
	}
	if failedRecords != 0 || failedVersions != 0 {
		t.Fatalf("late failure persisted records=%d versions=%d", failedRecords, failedVersions)
	}

	concurrent := importedPackage(t, "2.0.0", "20000000-0000-4000-8000-000000000903", "Concurrent import")
	start := make(chan struct{})
	results := make([]portability.ImportResult, 2)
	errs := make([]error, 2)
	var wait sync.WaitGroup
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errs[index] = importer.Import(ctx, concurrent)
		}(index)
	}
	close(start)
	wait.Wait()
	if errs[0] != nil || errs[1] != nil || results[0].CourseVersionID != results[1].CourseVersionID || results[0].ImportID != results[1].ImportID || results[0].Disposition == results[1].Disposition {
		t.Fatalf("concurrent imports results=%#v errors=%v", results, errs)
	}

	firstForNewSource := importedPackageForSource(t, "1.0.0", "30000000-0000-4000-8000-000000000001", "31000000-0000-4000-8000-000000000001", "Same title")
	secondForNewSource := importedPackageForSource(t, "1.1.0", "30000000-0000-4000-8000-000000000001", "31000000-0000-4000-8000-000000000002", "Same title")
	start = make(chan struct{})
	results = make([]portability.ImportResult, 2)
	errs = make([]error, 2)
	for index, candidate := range []*portability.ValidatedCoursePackage{firstForNewSource, secondForNewSource} {
		wait.Add(1)
		go func(index int, candidate *portability.ValidatedCoursePackage) {
			defer wait.Done()
			<-start
			results[index], errs[index] = importer.Import(ctx, candidate)
		}(index, candidate)
	}
	close(start)
	wait.Wait()
	if errs[0] != nil || errs[1] != nil || results[0].CourseID != results[1].CourseID || results[0].CourseVersionID == results[1].CourseVersionID || results[0].CourseID == first.CourseID {
		t.Fatalf("concurrent source-version mapping results=%#v errors=%v", results, errs)
	}
}

func importedPackage(t *testing.T, version, originVersionID, title string) *portability.ValidatedCoursePackage {
	return importedPackageForSourceAt(t, version, "10000000-0000-4000-8000-000000000001", originVersionID, title, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
}

func importedPackageAt(t *testing.T, version, originVersionID, title string, translatedAt time.Time) *portability.ValidatedCoursePackage {
	return importedPackageForSourceAt(t, version, "10000000-0000-4000-8000-000000000001", originVersionID, title, translatedAt)
}

func importedPackageForSource(t *testing.T, version, originCourseID, originVersionID, title string) *portability.ValidatedCoursePackage {
	return importedPackageForSourceAt(t, version, originCourseID, originVersionID, title, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
}

func importedPackageForSourceAt(t *testing.T, version, originCourseID, originVersionID, title string, translatedAt time.Time) *portability.ValidatedCoursePackage {
	t.Helper()
	assetBytes := []byte("image bytes")
	digest := sha256.Sum256(assetBytes)
	assetDigest := hex.EncodeToString(digest[:])
	course := portability.CoursePayload{Title: title, Description: "Portable description", LearningObjectives: []string{"Objective"}, Language: "en", Version: version, Changelog: "Imported exact version", Modules: []portability.Module{{StableKey: "module-one", Title: "Module", Description: "Module description", Position: 0, Lessons: []portability.Lesson{{StableKey: "lesson-one", Title: "Lesson", Description: "Lesson description", LearningObjectives: []string{"Lesson objective"}, Position: 0, PrerequisiteStableKeys: []string{}, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "image", Type: courses.BlockImage, Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: "70000000-0000-4000-8000-000000000001"}, AltText: "Image", Caption: "Caption"}}, {Key: "check", Type: courses.BlockKnowledgeCheck, Payload: courses.KnowledgeCheckBlockPayload{AssessmentKey: "60000000-0000-4000-8000-000000000001"}}}}}}}}}
	assessments := portability.AssessmentsPayload{Assessments: []portability.Assessment{{AssessmentKey: "60000000-0000-4000-8000-000000000001", Questions: []portability.Question{{StableKey: "question-one", Type: courses.PublishedQuestionSingleChoice, Prompt: "Prompt", Position: 0, Options: []portability.Option{{StableKey: "option-a", Text: "A", Position: 0}, {StableKey: "option-b", Text: "B", Position: 1}}, CorrectOptionKeys: []string{"option-b"}}}}}}
	es, err := courses.NormalizeLanguageTag("es")
	if err != nil {
		t.Fatal(err)
	}
	spanishTitle, spanishDescription, spanishPrompt, spanishOption := "Curso importado", "Descripción importada", "Pregunta", "Opción"
	tree := translations.TranslationTree{Title: &spanishTitle, Description: &spanishDescription, LearningObjectives: []*string{&spanishTitle}, Modules: []translations.TranslatedModule{{SourceStableKey: "module-one", Position: 0, Title: &spanishTitle, Description: &spanishDescription, Lessons: []translations.TranslatedLesson{{SourceStableKey: "lesson-one", Position: 0, Title: &spanishTitle, Description: &spanishDescription, LearningObjectives: []*string{&spanishTitle}, ContentBlocks: []translations.TranslatedContentBlock{{SourceBlockKey: "image", Type: courses.BlockImage, AltText: &spanishTitle, Caption: &spanishDescription}, {SourceBlockKey: "check", Type: courses.BlockKnowledgeCheck}}}}}}, Assessments: []translations.TranslatedAssessment{{AssessmentKey: "60000000-0000-4000-8000-000000000001", Questions: []translations.TranslatedAssessmentQuestion{{SourceStableKey: "question-one", Type: courses.PublishedQuestionSingleChoice, Position: 0, Prompt: &spanishPrompt, Options: []translations.TranslatedAssessmentOption{{SourceStableKey: "option-a", Position: 0, Text: &spanishOption}, {SourceStableKey: "option-b", Position: 1, Text: &spanishOption}}}}}}}
	translation := portability.TranslationPayload{Language: string(es), SourceCourseID: originCourseID, SourceCourseVersionID: originVersionID, SourceVersion: version, PublishedAt: translatedAt, Tree: tree}
	manifest := portability.Manifest{Format: portability.Format, FormatVersion: portability.FormatVersion, ExportedAt: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), Course: portability.ManifestCourse{OriginCourseID: translation.SourceCourseID, OriginCourseVersionID: originVersionID, Version: version, Language: "en", License: portability.License{Kind: string(courses.ContentLicenseStandard), Identifier: "CC-BY-4.0", DisplayName: "CC BY 4.0", URL: "https://creativecommons.org/licenses/by/4.0/"}, Attribution: []portability.Attribution{{DisplayName: "Public author", Role: string(courses.ContributorAuthor), Order: 0}}}, Assets: []portability.AssetEntry{{AssetKey: "70000000-0000-4000-8000-000000000001", OriginalFilename: "image.png", MediaType: "image/png", ByteSize: int64(len(assetBytes)), SHA256: assetDigest, Path: "assets/70000000-0000-4000-8000-000000000001/content"}}, Translations: []portability.TranslationEntry{{Language: "es", Path: "translations/es.json", SourceVersion: version, PublishedAt: translation.PublishedAt}}, ChecksumsPath: "checksums.json"}
	return parseImportedPackage(t, manifest, course, assessments, translation, assetBytes)
}

type importedPackageStorage struct {
	mu      sync.Mutex
	objects map[assets.StorageObjectID][]byte
}

func newImportedPackageStorage() *importedPackageStorage {
	return &importedPackageStorage{objects: map[assets.StorageObjectID][]byte{}}
}
func (s *importedPackageStorage) Put(_ context.Context, source io.Reader) (assets.StoredBinary, error) {
	body, err := io.ReadAll(source)
	if err != nil {
		return assets.StoredBinary{}, err
	}
	sum := sha256.Sum256(body)
	id := assets.StorageObjectID(uuid.NewString())
	s.mu.Lock()
	s.objects[id] = body
	s.mu.Unlock()
	return assets.StoredBinary{StorageObjectID: id, ByteSize: int64(len(body)), SHA256Digest: assets.SHA256Digest(hex.EncodeToString(sum[:]))}, nil
}
func (s *importedPackageStorage) Open(_ context.Context, id assets.StorageObjectID) (io.ReadCloser, error) {
	s.mu.Lock()
	body, ok := s.objects[id]
	s.mu.Unlock()
	if !ok {
		return nil, assets.ErrStorageObjectMissing
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}
func (s *importedPackageStorage) DiscardUncommitted(_ context.Context, id assets.StorageObjectID) error {
	s.mu.Lock()
	delete(s.objects, id)
	s.mu.Unlock()
	return nil
}
func (s *importedPackageStorage) count() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.objects) }

func parseImportedPackage(t *testing.T, manifest portability.Manifest, course portability.CoursePayload, assessments portability.AssessmentsPayload, translation portability.TranslationPayload, asset []byte) *portability.ValidatedCoursePackage {
	t.Helper()
	files := map[string][]byte{}
	var err error
	if files["course.json"], err = json.Marshal(course); err != nil {
		t.Fatal(err)
	}
	if files["assessments.json"], err = json.Marshal(assessments); err != nil {
		t.Fatal(err)
	}
	if files["translations/es.json"], err = json.Marshal(translation); err != nil {
		t.Fatal(err)
	}
	files["assets/70000000-0000-4000-8000-000000000001/content"] = asset
	paths := []string{"course.json", "assessments.json", "translations/es.json", "assets/70000000-0000-4000-8000-000000000001/content"}
	entries := make([]portability.ContentEntry, 0, len(paths))
	for _, path := range paths {
		sum := sha256.Sum256(files[path])
		entries = append(entries, portability.ContentEntry{Path: path, SHA256: hex.EncodeToString(sum[:])})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	manifest.Contents = append([]portability.ContentEntry(nil), entries...)
	checksums, err := json.Marshal(portability.Checksums{Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	for _, path := range []string{"manifest.json", "course.json", "assessments.json", "translations/es.json", "assets/70000000-0000-4000-8000-000000000001/content", "checksums.json"} {
		body := files[path]
		if path == "manifest.json" {
			body = manifestBytes
		}
		if path == "checksums.json" {
			body = checksums
		}
		writer, err := z.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	pkg, err := portability.NewReader(portability.DefaultLimits()).Read(context.Background(), bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}
