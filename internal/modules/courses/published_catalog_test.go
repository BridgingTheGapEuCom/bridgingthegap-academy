package courses

import (
	"context"
	"errors"
	"testing"
	"time"
)

type publishedCatalogRepositoryFake struct {
	versions []CourseVersion
	count    int
	err      error
	query    PublishedCatalogQuery
	language *LanguageTag
}

func (r *publishedCatalogRepositoryFake) ListLatestPublishedCourseVersions(_ context.Context, query PublishedCatalogQuery) ([]CourseVersion, error) {
	r.query = query
	return r.versions, r.err
}

func (r *publishedCatalogRepositoryFake) CountLatestPublishedCourses(_ context.Context, language *LanguageTag) (int, error) {
	if language != nil {
		value := *language
		r.language = &value
	}
	return r.count, r.err
}

func publishedCatalogVersion(t *testing.T, courseID CourseID, version Version, title string) CourseVersion {
	t.Helper()
	return CourseVersion{
		ID:             CourseVersionID("version-" + string(courseID)),
		CourseID:       courseID,
		Version:        version,
		Status:         CourseVersionPublished,
		Title:          title,
		Description:    "A lightweight immutable publication summary.",
		SourceLanguage: LanguageTag("en-GB"),
		License:        ContentLicense{Kind: ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"},
		Attribution:    []ContributorSnapshot{{UserID: "private-user-id", DisplayName: "Public author", Role: ContributorAuthor, Order: 0}},
		PublishedAt:    time.Date(2026, time.September, 16, 12, 0, 0, 0, time.UTC),
	}
}

func TestPublishedCatalogServiceReturnsLightweightLatestSummaries(t *testing.T) {
	version, err := ParseVersion("1.10.0")
	if err != nil {
		t.Fatal(err)
	}
	repository := &publishedCatalogRepositoryFake{
		versions: []CourseVersion{publishedCatalogVersion(t, "10000000-0000-4000-8000-000000000001", version, "Architecture")},
		count:    1,
	}
	page, err := NewPublishedCatalogService(repository).List(context.Background(), PublishedCatalogQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if repository.query.Limit != DefaultPublishedCatalogLimit || repository.query.Offset != 0 || page.Limit != DefaultPublishedCatalogLimit || page.Total != 1 {
		t.Fatalf("catalog pagination = query %#v, page %#v", repository.query, page)
	}
	if len(page.Items) != 1 || page.Items[0].Version.String() != "1.10.0" || page.Items[0].Title != "Architecture" {
		t.Fatalf("catalog item = %#v", page.Items)
	}
	if len(page.Items[0].Contributors) != 1 || page.Items[0].Contributors[0].DisplayName != "Public author" {
		t.Fatalf("public attribution was not projected: %#v", page.Items[0].Contributors)
	}
}

func TestPublishedCatalogServiceNormalizesOnlyValidQueryValues(t *testing.T) {
	language := LanguageTag("en-GB")
	repository := &publishedCatalogRepositoryFake{}
	service := NewPublishedCatalogService(repository)

	if _, err := service.List(context.Background(), PublishedCatalogQuery{Limit: MaxPublishedCatalogLimit + 1}); !errors.Is(err, ErrInvalidPublishedCatalogQuery) {
		t.Fatalf("over-limit query error = %v", err)
	}
	if _, err := service.List(context.Background(), PublishedCatalogQuery{Limit: 1, Offset: -1}); !errors.Is(err, ErrInvalidPublishedCatalogQuery) {
		t.Fatalf("negative offset query error = %v", err)
	}
	unnormalized := LanguageTag("EN-gb")
	if _, err := service.List(context.Background(), PublishedCatalogQuery{Limit: 1, Language: &unnormalized}); !errors.Is(err, ErrInvalidPublishedCatalogQuery) {
		t.Fatalf("unnormalized language query error = %v", err)
	}
	if _, err := service.List(context.Background(), PublishedCatalogQuery{Limit: 1, Offset: 2, Language: &language}); err != nil {
		t.Fatal(err)
	}
	if repository.query.Language == nil || *repository.query.Language != language || repository.language == nil || *repository.language != language {
		t.Fatalf("language filter was not passed to both catalog queries: %#v %#v", repository.query, repository.language)
	}
}

func TestPublishedCatalogServiceRejectsUnexpectedUnpublishedRows(t *testing.T) {
	version, err := ParseVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	stored := publishedCatalogVersion(t, "10000000-0000-4000-8000-000000000001", version, "Architecture")
	stored.Status = CourseVersionArchived
	if _, err := NewPublishedCatalogService(&publishedCatalogRepositoryFake{versions: []CourseVersion{stored}}).List(context.Background(), PublishedCatalogQuery{}); err == nil {
		t.Fatal("unpublished row was accepted")
	}
}
