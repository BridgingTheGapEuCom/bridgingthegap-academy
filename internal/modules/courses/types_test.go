package courses

import (
	"testing"
	"time"
)

func TestCourseVersionInputValidation(t *testing.T) {
	version, err := ParseVersion("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	language, err := NormalizeLanguageTag("en-gb")
	if err != nil {
		t.Fatal(err)
	}
	input := CourseVersionInput{
		CourseID:           "11111111-1111-1111-1111-111111111111",
		Version:            version,
		Status:             CourseVersionPublished,
		Title:              "Event-driven architecture",
		Description:        "A concise introduction to event-driven architecture.",
		LearningObjectives: []string{"Explain asynchronous boundaries", "Identify event ownership"},
		SourceLanguage:     language,
		Changelog:          "Initial published version.",
		License:            ContentLicense{Kind: ContentLicenseStandard, Identifier: "CC-BY-4.0", DisplayName: "Creative Commons Attribution 4.0", URL: "https://creativecommons.org/licenses/by/4.0/"},
		Attribution: []ContributorSnapshot{
			{DisplayName: "Ada Author", Role: ContributorAuthor, Order: 0},
			{DisplayName: "Mina Maintainer", Role: ContributorMaintainer, Order: 1},
		},
		PublishedAt: time.Now().UTC(),
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	input.Title = ""
	if err := input.Validate(); err == nil {
		t.Fatal("empty title accepted")
	}
	input.Title = "Event-driven architecture"
	input.Changelog = ""
	if err := input.Validate(); err == nil {
		t.Fatal("empty changelog accepted")
	}
}

func TestVersionParsingAndOrdering(t *testing.T) {
	for _, invalid := range []string{"1", "1.0", "01.0.0", "1.0.0-beta", "1.0.0.1", "1000000000.0.0"} {
		if _, err := ParseVersion(invalid); err == nil {
			t.Fatalf("invalid version accepted: %q", invalid)
		}
	}
	first, _ := ParseVersion("1.10.0")
	second, _ := ParseVersion("1.2.0")
	if first.Compare(second) <= 0 {
		t.Fatal("semantic version comparison was lexical or incorrect")
	}
}

func TestCourseVersionStatusAndTransitions(t *testing.T) {
	for _, status := range []CourseVersionStatus{CourseVersionPublished, CourseVersionDeprecated, CourseVersionArchived, CourseVersionWithdrawn} {
		if got, err := ParseCourseVersionStatus(string(status)); err != nil || got != status {
			t.Fatalf("status %q rejected: %v", status, err)
		}
	}
	if _, err := ParseCourseVersionStatus("DRAFT"); err == nil {
		t.Fatal("draft status accepted")
	}
	if !CourseVersionPublished.CanTransitionTo(CourseVersionDeprecated) || CourseVersionArchived.CanTransitionTo(CourseVersionPublished) || CourseVersionWithdrawn.CanTransitionTo(CourseVersionArchived) {
		t.Fatal("unexpected lifecycle transitions")
	}
}

func TestLanguageLicenseAndAttributionValidation(t *testing.T) {
	if got, err := NormalizeLanguageTag("EN-gb"); err != nil || got != "en-GB" {
		t.Fatalf("language canonicalization failed: %q, %v", got, err)
	}
	for _, language := range []string{"", "english", "en_uk", "en-"} {
		if _, err := NormalizeLanguageTag(language); err == nil {
			t.Fatalf("invalid language accepted: %q", language)
		}
	}
	if err := (ContentLicense{Kind: ContentLicenseCustom, DisplayName: "Publisher terms", CustomText: "Reuse requires permission."}).Validate(); err != nil {
		t.Fatalf("custom license rejected: %v", err)
	}
	if err := (ContentLicense{Kind: ContentLicenseStandard, DisplayName: "Broken"}).Validate(); err == nil {
		t.Fatal("standard license without identifier accepted")
	}
	if err := ValidateContributorSnapshots([]ContributorSnapshot{{DisplayName: "Ada", Role: ContributorAuthor, Order: 1}}); err == nil {
		t.Fatal("non-contiguous attribution accepted")
	}
	if err := ValidateContributorSnapshots([]ContributorSnapshot{{DisplayName: "Ada", Role: ContributorAuthor, Order: 0, UserID: "not-an-id"}}); err == nil {
		t.Fatal("invalid contributor identifier accepted")
	}
}
