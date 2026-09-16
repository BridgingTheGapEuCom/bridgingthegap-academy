package platform

import (
	"net/http"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type publishedCourseVersionDTO struct {
	CourseID       string                     `json:"courseId"`
	Version        string                     `json:"version"`
	Title          string                     `json:"title"`
	Description    string                     `json:"description"`
	Objectives     []string                   `json:"objectives"`
	SourceLanguage string                     `json:"sourceLanguage"`
	Changelog      string                     `json:"changelog"`
	License        publishedContentLicenseDTO `json:"license"`
	Contributors   []publishedContributorDTO  `json:"contributors"`
	PublishedAt    time.Time                  `json:"publishedAt"`
	Modules        []publishedModuleDTO       `json:"modules"`
}

type publishedContentLicenseDTO struct {
	Kind        courses.ContentLicenseKind `json:"kind"`
	Identifier  string                     `json:"identifier"`
	DisplayName string                     `json:"displayName"`
	URL         string                     `json:"url"`
	CustomText  string                     `json:"customText"`
}

type publishedContributorDTO struct {
	DisplayName string                  `json:"displayName"`
	Role        courses.ContributorRole `json:"role"`
	Order       int                     `json:"order"`
}

type publishedModuleDTO struct {
	StableKey   string               `json:"stableKey"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Position    int                  `json:"position"`
	Lessons     []publishedLessonDTO `json:"lessons"`
}

type publishedLessonDTO struct {
	StableKey                string                `json:"stableKey"`
	Title                    string                `json:"title"`
	Description              string                `json:"description"`
	Objectives               []string              `json:"objectives"`
	EstimatedDurationMinutes *int                  `json:"estimatedDurationMinutes"`
	Position                 int                   `json:"position"`
	PrerequisiteStableKeys   []string              `json:"prerequisiteStableKeys"`
	Content                  courses.LessonContent `json:"content"`
}

func (a *authHTTP) handlePublishedCourseVersion(w http.ResponseWriter, r *http.Request) {
	courseID, ok := publishedCourseID(w, r)
	if !ok {
		return
	}
	version, err := courses.ParseVersion(chi.URLParam(r, "version"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid course version")
		return
	}
	result, err := a.publishedCourses.Exact(r.Context(), courseID, version)
	if err != nil {
		courseProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, publishedCourseVersion(result))
}

func (a *authHTTP) handleLatestPublishedCourseVersion(w http.ResponseWriter, r *http.Request) {
	courseID, ok := publishedCourseID(w, r)
	if !ok {
		return
	}
	result, err := a.publishedCourses.Latest(r.Context(), courseID)
	if err != nil {
		courseProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, publishedCourseVersion(result))
}

func publishedCourseID(w http.ResponseWriter, r *http.Request) (courses.CourseID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid course ID")
		return "", false
	}
	return courses.CourseID(id.String()), true
}

func publishedCourseVersion(value courses.PublishedCourseVersion) publishedCourseVersionDTO {
	result := publishedCourseVersionDTO{
		CourseID: string(value.CourseID), Version: value.Version.String(), Title: value.Title,
		Description: value.Description, Objectives: value.LearningObjectives,
		SourceLanguage: string(value.SourceLanguage), Changelog: value.Changelog,
		License: publishedContentLicenseDTO{
			Kind: value.License.Kind, Identifier: value.License.Identifier, DisplayName: value.License.DisplayName,
			URL: value.License.URL, CustomText: value.License.CustomText,
		},
		Contributors: make([]publishedContributorDTO, 0, len(value.Contributors)),
		PublishedAt:  value.PublishedAt,
		Modules:      make([]publishedModuleDTO, 0, len(value.Modules)),
	}
	for _, contributor := range value.Contributors {
		result.Contributors = append(result.Contributors, publishedContributorDTO{
			DisplayName: contributor.DisplayName, Role: contributor.Role, Order: contributor.Order,
		})
	}
	for _, module := range value.Modules {
		publicModule := publishedModuleDTO{
			StableKey: module.StableKey, Title: module.Title, Description: module.Description,
			Position: module.Position, Lessons: make([]publishedLessonDTO, 0, len(module.Lessons)),
		}
		for _, lesson := range module.Lessons {
			publicModule.Lessons = append(publicModule.Lessons, publishedLessonDTO{
				StableKey: lesson.StableKey, Title: lesson.Title, Description: lesson.Description,
				Objectives: lesson.LearningObjectives, EstimatedDurationMinutes: lesson.EstimatedDurationMinutes,
				Position: lesson.Position, PrerequisiteStableKeys: lesson.PrerequisiteStableKeys, Content: lesson.Content,
			})
		}
		result.Modules = append(result.Modules, publicModule)
	}
	return result
}
