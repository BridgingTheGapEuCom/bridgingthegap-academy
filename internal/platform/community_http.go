package platform

import (
	"errors"
	"io"
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
type communityModerationProbeDTO struct {
	CanModerate bool `json:"canModerate"`
}
type communityModeratorPostDTO struct {
	PostID        string               `json:"postId"`
	Author        communityAuthorDTO   `json:"author"`
	Body          string               `json:"body"`
	State         community.Visibility `json:"state"`
	IsOpeningPost bool                 `json:"isOpeningPost"`
	CreatedAt     time.Time            `json:"createdAt"`
	UpdatedAt     time.Time            `json:"updatedAt"`
}
type communityModeratorThreadDTO struct {
	ThreadID  string                      `json:"threadId"`
	Title     string                      `json:"title"`
	Author    communityAuthorDTO          `json:"author"`
	State     community.Visibility        `json:"state"`
	CreatedAt time.Time                   `json:"createdAt"`
	UpdatedAt time.Time                   `json:"updatedAt"`
	Posts     []communityModeratorPostDTO `json:"posts"`
	PostTotal int                         `json:"postTotal"`
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
	case errors.Is(err, community.ErrOpeningPost):
		problemCode(w, r, http.StatusConflict, "Hide the thread to hide its opening post", "opening_post_requires_thread_moderation")
	default:
		problem(w, r, http.StatusInternalServerError, "Internal server error")
	}
}

func communityNoBody(w http.ResponseWriter, r *http.Request) bool {
	if r.Body == nil {
		return true
	}
	content, err := io.ReadAll(io.LimitReader(r.Body, 1))
	if err != nil || len(content) != 0 {
		problemCode(w, r, http.StatusBadRequest, "Unexpected request body", "unexpected_community_moderation_body")
		return false
	}
	return true
}

func (h *authHTTP) handleCommunityModerateThread(w http.ResponseWriter, r *http.Request) {
	if !communityNoBody(w, r) {
		return
	}
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	course, ok := communityCourseID(w, r)
	if !ok {
		return
	}
	thread, ok := communityUUIDParam(w, r, "threadId", "Invalid thread ID")
	if !ok {
		return
	}
	state := community.Visible
	if chi.URLParam(r, "action") == "hide" {
		state = community.Hidden
	}
	x, err := h.community.ModerateThread(r.Context(), course, string(actor.UserID()), thread, state)
	if err != nil {
		communityProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"threadId": x.ID, "state": string(x.State)})
}
func (h *authHTTP) handleCommunityModeratePost(w http.ResponseWriter, r *http.Request) {
	if !communityNoBody(w, r) {
		return
	}
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	course, ok := communityCourseID(w, r)
	if !ok {
		return
	}
	thread, ok := communityUUIDParam(w, r, "threadId", "Invalid thread ID")
	if !ok {
		return
	}
	post, ok := communityUUIDParam(w, r, "postId", "Invalid post ID")
	if !ok {
		return
	}
	state := community.Visible
	if chi.URLParam(r, "action") == "hide" {
		state = community.Hidden
	}
	x, err := h.community.ModeratePost(r.Context(), course, string(actor.UserID()), thread, post, state)
	if err != nil {
		communityProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"postId": x.ID, "state": string(x.State)})
}
func (h *authHTTP) handleCommunityModerationProbe(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	course, ok := communityCourseID(w, r)
	if !ok {
		return
	}
	can, err := h.community.CanModerate(r.Context(), course, string(actor.UserID()))
	if err != nil {
		communityProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, communityModerationProbeDTO{CanModerate: can})
}
func (h *authHTTP) handleCommunityModeratorThreads(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	course, ok := communityCourseID(w, r)
	if !ok {
		return
	}
	limit, offset, ok := communityPage(w, r)
	if !ok {
		return
	}
	threads, total, err := h.community.ModeratorThreads(r.Context(), course, string(actor.UserID()), limit, offset)
	if err != nil {
		communityProblem(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(threads))
	for _, t := range threads {
		out = append(out, map[string]any{"threadId": t.ID, "title": t.Title, "author": communityAuthorDTO{UserID: t.CreatedByUserID}, "state": t.State, "createdAt": t.CreatedAt, "updatedAt": t.UpdatedAt, "postCount": t.PostCount})
	}
	writeJSON(w, http.StatusOK, map[string]any{"threads": out, "total": total, "limit": limit, "offset": offset})
}
func (h *authHTTP) handleCommunityModeratorThread(w http.ResponseWriter, r *http.Request) {
	actor, ok := learnerActor(w, r)
	if !ok {
		return
	}
	course, ok := communityCourseID(w, r)
	if !ok {
		return
	}
	thread, ok := communityUUIDParam(w, r, "threadId", "Invalid thread ID")
	if !ok {
		return
	}
	limit, offset, ok := communityPage(w, r)
	if !ok {
		return
	}
	t, posts, total, err := h.community.ModeratorThread(r.Context(), course, string(actor.UserID()), thread, limit, offset)
	if err != nil {
		communityProblem(w, r, err)
		return
	}
	out := make([]communityModeratorPostDTO, 0, len(posts))
	for i, p := range posts {
		out = append(out, communityModeratorPostDTO{PostID: p.ID, Author: communityAuthorDTO{UserID: p.AuthorUserID}, Body: p.Body, State: p.State, IsOpeningPost: offset == 0 && i == 0, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt})
	}
	writeJSON(w, http.StatusOK, communityModeratorThreadDTO{ThreadID: t.ID, Title: t.Title, Author: communityAuthorDTO{UserID: t.CreatedByUserID}, State: t.State, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, Posts: out, PostTotal: total})
}
