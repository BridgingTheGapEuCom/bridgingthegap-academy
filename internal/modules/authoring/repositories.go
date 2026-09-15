package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

var (
	ErrNotFound         = errors.New("authoring record not found")
	ErrConflict         = errors.New("authoring conflict")
	ErrRevisionMismatch = errors.New("authoring revision mismatch")
	ErrInvalidState     = errors.New("invalid draft state")
	ErrInvalidPatch     = errors.New("invalid draft metadata patch")
	ErrInvalidStructure = errors.New("invalid authoring structure mutation")
)

// Repository is Authoring-owned. All mutators are explicit; published Courses
// rows are never updated here. Expected revisions protect stale writes.
type Repository interface {
	CreateDraft(context.Context, DraftMetadata, string) (CourseDraft, AuthoringWorkspace, error)
	GetDraft(context.Context, DraftID) (CourseDraft, error)
	GetWorkspace(context.Context, DraftID) (AuthoringWorkspace, error)
	UpdateDraftMetadata(context.Context, DraftID, int64, DraftMetadata) (CourseDraft, error)
	AbandonDraft(context.Context, DraftID, int64) (CourseDraft, error)
	ListMembers(context.Context, WorkspaceID) ([]WorkspaceMember, error)
	ActiveMembershipForDraft(context.Context, DraftID, string) (MemberRole, bool, error)
	CreateModule(context.Context, ModuleInput) (DraftModule, error)
	GetModule(context.Context, ModuleID) (DraftModule, error)
	ListModules(context.Context, DraftID) ([]DraftModule, error)
	UpdateModule(context.Context, ModuleID, int64, string, string) (DraftModule, error)
	ReorderModules(context.Context, DraftID, int64, []ModuleID) (CourseDraft, error)
	DeleteModule(context.Context, ModuleID, int64) error
	CreateModuleAtPosition(context.Context, DraftID, int64, ModuleInput) (DraftModule, CourseDraft, error)
	UpdateModuleMetadata(context.Context, DraftID, ModuleID, int64, DraftModulePatch) (DraftModule, CourseDraft, error)
	DeleteEmptyModule(context.Context, DraftID, ModuleID, int64, int64) (CourseDraft, error)
	CreateLessonAtPosition(context.Context, DraftID, ModuleID, int64, LessonInput) (DraftLesson, CourseDraft, error)
	UpdateLessonMetadataForDraft(context.Context, DraftID, LessonID, int64, DraftLessonPatch) (DraftLesson, CourseDraft, error)
	ReplaceLessonContentForDraft(context.Context, DraftID, LessonID, int64, courses.LessonContent) (DraftLesson, CourseDraft, error)
	ReorderLessonsForDraft(context.Context, DraftID, int64, []ModuleLessonOrder) (CourseDraft, error)
	ReplaceLessonPrerequisitesForDraft(context.Context, DraftID, LessonID, int64, []string) (DraftLesson, CourseDraft, error)
	DeleteLessonForDraft(context.Context, DraftID, LessonID, int64, int64) (CourseDraft, error)
	AddMemberForDraft(context.Context, DraftID, int64, string, MemberRole) (WorkspaceMember, CourseDraft, error)
	ChangeMemberRoleForDraft(context.Context, DraftID, int64, string, MemberRole) (WorkspaceMember, CourseDraft, error)
	RevokeMemberForDraft(context.Context, DraftID, int64, string) (WorkspaceMember, CourseDraft, error)
	CreateLesson(context.Context, LessonInput) (DraftLesson, error)
	GetLesson(context.Context, LessonID) (DraftLesson, error)
	ListLessons(context.Context, ModuleID) ([]DraftLesson, error)
	ListLessonsForDraft(context.Context, DraftID) ([]DraftLesson, error)
	UpdateLessonMetadata(context.Context, LessonID, int64, string, string, []string, *int) (DraftLesson, error)
	UpdateLessonContent(context.Context, LessonID, int64, courses.LessonContent) (DraftLesson, error)
	ReorderLessons(context.Context, ModuleID, int64, []LessonID) (DraftModule, error)
	MoveLesson(context.Context, LessonID, int64, ModuleID, int) (DraftLesson, error)
	DeleteLesson(context.Context, LessonID, int64) error
	ReplacePrerequisites(context.Context, LessonID, int64, []string) (DraftLesson, error)
	ListPrerequisites(context.Context, LessonID) ([]Prerequisite, error)
	ListPrerequisitesForDraft(context.Context, DraftID) ([]Prerequisite, error)
}

// ReadRepository is the narrow Authoring-owned read contract used by private
// draft serving. It deliberately contains no membership role interpretation;
// resource-scoped decisions remain with Authorizer.
// Snapshot reads keep outline and prerequisites consistent with the returned revisions.
type ReadRepository interface {
	GetDraft(context.Context, DraftID) (CourseDraft, error)
	GetWorkspace(context.Context, DraftID) (AuthoringWorkspace, error)
	ActiveMembers(context.Context, DraftID) ([]WorkspaceMember, error)
	ReadStructure(context.Context, DraftID) ([]ModuleStructure, error)
	ReadLesson(context.Context, DraftID, LessonID) (DraftLesson, []Prerequisite, error)
}
