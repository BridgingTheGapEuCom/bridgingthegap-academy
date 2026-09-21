package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

func TestPublishedCourseVersionDTOOmitsAssessmentAnswerDefinitions(t *testing.T) {
	value := courses.PublishedCourseVersion{
		Assessments: []courses.PublishedAssessmentLearnerView{{
			AssessmentKey: "10000000-0000-4000-8000-000000000001",
			Questions: []courses.PublishedAssessmentLearnerQuestion{{
				StableKey: "question", Type: courses.PublishedQuestionSingleChoice, Prompt: "Choose", Position: 0,
				Options: []courses.PublishedAssessmentLearnerOption{{StableKey: "one", Text: "One", Position: 0}, {StableKey: "two", Text: "Two", Position: 1}},
			}},
		}},
	}
	encoded, err := json.Marshal(publishedCourseVersion(value))
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"correctOption", "correctPairs", "answerKey", "answers", "creator", "draft"} {
		if strings.Contains(strings.ToLower(string(encoded)), strings.ToLower(private)) {
			t.Fatalf("public DTO leaked %q: %s", private, encoded)
		}
	}
}

type publishedCourseHTTPRepository struct {
	exact     courses.ImmutableCourseVersion
	latest    courses.ImmutableCourseVersion
	exactErr  error
	latestErr error
}

type publishedCatalogHTTPRepository struct {
	versions []courses.CourseVersion
	count    int
	err      error
	query    courses.PublishedCatalogQuery
	language *courses.LanguageTag
}

func (r *publishedCatalogHTTPRepository) ListLatestPublishedCourseVersions(_ context.Context, query courses.PublishedCatalogQuery) ([]courses.CourseVersion, error) {
	r.query = query
	return r.versions, r.err
}

func (r *publishedCatalogHTTPRepository) CountLatestPublishedCourses(_ context.Context, language *courses.LanguageTag) (int, error) {
	if language != nil {
		value := *language
		r.language = &value
	}
	return r.count, r.err
}

func publishedCatalogHTTPVersion(t *testing.T, courseID, versionText, title, language string) courses.CourseVersion {
	t.Helper()
	version, err := courses.ParseVersion(versionText)
	if err != nil {
		t.Fatal(err)
	}
	return courses.CourseVersion{
		ID:             courses.CourseVersionID("version-" + courseID),
		CourseID:       courses.CourseID(courseID),
		Version:        version,
		Status:         courses.CourseVersionPublished,
		Title:          title,
		Description:    "A lightweight published-course summary.",
		SourceLanguage: courses.LanguageTag(language),
		License:        courses.ContentLicense{Kind: courses.ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"},
		Attribution:    []courses.ContributorSnapshot{{UserID: "private-user-id", DisplayName: "Public author", Role: courses.ContributorAuthor, Order: 0}},
		PublishedAt:    time.Date(2026, time.September, 16, 12, 0, 0, 0, time.UTC),
	}
}

func (r publishedCourseHTTPRepository) GetPublishedImmutableCourseVersionByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error) {
	return r.exact, r.exactErr
}

func (r publishedCourseHTTPRepository) GetLatestPublishedImmutableCourseVersion(context.Context, courses.CourseID) (courses.ImmutableCourseVersion, error) {
	return r.latest, r.latestErr
}

func publishedCourseHTTPFixture(t *testing.T) courses.ImmutableCourseVersion {
	t.Helper()
	version, err := courses.ParseVersion("1.10.0")
	if err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Date(2026, time.September, 16, 12, 0, 0, 0, time.UTC)
	return courses.ImmutableCourseVersion{
		ID: "40000000-0000-4000-8000-000000000001",
		CourseVersion: courses.CourseVersionInput{
			CourseID: "10000000-0000-4000-8000-000000000001", Version: version, Status: courses.CourseVersionPublished,
			Title: "Published course", Description: "An immutable published course.", LearningObjectives: []string{"Read published content"},
			SourceLanguage: "en-GB", Changelog: "Version 1.10.0.",
			License:     courses.ContentLicense{Kind: courses.ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"},
			Attribution: []courses.ContributorSnapshot{{UserID: "20000000-0000-4000-8000-000000000001", DisplayName: "Ada Author", Role: courses.ContributorAuthor, Order: 0}},
			PublishedAt: publishedAt,
		},
		AssetBindings: []courses.PublishedAssetBinding{{
			AssetKey: "50000000-0000-4000-8000-000000000001", StorageObjectID: "60000000-0000-4000-8000-000000000001",
			OriginalFilename: "architecture.png", MediaType: "image/png", ByteSize: 100,
			SHA256Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
		Modules: []courses.ImmutableCourseVersionModule{{
			StableKey: "foundations", Title: "Foundations", Description: "Core material.", Position: 0,
			Lessons: []courses.ImmutableCourseVersionLesson{{
				StableKey: "introduction", Title: "Introduction", Description: "First lesson.", LearningObjectives: []string{"Understand the version"}, Position: 0,
				PrerequisiteStableKeys: []string{}, Content: courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{{Key: "asset", Type: courses.BlockImage, Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: "50000000-0000-4000-8000-000000000001"}, AltText: "Architecture diagram"}}}},
			}},
		}},
	}
}

