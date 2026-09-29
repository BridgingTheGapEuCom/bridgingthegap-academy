package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
)

const previewWidgetLessonID = authoring.LessonID("22222222-2222-4222-8222-222222222222")

type draftPreviewWidgetLaunchFake struct {
	actor        identity.AuthenticatedActor
	draftID      authoring.DraftID
	lessonID     authoring.LessonID
	placementKey string
	calls        int
	err          error
}

func (f *draftPreviewWidgetLaunchFake) Prepare(_ context.Context, actor identity.AuthenticatedActor, draftID authoring.DraftID, lessonID authoring.LessonID, placementKey string) (plugins.RuntimeLaunch, error) {
	f.calls++
	f.actor, f.draftID, f.lessonID, f.placementKey = actor, draftID, lessonID, placementKey
	if f.err != nil {
		return plugins.RuntimeLaunch{}, f.err
	}
	return plugins.RuntimeLaunch{
		Context:             plugins.RuntimeContext{RuntimeInstanceID: "33333333-3333-4333-8333-333333333333", PluginID: "com.example.academy.preview", PluginVersion: "1.0.0", ArtifactDigest: strings.Repeat("a", 64), WidgetID: "timeline", WidgetType: plugins.TypeCourseWidget},
		Capabilities:        []string{plugins.CapabilityBootstrap, plugins.CapabilityContextRead, plugins.CapabilityDraftPreviewContextRead},
		DraftPreviewContext: &plugins.DraftPreviewWidgetRuntimeContext{ContextType: "DRAFT_PREVIEW", PlacementKey: "preview-widget", Configuration: []byte(`{"label":"safe"}`)},
	}, nil
}

func TestDraftPreviewWidgetLaunchUsesOnlyAuthorizedPlacementIdentity(t *testing.T) {
	launches := &draftPreviewWidgetLaunchFake{}
	auth := &authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, draftPreviewWidgetRuntime: launches}
	router := authTestRouter(auth)
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	csrf := authTestCSRFToken().Value()
	path := "/api/authoring/drafts/" + string(assetUploadDraftID) + "/lessons/" + string(previewWidgetLessonID) + "/blocks/preview-widget/widget-runtime"

	if response := authRequest(router, http.MethodPost, path, "", nil, csrf); response.Code != http.StatusUnauthorized || launches.calls != 0 {
		t.Fatalf("anonymous=%d calls=%d", response.Code, launches.calls)
	}
	if response := authRequest(router, http.MethodPost, path, `{"pluginId":"forged","configuration":{"admin":true}}`, cookie, csrf); response.Code != http.StatusBadRequest || launches.calls != 0 {
		t.Fatalf("body accepted=%d calls=%d", response.Code, launches.calls)
	}
	if response := authRequest(router, http.MethodPost, path, "", cookie); response.Code != http.StatusForbidden || launches.calls != 0 {
		t.Fatalf("CSRF bypass=%d calls=%d", response.Code, launches.calls)
	}
	bearer := httptest.NewRequest(http.MethodPost, path, nil)
	bearer.Header.Set("Authorization", "Bearer widget-runtime-token")
	bearer.Header.Set("Origin", "https://academy.example.com")
	bearer.Header.Set("X-CSRF-Token", csrf)
	bearerResponse := httptest.NewRecorder()
	router.ServeHTTP(bearerResponse, bearer)
	if bearerResponse.Code != http.StatusUnauthorized || launches.calls != 0 {
		t.Fatalf("runtime bearer launched=%d calls=%d", bearerResponse.Code, launches.calls)
	}

	response := authRequest(router, http.MethodPost, path, "", cookie, csrf)
	if response.Code != http.StatusOK || launches.calls != 1 || launches.draftID != assetUploadDraftID || launches.lessonID != previewWidgetLessonID || launches.placementKey != "preview-widget" || launches.actor.UserID() == "" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("launch=%d calls=%d coordinates=%q/%q/%q body=%s", response.Code, launches.calls, launches.draftID, launches.lessonID, launches.placementKey, response.Body.String())
	}
	for _, forbidden := range []string{"userId", "email", "plugins.manage", "authoring.content.edit", plugins.CapabilityCourseContextRead, plugins.CapabilityDashboardContextRead} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("launch exposed %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestDraftPreviewWidgetLaunchHidesUnavailablePlacement(t *testing.T) {
	launches := &draftPreviewWidgetLaunchFake{err: authoring.ErrNotFound}
	router := authTestRouter(&authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, draftPreviewWidgetRuntime: launches})
	path := "/api/authoring/drafts/" + string(assetUploadDraftID) + "/lessons/" + string(previewWidgetLessonID) + "/blocks/missing/widget-runtime"
	response := authRequest(router, http.MethodPost, path, "", &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}, authTestCSRFToken().Value())
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "approval") || strings.Contains(response.Body.String(), "digest") {
		t.Fatalf("unavailable=%d body=%s", response.Code, response.Body.String())
	}
}

