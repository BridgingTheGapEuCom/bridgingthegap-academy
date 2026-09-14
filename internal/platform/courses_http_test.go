package platform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

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