type courseHTTPRepository struct {
	courses.Repository
	course        courses.Course
	versions      []courses.CourseVersion
	modules       []courses.Module
	lessons       []courses.Lesson
	prerequisites []courses.LessonPrerequisite
}

func (r courseHTTPRepository) GetCourseBySlug(context.Context, string) (courses.Course, error) {
	return r.course, nil
}
func (r courseHTTPRepository) ListCourses(context.Context) ([]courses.Course, error) {
	return []courses.Course{r.course}, nil
}
func (r courseHTTPRepository) ListCourseVersions(context.Context, courses.CourseID) ([]courses.CourseVersion, error) {
	return r.versions, nil
}
func (r courseHTTPRepository) ListPublishedCourseVersions(context.Context) ([]courses.CourseVersion, error) {
	result := make([]courses.CourseVersion, 0, len(r.versions))
	for _, version := range r.versions {
		if version.Status == courses.CourseVersionPublished {
			result = append(result, version)
		}
	}
	return result, nil
}
func (r courseHTTPRepository) GetCourseVersionByCourseAndVersion(_ context.Context, _ courses.CourseID, version courses.Version) (courses.CourseVersion, error) {
	for _, item := range r.versions {
		if item.Version == version {
			return item, nil
		}
	}
	return courses.CourseVersion{}, courses.ErrNotFound
}
func (r courseHTTPRepository) ListModulesForCourseVersion(context.Context, courses.CourseVersionID) ([]courses.Module, error) {
	return r.modules, nil
}
func (r courseHTTPRepository) ListLessonSummariesForCourseVersion(context.Context, courses.CourseVersionID) ([]courses.LessonSummary, error) {
	summaries := make([]courses.LessonSummary, 0, len(r.lessons))
	for _, lesson := range r.lessons {
		summaries = append(summaries, courses.LessonSummary{ID: lesson.ID, ModuleID: lesson.ModuleID, StableKey: lesson.StableKey, Title: lesson.Title, Description: lesson.Description, LearningObjectives: lesson.LearningObjectives, EstimatedDurationMinutes: lesson.EstimatedDurationMinutes, Position: lesson.Position})
	}
	return summaries, nil
}
func (r courseHTTPRepository) ListLessonPrerequisitesForCourseVersion(context.Context, courses.CourseVersionID) ([]courses.LessonPrerequisite, error) {
	return r.prerequisites, nil
}
func (r courseHTTPRepository) GetLessonByCourseVersionAndKey(_ context.Context, _ courses.CourseVersionID, key string) (courses.Lesson, error) {
	for _, item := range r.lessons {
		if item.StableKey == key {
			return item, nil
		}
	}
	return courses.Lesson{}, courses.ErrNotFound
}
func (r courseHTTPRepository) GetModule(_ context.Context, id courses.ModuleID) (courses.Module, error) {
	for _, item := range r.modules {
		if item.ID == id {
			return item, nil
		}
	}
	return courses.Module{}, courses.ErrNotFound
}
func (r courseHTTPRepository) ListLessonPrerequisites(_ context.Context, lessonID courses.LessonID) ([]courses.LessonPrerequisite, error) {
	result := []courses.LessonPrerequisite{}
	for _, item := range r.prerequisites {
		if item.LessonID == lessonID {
			result = append(result, item)
		}
	}
	return result, nil
}

