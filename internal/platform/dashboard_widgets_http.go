package platform

import (
	"encoding/json"
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
	"github.com/go-chi/chi/v5"
	"net/http"
)

const maxDashboardWidgetBody = 24 << 10

type dashboardWidgetAPI struct {
	placements *plugins.DashboardPlacementService
	registry   *plugins.RegistryService
}

func (h *dashboardWidgetAPI) available(w http.ResponseWriter, r *http.Request) {
	releases, err := h.registry.DashboardWidgets(r.Context())
	if err != nil {
		dashboardProblem(w, r, err)
		return
	}
	items := make([]map[string]any, 0)
	for _, release := range releases {
		for _, entry := range release.Manifest.Entrypoints {
			if entry.Type == plugins.TypeDashboardWidget {
				items = append(items, map[string]any{"pluginId": release.Release.PluginID, "pluginName": release.Manifest.Name, "pluginVersion": release.Release.Version, "artifactDigest": release.Release.ArtifactDigest, "widgetId": entry.ID, "widgetName": entry.Name, "description": release.Manifest.Description})
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"widgets": items})
}

type dashboardCreateRequest struct {
	PluginID       string          `json:"pluginId"`
	PluginVersion  string          `json:"pluginVersion"`
	ArtifactDigest string          `json:"artifactDigest"`
	WidgetID       string          `json:"widgetId"`
	Configuration  json.RawMessage `json:"configuration"`
}
type dashboardUpdateRequest struct {
	ExpectedRevision int64           `json:"expectedRevision"`
	Configuration    json.RawMessage `json:"configuration"`
	Enabled          bool            `json:"enabled"`
}
type dashboardMoveRequest struct {
	ExpectedRevisions map[string]int64 `json:"expectedRevisions"`
}
type dashboardDeleteRequest struct {
	ExpectedRevision int64 `json:"expectedRevision"`
}

func (h *dashboardWidgetAPI) list(w http.ResponseWriter, r *http.Request) {
	values, e := h.placements.List(r.Context())
	if e != nil {
		dashboardProblem(w, r, e)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"widgets": values})
}
func (h *dashboardWidgetAPI) create(w http.ResponseWriter, r *http.Request) {
	var in dashboardCreateRequest
	if decodeAuthoringBody(w, r, maxDashboardWidgetBody, &in) != nil {
		problem(w, r, 400, "Invalid request")
		return
	}
	value, e := h.placements.Create(r.Context(), plugins.DashboardPlacement{PluginID: plugins.PluginID(in.PluginID), PluginVersion: in.PluginVersion, ArtifactDigest: in.ArtifactDigest, WidgetID: in.WidgetID, Configuration: in.Configuration})
	if e != nil {
		dashboardProblem(w, r, e)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 201, value)
}
func (h *dashboardWidgetAPI) update(w http.ResponseWriter, r *http.Request) {
	var in dashboardUpdateRequest
	if decodeAuthoringBody(w, r, maxDashboardWidgetBody, &in) != nil {
		problem(w, r, 400, "Invalid request")
		return
	}
	id := chi.URLParam(r, "placementId")
	existing, e := h.placements.List(r.Context())
	if e != nil {
		dashboardProblem(w, r, e)
		return
	}
	for _, v := range existing {
		if v.ID == id {
			v.Configuration = in.Configuration
			v.Enabled = in.Enabled
			out, e := h.placements.Update(r.Context(), v, in.ExpectedRevision)
			if e != nil {
				dashboardProblem(w, r, e)
				return
			}
			writeJSON(w, 200, out)
			return
		}
	}
	problem(w, r, 404, "Not found")
}
func (h *dashboardWidgetAPI) move(w http.ResponseWriter, r *http.Request, delta int) {
	var in dashboardMoveRequest
	if decodeAuthoringBody(w, r, maxDashboardWidgetBody, &in) != nil {
		problem(w, r, 400, "Invalid request")
		return
	}
	values, e := h.placements.List(r.Context())
	if e != nil {
		dashboardProblem(w, r, e)
		return
	}
	index := -1
	for i, v := range values {
		if v.ID == chi.URLParam(r, "placementId") {
			index = i
		}
	}
	if index < 0 {
		problem(w, r, 404, "Not found")
		return
	}
	next := index + delta
	if next >= 0 && next < len(values) {
		values[index], values[next] = values[next], values[index]
	}
	ids := make([]string, len(values))
	for i, v := range values {
		ids[i] = v.ID
	}
	out, e := h.placements.Reorder(r.Context(), ids, in.ExpectedRevisions)
	if e != nil {
		dashboardProblem(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"widgets": out})
}
func (h *dashboardWidgetAPI) delete(w http.ResponseWriter, r *http.Request) {
	var in dashboardDeleteRequest
	if decodeAuthoringBody(w, r, maxDashboardWidgetBody, &in) != nil {
		problem(w, r, 400, "Invalid request")
		return
	}
	if e := h.placements.Delete(r.Context(), chi.URLParam(r, "placementId"), in.ExpectedRevision); e != nil {
		dashboardProblem(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func dashboardProblem(w http.ResponseWriter, r *http.Request, e error) {
	if errors.Is(e, plugins.ErrDashboardPlacementConflict) {
		problem(w, r, 409, "Dashboard changed")
		return
	}
	if errors.Is(e, plugins.ErrDashboardPlacementNotFound) || errors.Is(e, plugins.ErrNotFound) {
		problem(w, r, 404, "Not found")
		return
	}
	if errors.Is(e, plugins.ErrLaunchDenied) {
		problem(w, r, 400, "Invalid dashboard widget")
		return
	}
	problem(w, r, 500, "Dashboard unavailable")
}
