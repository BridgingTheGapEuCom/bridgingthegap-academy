package courses

import (
	"context"
	"errors"
	"time"
)

const DefaultPublishedCatalogLimit = 20
const MaxPublishedCatalogLimit = 100
const maxPublishedCatalogOffset = 2147483647

var ErrInvalidPublishedCatalogQuery = errors.New("invalid published catalog query")

// PublishedCatalogRepository returns one latest PUBLISHED version per Course.
// Implementations must apply the language filter after selecting that latest
// version, so a prior version never makes a course match the current catalog.
type PublishedCatalogRepository interface {
	ListLatestPublishedCourseVersions(context.Context, PublishedCatalogQuery) ([]CourseVersion, error)
	CountLatestPublishedCourses(context.Context, *LanguageTag) (int, error)
}

type PublishedCatalogQuery struct {
	Limit    int
	Offset   int
	Language *LanguageTag
}

type PublishedCatalogPage struct {
	Items  []PublishedCatalogItem
	Limit  int
	Offset int
	Total  int
}

// PublishedCatalogItem is intentionally lightweight. Full immutable modules,
// lessons, and canonical LessonContent remain available only through the exact
// and latest CourseVersion read endpoints.
type PublishedCatalogItem struct {
	CourseID       CourseID
	Version        Version
	Title          string
	Description    string
	SourceLanguage LanguageTag
	License        ContentLicense
	Contributors   []PublishedContributor
	PublishedAt    time.Time
}

// PublishedCatalogService is a Courses-only discovery boundary. It has no
// Authoring or publication-workflow dependency.
type PublishedCatalogService struct{ repository PublishedCatalogRepository }

func NewPublishedCatalogService(repository PublishedCatalogRepository) *PublishedCatalogService {
	return &PublishedCatalogService{repository: repository}
}

func (s *PublishedCatalogService) List(ctx context.Context, query PublishedCatalogQuery) (PublishedCatalogPage, error) {
	if s == nil || s.repository == nil {
		return PublishedCatalogPage{}, ErrNotFound
	}
	normalized, err := normalizePublishedCatalogQuery(query)
	if err != nil {
		return PublishedCatalogPage{}, err
	}
	versions, err := s.repository.ListLatestPublishedCourseVersions(ctx, normalized)
	if err != nil {
		return PublishedCatalogPage{}, err
	}
	total, err := s.repository.CountLatestPublishedCourses(ctx, normalized.Language)
	if err != nil {
		return PublishedCatalogPage{}, err
	}
	page := PublishedCatalogPage{Items: make([]PublishedCatalogItem, 0, len(versions)), Limit: normalized.Limit, Offset: normalized.Offset, Total: total}
	for _, version := range versions {
		if version.Status != CourseVersionPublished {
			return PublishedCatalogPage{}, errors.New("invalid stored catalog version")
		}
		item := PublishedCatalogItem{
			CourseID: version.CourseID, Version: version.Version, Title: version.Title, Description: version.Description,
			SourceLanguage: version.SourceLanguage, License: version.License,
			Contributors: make([]PublishedContributor, 0, len(version.Attribution)), PublishedAt: version.PublishedAt.UTC(),
		}
		for _, contributor := range version.Attribution {
			item.Contributors = append(item.Contributors, PublishedContributor{DisplayName: contributor.DisplayName, Role: contributor.Role, Order: contributor.Order})
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}

func normalizePublishedCatalogQuery(query PublishedCatalogQuery) (PublishedCatalogQuery, error) {
	if query.Limit == 0 {
		query.Limit = DefaultPublishedCatalogLimit
	}
	if query.Limit < 1 || query.Limit > MaxPublishedCatalogLimit || query.Offset < 0 || query.Offset > maxPublishedCatalogOffset {
		return PublishedCatalogQuery{}, ErrInvalidPublishedCatalogQuery
	}
	if query.Language != nil {
		normalized, err := NormalizeLanguageTag(string(*query.Language))
		if err != nil || normalized != *query.Language {
			return PublishedCatalogQuery{}, ErrInvalidPublishedCatalogQuery
		}
	}
	return query, nil
}
