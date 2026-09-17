package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

var ErrInvalidDraftCreation = errors.New("invalid draft creation")

// DraftCreationInput contains the complete metadata that the current Draft
// aggregate requires at creation. Course identity and creator membership are
// deliberately server-owned.
type DraftCreationInput struct {
	IntendedVersion    courses.Version
	SourceLanguage     courses.LanguageTag
	Title              string
	Description        string
	LearningObjectives []string
	Changelog          string
}

func (i DraftCreationInput) Validate() error {
	// A syntactically valid identity lets the existing complete DraftMetadata
	// validator remain the single authority for creation metadata. The storage
	// boundary replaces it with the generated immutable Course identity.
	metadata := DraftMetadata{
		CourseID:           courses.CourseID("00000000-0000-4000-8000-000000000001"),
		IntendedVersion:    i.IntendedVersion,
		SourceLanguage:     i.SourceLanguage,
		Title:              i.Title,
		Description:        i.Description,
		LearningObjectives: append([]string(nil), i.LearningObjectives...),
		Changelog:          i.Changelog,
		License:            courses.ContentLicense{Kind: courses.ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"},
	}
	return metadata.Validate()
}

type DraftCreationRepository interface {
	CreateDraftForCreator(context.Context, DraftCreationInput, string) (CourseDraft, error)
}

// DraftCreationService creates the first Draft for any authenticated actor.
// It intentionally does not ask for a Draft-scoped capability: no Draft exists
// yet. The repository owns the atomic Draft, workspace, and MAINTAINER write.
type DraftCreationService struct {
	repository DraftCreationRepository
}

func NewDraftCreationService(repository DraftCreationRepository) *DraftCreationService {
	return &DraftCreationService{repository: repository}
}

func (s *DraftCreationService) Create(ctx context.Context, actor identity.AuthenticatedActor, input DraftCreationInput) (CourseDraft, error) {
	if s == nil || s.repository == nil || actor.UserID() == "" || actor.SessionID() == "" || input.Validate() != nil {
		return CourseDraft{}, ErrInvalidDraftCreation
	}
	input.LearningObjectives = append([]string(nil), input.LearningObjectives...)
	return s.repository.CreateDraftForCreator(ctx, input, string(actor.UserID()))
}
