package platform

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/community"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxCommunityBodyBytes int64 = 32 * 1024
const defaultCommunityPageLimit = 20
const maxCommunityPageLimit = 100

// These are participant-facing DTOs. Author IDs are opaque stable references;
// they deliberately avoid serializing Identity accounts, roles, or email.
type communityMetadataDTO struct {
	CourseID string         `json:"courseId"`
	Mode     community.Mode `json:"mode"`
}
type communityAuthorDTO struct {
	UserID string `json:"userId"`
}
type communityThreadSummaryDTO struct {
	ThreadID  string             `json:"threadId"`
	Title     string             `json:"title"`
	Author    communityAuthorDTO `json:"author"`
	PostCount int                `json:"postCount"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
}
type communityPostDTO struct {
	PostID    string             `json:"postId"`
	Author    communityAuthorDTO `json:"author"`
	Body      string             `json:"body"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
}
type communityThreadDTO struct {
	ThreadID  string             `json:"threadId"`
	Title     string             `json:"title"`
	Author    communityAuthorDTO `json:"author"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
	Posts     []communityPostDTO `json:"posts"`
	PostTotal int                `json:"postTotal"`
}
type communityThreadPageDTO struct {
	Threads []communityThreadSummaryDTO `json:"threads"`
	Total   int                         `json:"total"`
	Limit   int                         `json:"limit"`
	Offset  int                         `json:"offset"`
}
type communityCreateThreadRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}
type communityCreatePostRequest struct {
	Body string `json:"body"`
}

func (h *authHTTP) handleCommunity(w http.ResponseWriter, r *http.Request) {
	courseID, ok := communityCourseID(w, r)
	if !ok {
		return
	}
	x, err := h.community.GetCommunity(r.Context(), courseID)
	if err != nil {
		communityProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, communityMetadataDTO{CourseID: x.CourseID, Mode: x.Mode})
}
func (h *authHTTP) handleCommunityThreads(w http.ResponseWriter, r *http.Request) {
	courseID, ok := communityCourseID(w, r)
	if !ok {
		return
	}
	limit, offset, ok := communityPage(w, r)
	if !ok {
		return
	}
	threads, total, err := h.community.ListThreads(r.Context(), courseID, limit, offset)
	if err != nil {
		communityProblem(w, r, err)
		return
	}
	result := make([]communityThreadSummaryDTO, 0, len(threads))
	for _, x := range threads {
		result = append(result, communityThreadSummary(x))
	}
	writeJSON(w, http.StatusOK, communityThreadPageDTO{Threads: result, Total: total, Limit: limit, Offset: offset})
}
func (h *authHTTP) handleCommunityThread(w http.ResponseWriter, r *http.Request) {
	courseID, ok := communityCourseID(w, r)
	if !ok {
		return
	}
	threadID, ok := communityUUIDParam(w, r, "threadId", "Invalid thread ID")
	if !ok {
		return
	}
	limit, offset, ok := communityPage(w, r)
	if !ok {
		return
	}
	t, posts, total, err := h.community.GetThread(r.Context(), courseID, threadID, limit, offset)
	if err != nil {
		communityProblem(w, r, err)
		return
	}
	result := make([]communityPostDTO, 0, len(posts))
	for _, p := range posts {
		result = append(result, communityPost(p))
	}
	writeJSON(w, http.StatusOK, communityThreadDTO{ThreadID: t.ID, Title: t.Title, Author: communityAuthorDTO{UserID: t.CreatedByUserID}, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, Posts: result, PostTotal: total})
}
func (h *authHTTP) handleCommunityCreateThread(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	courseID, ok := communityCourseID(w, r)
	if !ok {
		return
	}
	var input communityCreateThreadRequest
	if err := decodeAuthoringBody(w, r, maxCommunityBodyBytes, &input); err != nil {
		problemCode(w, r, http.StatusBadRequest, "Invalid community thread", "invalid_community_thread")
		return
	}
	t, p, err := h.community.CreateThread(r.Context(), community.ThreadInput{CourseID: courseID, Title: input.Title, CreatedByUserID: string(actor.UserID()), OpeningBody: input.Body})
	if err != nil {
		communityProblem(w, r, err)
		return
	}
	w.Header().Set("Location", r.URL.Path+"/"+t.ID)
	writeJSON(w, http.StatusCreated, communityThreadDTO{ThreadID: t.ID, Title: t.Title, Author: communityAuthorDTO{UserID: t.CreatedByUserID}, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, Posts: []communityPostDTO{communityPost(p)}, PostTotal: 1})
}
func (h *authHTTP) handleCommunityCreatePost(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	courseID, ok := communityCourseID(w, r)
	if !ok {
		return
	}
	threadID, ok := communityUUIDParam(w, r, "threadId", "Invalid thread ID")
	if !ok {
		return
	}
	var input communityCreatePostRequest
	if err := decodeAuthoringBody(w, r, maxCommunityBodyBytes, &input); err != nil {
		problemCode(w, r, http.StatusBadRequest, "Invalid community post", "invalid_community_post")
		return
	}
	p, err := h.community.CreatePost(r.Context(), community.PostInput{CourseID: courseID, ThreadID: threadID, AuthorUserID: string(actor.UserID()), Body: input.Body})
	if err != nil {
		communityProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, communityPost(p))
}
func communityCourseID(w http.ResponseWriter, r *http.Request) (string, bool) {
	return communityUUIDParam(w, r, "courseId", "Invalid course ID")
}
func communityUUIDParam(w http.ResponseWriter, r *http.Request, name, message string) (string, bool) {
	raw := chi.URLParam(r, name)
	id, err := uuid.Parse(raw)
	if err != nil || id.String() != raw {
		problem(w, r, http.StatusBadRequest, message)
		return "", false
	}
	return id.String(), true
}
func communityPage(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	limit, offset := defaultCommunityPageLimit, 0
	var err error
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > maxCommunityPageLimit {
			problemCode(w, r, http.StatusBadRequest, "Invalid pagination", "invalid_pagination")
			return 0, 0, false
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			problemCode(w, r, http.StatusBadRequest, "Invalid pagination", "invalid_pagination")
			return 0, 0, false
		}
	}
	return limit, offset, true
}
func communityThreadSummary(x community.ThreadSummary) communityThreadSummaryDTO {
	return communityThreadSummaryDTO{ThreadID: x.ID, Title: x.Title, Author: communityAuthorDTO{UserID: x.CreatedByUserID}, PostCount: x.PostCount, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt}
}
func communityPost(x community.Post) communityPostDTO {
	return communityPostDTO{PostID: x.ID, Author: communityAuthorDTO{UserID: x.AuthorUserID}, Body: x.Body, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt}
}
func communityProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, community.ErrNotFound):
		problem(w, r, http.StatusNotFound, "Community resource not found")
	case errors.Is(err, community.ErrDisabled):
		problemCode(w, r, http.StatusConflict, "Community is disabled", "community_disabled")
	case errors.Is(err, community.ErrInvalid):
		problemCode(w, r, http.StatusBadRequest, "Invalid community content", "invalid_community_content")
	default:
		problem(w, r, http.StatusInternalServerError, "Internal server error")
	}
}
