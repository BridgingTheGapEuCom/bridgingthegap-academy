package platform

import (
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
)

type translatedCourseDTO struct {
	Course      publishedCourseVersionDTO `json:"course"`
	Translation translationMetadataDTO    `json:"translation"`
	SourceLag   sourceLagDTO              `json:"sourceLag"`
}
type translationMetadataDTO struct {
	Language       string `json:"language"`
	SourceLanguage string `json:"sourceLanguage"`
	SourceVersion  string `json:"sourceVersion"`
	PublicationID  string `json:"publicationId"`
	PublishedAt    string `json:"publishedAt"`
}
type sourceLagDTO struct {
	TranslatedSourceVersion string  `json:"translatedSourceVersion"`
	LatestSourceVersion     *string `json:"latestSourceVersion"`
	IsLatest                *bool   `json:"isLatest"`
}

func (a *authHTTP) handleLearnerTranslatedCourse(w http.ResponseWriter, r *http.Request) {
	id, v, ok := translationPublicContext(w, r)
	if !ok {
		return
	}
	lang, err := courses.NormalizeLanguageTag(chi.URLParam(r, "language"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid translation language")
		return
	}
	value, err := a.translatedCourses.Read(r.Context(), id, v, lang)
	if err != nil {
		translationPublicProblem(w, r, err)
		return
	}
	var latest *string
	if value.Lag.LatestSourceVersion != nil {
		x := value.Lag.LatestSourceVersion.String()
		latest = &x
	}
	writeJSON(w, http.StatusOK, translatedCourseDTO{Course: publishedCourseVersion(value.Course), Translation: translationMetadataDTO{Language: string(value.Translation.Language), SourceLanguage: string(value.Translation.SourceLanguage), SourceVersion: value.Translation.SourceVersion.String(), PublicationID: string(value.Translation.PublicationID), PublishedAt: value.Translation.PublishedAt}, SourceLag: sourceLagDTO{TranslatedSourceVersion: value.Lag.TranslatedSourceVersion.String(), LatestSourceVersion: latest, IsLatest: value.Lag.IsLatest}})
}
func (a *authHTTP) handleTranslationLanguages(w http.ResponseWriter, r *http.Request) {
	id, v, ok := translationPublicContext(w, r)
	if !ok {
		return
	}
	values, err := a.translatedCourses.Languages(r.Context(), id, v)
	if err != nil {
		translationPublicProblem(w, r, err)
		return
	}
	type choice struct {
		Language      string  `json:"language"`
		Kind          string  `json:"kind"`
		PublicationID *string `json:"translationPublicationId"`
	}
	out := make([]choice, 0, len(values))
	for _, x := range values {
		var p *string
		if x.PublicationID != nil {
			z := string(*x.PublicationID)
			p = &z
		}
		out = append(out, choice{string(x.Language), x.Kind, p})
	}
	writeJSON(w, http.StatusOK, struct {
		SourceLanguage string   `json:"sourceLanguage"`
		Languages      []choice `json:"languages"`
	}{SourceLanguage: string(values[0].Language), Languages: out})
}
func translationPublicContext(w http.ResponseWriter, r *http.Request) (courses.CourseID, courses.Version, bool) {
	raw := chi.URLParam(r, "courseId")
	id, err := uuid.Parse(raw)
	if err != nil || id.String() != raw {
		problem(w, r, http.StatusBadRequest, "Invalid course ID")
		return "", courses.Version{}, false
	}
	v, err := courses.ParseVersion(chi.URLParam(r, "version"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid course version")
		return "", courses.Version{}, false
	}
	return courses.CourseID(raw), v, true
}
func translationPublicProblem(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, translations.ErrTranslationNotFound) || errors.Is(err, translations.ErrSourceIntegrity) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	courseProblem(w, r, err)
}
