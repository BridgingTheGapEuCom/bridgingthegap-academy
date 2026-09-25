package platform

import (
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (a *authHTTP) handleCourseList(w http.ResponseWriter, r *http.Request) {
	reads, e := a.courses.Discover(r.Context())
	if e != nil {
		courseProblem(w, r, e)
		return
	}
	items := make([]any, 0, len(reads))
	for _, x := range reads {
		items = append(items, summary(x))
	}
	writeJSON(w, 200, map[string]any{"courses": items})
}
func (a *authHTTP) handleCourseCurrent(w http.ResponseWriter, r *http.Request) {
	slug, ok := courseSlug(w, r)
	if !ok {
		return
	}
	x, e := a.courses.Preferred(r.Context(), slug)
	if e != nil {
		courseProblem(w, r, e)
		return
	}
	writeJSON(w, 200, detail(x))
}
func (a *authHTTP) handleCourseVersion(w http.ResponseWriter, r *http.Request) {
	slug, ok := courseSlug(w, r)
	if !ok {
		return
	}
	v, e := courses.ParseVersion(chi.URLParam(r, "version"))
	if e != nil {
		problem(w, r, 400, "Invalid course version")
		return
	}
	x, e := a.courses.Explicit(r.Context(), slug, v)
	if e != nil {
		courseProblem(w, r, e)
		return
	}
	writeJSON(w, 200, detail(x))
}
func (a *authHTTP) handleLesson(w http.ResponseWriter, r *http.Request) {
	slug, ok := courseSlug(w, r)
	if !ok {
		return
	}
	key := chi.URLParam(r, "lessonKey")
	if _, e := courses.NormalizeStructureKey(key); e != nil {
		problem(w, r, 400, "Invalid lesson key")
		return
	}
	v, e := courses.ParseVersion(chi.URLParam(r, "version"))
	if e != nil {
		problem(w, r, 400, "Invalid course version")
		return
	}
	x, m, l, p, e := a.courses.Lesson(r.Context(), slug, v, key)
	if e != nil {
		courseProblem(w, r, e)
		return
	}
	keys := []string{}
	for _, q := range p {
		keys = append(keys, q.PrerequisiteStableKey)
	}
	writeJSON(w, 200, map[string]any{"course": map[string]any{"slug": x.Course.Slug, "courseId": x.Course.ID}, "version": version(x.Version), "module": module(m), "lesson": lessonDetail(l, keys)})
}
func courseSlug(w http.ResponseWriter, r *http.Request) (string, bool) {
	slug := chi.URLParam(r, "slug")
	if _, err := courses.NormalizeSlug(slug); err != nil {
		problem(w, r, 400, "Invalid course slug")
		return "", false
	}
	return slug, true
}
func courseProblem(w http.ResponseWriter, r *http.Request, e error) {
	if errors.Is(e, courses.ErrNotFound) || errors.Is(e, courses.ErrNotServable) {
		problem(w, r, 404, "Not found")
		return
	}
	problem(w, r, 500, "Course service unavailable")
}
func summary(x courses.CourseRead) any {
	return map[string]any{"slug": x.Course.Slug, "version": version(x.Version)}
}
func detail(x courses.CourseRead) any {
	ms := []any{}
	for _, s := range x.Modules {
		ls := []any{}
		for _, l := range s.Lessons {
			ls = append(ls, lessonSummary(l.Lesson, l.RecommendedPrerequisiteKeys))
		}
		ms = append(ms, map[string]any{"module": module(s.Module), "lessons": ls})
	}
	return map[string]any{"course": map[string]any{"slug": x.Course.Slug}, "version": version(x.Version), "modules": ms}
}
func version(v courses.CourseVersion) any {
	return map[string]any{"version": v.Version.String(), "status": v.Status, "title": v.Title, "description": v.Description, "objectives": v.LearningObjectives, "source_language": v.SourceLanguage, "license": map[string]any{"kind": v.License.Kind, "identifier": v.License.Identifier, "display_name": v.License.DisplayName, "url": v.License.URL, "custom_text": v.License.CustomText}, "contributors": contributors(v.Attribution)}
}
func contributors(c []courses.ContributorSnapshot) []any {
	r := []any{}
	for _, x := range c {
		r = append(r, map[string]any{"display_name": x.DisplayName, "role": x.Role, "order": x.Order})
	}
	return r
}
func module(m courses.Module) any {
	return map[string]any{"key": m.StableKey, "title": m.Title, "description": m.Description, "position": m.Position}
}
func lessonSummary(l courses.LessonSummary, p []string) map[string]any {
	return map[string]any{"key": l.StableKey, "title": l.Title, "description": l.Description, "objectives": l.LearningObjectives, "estimated_duration_minutes": l.EstimatedDurationMinutes, "position": l.Position, "recommended_prerequisite_keys": p}
}
func lessonDetail(l courses.Lesson, p []string) map[string]any {
	r := lessonSummary(courses.LessonSummary{StableKey: l.StableKey, Title: l.Title, Description: l.Description, LearningObjectives: l.LearningObjectives, EstimatedDurationMinutes: l.EstimatedDurationMinutes, Position: l.Position}, p)
	r["content"] = l.Content
	return r
}
