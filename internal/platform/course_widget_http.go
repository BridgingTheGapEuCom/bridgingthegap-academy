package platform

import (
	"context"
	"net/http"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
	"github.com/go-chi/chi/v5"
)

type courseWidgetDiscoveryAdapter struct{ registry *plugins.RegistryService }

func (a courseWidgetDiscoveryAdapter) DiscoverCourseWidgets(ctx context.Context) ([]authoring.CourseWidgetDescriptor, error) {
	entries, err := a.registry.DiscoverCourseWidgets(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]authoring.CourseWidgetDescriptor, 0, len(entries))
	for _, entry := range entries {
		result = append(result, authoring.CourseWidgetDescriptor{
			PluginID: string(entry.PluginID), PluginName: entry.PluginName, PluginVersion: entry.PluginVersion,
			ArtifactDigest: entry.ArtifactDigest, WidgetID: entry.WidgetID, WidgetName: entry.WidgetName,
			Description: entry.Description, Trust: string(entry.Trust),
		})
	}
	return result, nil
}

type courseWidgetDiscoveryDTO struct {
	PluginID       string `json:"pluginId"`
	PluginName     string `json:"pluginName"`
	PluginVersion  string `json:"pluginVersion"`
	ArtifactDigest string `json:"artifactDigest"`
	WidgetID       string `json:"widgetId"`
	WidgetName     string `json:"widgetName"`
	Description    string `json:"description"`
	Trust          string `json:"trust"`
}

func (a *authHTTP) handleAuthoringCourseWidgets(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	entries, err := a.authoringCourseWidgets.List(r.Context(), actor, draftID)
	if err != nil {
		authoringStructureMutationProblem(w, r, err)
		return
	}
	result := make([]courseWidgetDiscoveryDTO, 0, len(entries))
	for _, entry := range entries {
		result = append(result, courseWidgetDiscoveryDTO{PluginID: entry.PluginID, PluginName: entry.PluginName, PluginVersion: entry.PluginVersion, ArtifactDigest: entry.ArtifactDigest, WidgetID: entry.WidgetID, WidgetName: entry.WidgetName, Description: entry.Description, Trust: entry.Trust})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"widgets": result})
}

type courseWidgetLaunch interface {
	Prepare(context.Context, courses.CourseID, courses.Version, string, string) (plugins.RuntimeLaunch, error)
}

func (a *authHTTP) handleCourseWidgetRuntimeLaunch(w http.ResponseWriter, r *http.Request) {
	if a.courseWidgetRuntime == nil {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	courseID, ok := publishedCourseID(w, r)
	if !ok {
		return
	}
	version, err := courses.ParseVersion(chi.URLParam(r, "version"))
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid course version")
		return
	}
	launch, err := a.courseWidgetRuntime.Prepare(r.Context(), courseID, version, chi.URLParam(r, "lessonKey"), chi.URLParam(r, "blockKey"))
	if err != nil {
		if plugins.IsCourseWidgetLaunchUnavailable(err) {
			problem(w, r, http.StatusNotFound, "Widget unavailable")
			return
		}
		problem(w, r, http.StatusServiceUnavailable, "Widget unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, launch)
}
