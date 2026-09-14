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
)

// Repository is Authoring-owned. All mutators are explicit; published Courses
// rows are never updated here. Expected revisions protect stale writes.
type Repository interface {
	CreateDraft(context.Context, DraftMetadata, string) (CourseDraft, AuthoringWorkspace, error)
	GetDraft(context.Context, DraftID) (CourseDraft, error)
	GetWorkspace(context.Context, DraftID) (AuthoringWorkspace, error)
	UpdateDraftMetadata(context.Context, DraftID, int64, DraftMetadata) (CourseDraft, error)
	AbandonDraft(context.Context, DraftID, int64) (CourseDraft, error)
	AddMember(context.Context, WorkspaceID, string, MemberRole) (WorkspaceMember, error)
	RevokeMember(context.Context, WorkspaceID, string) (WorkspaceMember, error)
	ListMembers(context.Context, WorkspaceID) ([]WorkspaceMember, error)
	CreateModule(context.Context, ModuleInput) (DraftModule, error)
	GetModule(context.Context, ModuleID) (DraftModule, error)
	ListModules(context.Context, DraftID) ([]DraftModule, error)
	UpdateModule(context.Context, ModuleID, int64, string, string) (DraftModule, error)
	ReorderModules(context.Context, DraftID, int64, []ModuleID) (CourseDraft, error)
	DeleteModule(context.Context, ModuleID, int64) error
	CreateLesson(context.Context, LessonInput) (DraftLesson, error)
	GetLesson(context.Context, LessonID) (DraftLesson, error)
	ListLessons(context.Context, ModuleID) ([]DraftLesson, error)
	UpdateLessonMetadata(context.Context, LessonID, int64, string, string, []string, *int) (DraftLesson, error)
	UpdateLessonContent(context.Context, LessonID, int64, courses.LessonContent) (DraftLesson, error)
	ReorderLessons(context.Context, ModuleID, int64, []LessonID) (DraftModule, error)
	MoveLesson(context.Context, LessonID, int64, ModuleID, int) (DraftLesson, error)
	DeleteLesson(context.Context, LessonID, int64) error
	ReplacePrerequisites(context.Context, LessonID, int64, []string) (DraftLesson, error)
	ListPrerequisites(context.Context, LessonID) ([]Prerequisite, error)
}