func TestPublishedCourseHTTPReadPolicyAndCacheHeaders(t *testing.T) {
	version := courses.Version{Major: 2}
	content := courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{}}
	repository := courseHTTPRepository{
		course:   courses.Course{ID: "course", Slug: "event-driven-architecture"},
		versions: []courses.CourseVersion{{ID: "published", CourseID: "course", Version: version, Status: courses.CourseVersionPublished}, {ID: "withdrawn", CourseID: "course", Version: courses.Version{Major: 3}, Status: courses.CourseVersionWithdrawn}},
		modules:  []courses.Module{{ID: "module", CourseVersionID: "published", StableKey: "fundamentals", Title: "Fundamentals", Position: 0}},
		lessons:  []courses.Lesson{{ID: "lesson", CourseVersionID: "published", ModuleID: "module", StableKey: "what-is-eai", Title: "What is EAI?", Description: "Lesson summary.", Position: 0, Content: content}},
	}
	router := authTestRouter(&authHTTP{courses: courses.NewReadService(repository)})

	for _, check := range []struct {
		path   string
		status int
		cache  string
	}{
		{"/api/courses", http.StatusOK, "no-store"},
		{"/api/courses/event-driven-architecture", http.StatusOK, "no-store"},
		{"/api/courses/event-driven-architecture/versions/2.0.0", http.StatusOK, "no-store"},
		{"/api/courses/event-driven-architecture/versions/2.0.0/lessons/what-is-eai", http.StatusOK, "no-store"},
		{"/api/courses/event-driven-architecture/versions/3.0.0", http.StatusNotFound, ""},
		{"/api/courses/not%20valid", http.StatusBadRequest, ""},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, check.path, nil))
		if response.Code != check.status {
			t.Fatalf("GET %s = %d, want %d: %s", check.path, response.Code, check.status, response.Body.String())
		}
		if check.cache != "" && response.Header().Get("Cache-Control") != check.cache {
			t.Fatalf("GET %s cache = %q, want %q", check.path, response.Header().Get("Cache-Control"), check.cache)
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/courses/event-driven-architecture", nil))
	if strings.Contains(response.Body.String(), `"content"`) || strings.Contains(response.Body.String(), `"blocks"`) {
		t.Fatalf("course outline leaked full lesson documents: %s", response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/courses/event-driven-architecture/versions/2.0.0/lessons/what-is-eai", nil))
	if strings.Contains(response.Body.String(), `"user_id"`) || !strings.Contains(response.Body.String(), `"schemaVersion":1`) || !strings.Contains(response.Body.String(), `"recommended_prerequisite_keys"`) {
		t.Fatalf("lesson public DTO leaked internal attribution or omitted canonical semantic content: %s", response.Body.String())
	}
}

func TestImmutablePublishedCourseVersionHTTP(t *testing.T) {
	aggregate := publishedCourseHTTPFixture(t)
	router := authTestRouter(&authHTTP{publishedCourses: courses.NewPublishedReadService(publishedCourseHTTPRepository{exact: aggregate, latest: aggregate})})
	base := "/api/courses/by-id/" + string(aggregate.CourseVersion.CourseID)

	for _, path := range []string{base + "/versions/1.10.0", base + "/latest"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("GET %s = %d, cache=%q: %s", path, response.Code, response.Header().Get("Cache-Control"), response.Body.String())
		}
		body := response.Body.String()
		for _, value := range []string{`"courseId":"10000000-0000-4000-8000-000000000001"`, `"version":"1.10.0"`, `"stableKey":"foundations"`, `"stableKey":"introduction"`, `"schemaVersion":1`, `"assetKey":"50000000-0000-4000-8000-000000000001"`} {
			if !strings.Contains(body, value) {
				t.Fatalf("GET %s omitted immutable content %s: %s", path, value, body)
			}
		}
		for _, hidden := range []string{`"reviewId"`, `"draftId"`, `"snapshotSchemaVersion"`, `"submittedBy"`, `"userId"`, `"storageObjectId"`, `60000000-0000-4000-8000-000000000001`} {
			if strings.Contains(body, hidden) {
				t.Fatalf("GET %s exposed internal provenance %s: %s", path, hidden, body)
			}
		}
	}

	for _, path := range []string{
		"/api/courses/by-id/not-a-uuid/latest",
		base + "/versions/1.10.0-rc.1",
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("GET %s = %d, want bad request: %s", path, response.Code, response.Body.String())
		}
	}
}