func TestExactDraftPreviewPlacementRejectsNonWidgetAndWrongLesson(t *testing.T) {
	textLesson := authoring.DraftLesson{
		ID: previewWidgetLessonID,
		LessonInput: authoring.LessonInput{
			DraftID: assetUploadDraftID,
			Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{
				Key: "text-block", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{},
			}}},
		},
	}
	if _, err := exactDraftPreviewPlacement(textLesson, "text-block"); !errors.Is(err, plugins.ErrLaunchDenied) {
		t.Fatalf("non-widget placement=%v", err)
	}
	repository := authoringHTTPRepository{lessons: map[authoring.LessonID]authoring.DraftLesson{previewWidgetLessonID: textLesson}}
	source := &authoringDraftPreviewPlacementSource{repository: repository}
	if _, err := source.DraftPreviewPlacement(context.Background(), string(assetUploadDraftID), "99999999-9999-4999-8999-999999999999", "text-block"); !errors.Is(err, plugins.ErrLaunchDenied) {
		t.Fatalf("wrong Lesson=%v", err)
	}
}

func TestDraftPreviewPlacementSourceRequiresExactDraftAuthorizationAndOwnership(t *testing.T) {
	actor, err := identity.ActorFromResolvedSession(loginTestCurrent(t))
	if err != nil {
		t.Fatal(err)
	}
	placement := courses.PluginWidgetBlockPayload{
		PluginID: "com.example.preview", PluginVersion: "1.0.0", ArtifactDigest: strings.Repeat("a", 64),
		WidgetID: "timeline", WidgetType: "COURSE_WIDGET", Configuration: []byte(`{"label":"safe"}`),
	}
	lesson := authoring.DraftLesson{ID: previewWidgetLessonID, LessonInput: authoring.LessonInput{
		DraftID: assetUploadDraftID,
		Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{
			Key: "preview-widget", Type: courses.BlockPluginWidget, Payload: placement,
		}}},
	}}
	repository := authoringHTTPRepository{lessons: map[authoring.LessonID]authoring.DraftLesson{previewWidgetLessonID: lesson}}
	memberships := authoringMembershipsFake{roles: map[authoring.DraftID]authoring.MemberRole{assetUploadDraftID: authoring.MemberAuthor}}
	source := &authoringDraftPreviewPlacementSource{
		reads:      authoring.NewReadService(repository, authoring.NewAuthorizationService(memberships)),
		repository: repository,
	}
	if got, err := source.authorizedPlacement(context.Background(), actor, assetUploadDraftID, previewWidgetLessonID, "preview-widget"); err != nil || got.PluginID != placement.PluginID {
		t.Fatalf("authorized placement=%#v err=%v", got, err)
	}
	otherDraft := authoring.DraftID("44444444-4444-4444-8444-444444444444")
	if _, err := source.authorizedPlacement(context.Background(), actor, otherDraft, previewWidgetLessonID, "preview-widget"); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("cross-Draft placement=%v", err)
	}
	delete(memberships.roles, assetUploadDraftID)
	if _, err := source.authorizedPlacement(context.Background(), actor, assetUploadDraftID, previewWidgetLessonID, "preview-widget"); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("inaccessible Draft placement=%v", err)
	}
}
