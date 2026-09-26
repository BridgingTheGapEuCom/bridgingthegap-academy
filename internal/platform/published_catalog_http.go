package platform

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type publishedCatalogPageDTO struct {
	Items  []publishedCatalogItemDTO `json:"items"`
	Limit  int                       `json:"limit"`
	Offset int                       `json:"offset"`
	Total  int                       `json:"total"`
}

type publishedCatalogItemDTO struct {
	CourseID       string                     `json:"courseId"`
	Version        string                     `json:"version"`
	Title          string                     `json:"title"`
	Description    string                     `json:"description"`
	SourceLanguage string                     `json:"sourceLanguage"`
	License        publishedContentLicenseDTO `json:"license"`
	Contributors   []publishedContributorDTO  `json:"contributors"`
	PublishedAt    string                     `json:"publishedAt"`
}

func (a *authHTTP) handlePublishedCourseCatalog(w http.ResponseWriter, r *http.Request) {
	query, err := publishedCatalogQuery(r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid catalog query")
		return
	}
	page, err := a.publishedCatalog.List(r.Context(), query)
	if err != nil {
		if errors.Is(err, courses.ErrInvalidPublishedCatalogQuery) {
			problem(w, r, http.StatusBadRequest, "Invalid catalog query")
			return
		}
		courseProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, publishedCatalogPage(page))
}

func publishedCatalogQuery(r *http.Request) (courses.PublishedCatalogQuery, error) {
	values := r.URL.Query()
	query := courses.PublishedCatalogQuery{}
	if value, present, err := singleCatalogQueryValue(values["limit"]); err != nil {
		return courses.PublishedCatalogQuery{}, err
	} else if present {
		query.Limit, err = strconv.Atoi(value)
		if err != nil || query.Limit < 1 {
			return courses.PublishedCatalogQuery{}, courses.ErrInvalidPublishedCatalogQuery
		}
	}
	if value, present, err := singleCatalogQueryValue(values["offset"]); err != nil {
		return courses.PublishedCatalogQuery{}, err
	} else if present {
		query.Offset, err = strconv.Atoi(value)
		if err != nil || query.Offset < 0 {
			return courses.PublishedCatalogQuery{}, courses.ErrInvalidPublishedCatalogQuery
		}
	}
	if value, present, err := singleCatalogQueryValue(values["language"]); err != nil {
		return courses.PublishedCatalogQuery{}, err
	} else if present {
		language, normalizeErr := courses.NormalizeLanguageTag(value)
		if normalizeErr != nil {
			return courses.PublishedCatalogQuery{}, courses.ErrInvalidPublishedCatalogQuery
		}
		query.Language = &language
	}
	return query, nil
}

func singleCatalogQueryValue(values []string) (string, bool, error) {
	if len(values) == 0 {
		return "", false, nil
	}
	if len(values) != 1 || values[0] == "" {
		return "", false, courses.ErrInvalidPublishedCatalogQuery
	}
	return values[0], true, nil
}

func publishedCatalogPage(page courses.PublishedCatalogPage) publishedCatalogPageDTO {
	result := publishedCatalogPageDTO{Items: make([]publishedCatalogItemDTO, 0, len(page.Items)), Limit: page.Limit, Offset: page.Offset, Total: page.Total}
	for _, item := range page.Items {
		contributors := make([]publishedContributorDTO, 0, len(item.Contributors))
		for _, contributor := range item.Contributors {
			contributors = append(contributors, publishedContributorDTO{DisplayName: contributor.DisplayName, Role: contributor.Role, Order: contributor.Order})
		}
		result.Items = append(result.Items, publishedCatalogItemDTO{
			CourseID: string(item.CourseID), Version: item.Version.String(), Title: item.Title, Description: item.Description,
			SourceLanguage: string(item.SourceLanguage),
			License:        publishedContentLicenseDTO{Kind: item.License.Kind, Identifier: item.License.Identifier, DisplayName: item.License.DisplayName, URL: item.License.URL, CustomText: item.License.CustomText},
			Contributors:   contributors, PublishedAt: item.PublishedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return result
}