func TestImmutablePublishedCourseVersionHTTPErrorsAreSanitized(t *testing.T) {
	aggregate := publishedCourseHTTPFixture(t)
	base := "/api/courses/by-id/" + string(aggregate.CourseVersion.CourseID)
	for _, scenario := range []struct {
		name   string
		repo   publishedCourseHTTPRepository
		path   string
		status int
	}{
		{name: "missing exact", repo: publishedCourseHTTPRepository{exactErr: courses.ErrNotFound}, path: base + "/versions/1.10.0", status: http.StatusNotFound},
		{name: "no published latest", repo: publishedCourseHTTPRepository{latestErr: courses.ErrNotFound}, path: base + "/latest", status: http.StatusNotFound},
		{name: "storage failure", repo: publishedCourseHTTPRepository{exactErr: errors.New("SQLSTATE 42P01 private_table")}, path: base + "/versions/1.10.0", status: http.StatusInternalServerError},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			router := authTestRouter(&authHTTP{publishedCourses: courses.NewPublishedReadService(scenario.repo)})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, scenario.path, nil))
			if response.Code != scenario.status || strings.Contains(response.Body.String(), "private_table") {
				t.Fatalf("GET %s = %d: %s", scenario.path, response.Code, response.Body.String())
			}
		})
	}
}

func TestPublishedCourseCatalogHTTP(t *testing.T) {
	firstID := "10000000-0000-4000-8000-000000000001"
	secondID := "10000000-0000-4000-8000-000000000002"
	repository := &publishedCatalogHTTPRepository{
		versions: []courses.CourseVersion{
			publishedCatalogHTTPVersion(t, firstID, "1.10.0", "Architecture", "en-GB"),
			publishedCatalogHTTPVersion(t, secondID, "2.0.0", "Boundaries", "en-GB"),
		},
		count: 2,
	}
	router := authTestRouter(&authHTTP{publishedCatalog: courses.NewPublishedCatalogService(repository)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/courses/catalog?limit=1&offset=1&language=en-GB", nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("catalog response = %d cache=%q: %s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	body := response.Body.String()
	for _, value := range []string{`"items"`, `"limit":1`, `"offset":1`, `"total":2`, `"courseId":"` + firstID + `"`, `"version":"1.10.0"`, `"publishedAt"`} {
		if !strings.Contains(body, value) {
			t.Fatalf("catalog omitted %s: %s", value, body)
		}
	}
	for _, hidden := range []string{`"reviewId"`, `"draftId"`, `"snapshotSchemaVersion"`, `"userId"`, `"modules"`, `"lessons"`, `"content"`, `"blocks"`} {
		if strings.Contains(body, hidden) {
			t.Fatalf("catalog exposed %s: %s", hidden, body)
		}
	}
	if repository.query.Limit != 1 || repository.query.Offset != 1 || repository.query.Language == nil || string(*repository.query.Language) != "en-GB" || repository.language == nil || string(*repository.language) != "en-GB" {
		t.Fatalf("catalog query was not normalized and forwarded: %#v %#v", repository.query, repository.language)
	}
}

func TestPublishedCourseCatalogHTTPEmptyPage(t *testing.T) {
	router := authTestRouter(&authHTTP{publishedCatalog: courses.NewPublishedCatalogService(&publishedCatalogHTTPRepository{})})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/courses/catalog", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"items":[]`) || !strings.Contains(response.Body.String(), `"total":0`) {
		t.Fatalf("empty catalog response = %d: %s", response.Code, response.Body.String())
	}
}

func TestPublishedCourseCatalogHTTPValidationAndErrors(t *testing.T) {
	for _, path := range []string{
		"/api/courses/catalog?limit=0",
		"/api/courses/catalog?limit=101",
		"/api/courses/catalog?offset=-1",
		"/api/courses/catalog?language=not%20a%20language",
		"/api/courses/catalog?limit=1&limit=2",
	} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			authTestRouter(&authHTTP{publishedCatalog: courses.NewPublishedCatalogService(&publishedCatalogHTTPRepository{})}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("GET %s = %d, want 400: %s", path, response.Code, response.Body.String())
			}
		})
	}
	response := httptest.NewRecorder()
	authTestRouter(&authHTTP{publishedCatalog: courses.NewPublishedCatalogService(&publishedCatalogHTTPRepository{err: errors.New("SQLSTATE 42P01 private_publication_table")})}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/courses/catalog", nil))
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "private_publication_table") {
		t.Fatalf("catalog storage error = %d: %s", response.Code, response.Body.String())
	}
}
