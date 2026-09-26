package platform

import (
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"net/http"
	"strconv"
)

type authoringAssetSummaryDTO struct {
	AssetKey  string `json:"assetKey"`
	Filename  string `json:"filename"`
	MediaType string `json:"mediaType"`
	ByteSize  int64  `json:"byteSize"`
	CreatedAt string `json:"createdAt"`
}
type authoringAssetListDTO struct {
	Items  []authoringAssetSummaryDTO `json:"items"`
	Limit  int                        `json:"limit"`
	Offset int                        `json:"offset"`
	Total  int                        `json:"total"`
}

func (h *authHTTP) handleAuthoringAssetList(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	if h.authoringAssets == nil {
		problem(w, r, http.StatusInternalServerError, "Asset listing unavailable")
		return
	}
	limit, offset, err := authoringAssetListQuery(r)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid asset query")
		return
	}
	page, err := h.authoringAssets.List(r.Context(), actor, draftID, limit, offset)
	if err != nil {
		switch {
		case errors.Is(err, authoring.ErrNotFound), errors.Is(err, authoring.ErrAuthorizationDenied):
			problem(w, r, http.StatusNotFound, "Not found")
		case errors.Is(err, assets.ErrInvalidAsset):
			problem(w, r, http.StatusBadRequest, "Invalid asset query")
		default:
			problem(w, r, http.StatusInternalServerError, "Asset listing failed")
		}
		return
	}
	out := authoringAssetListDTO{Items: make([]authoringAssetSummaryDTO, 0, len(page.Items)), Limit: page.Limit, Offset: page.Offset, Total: page.Total}
	for _, a := range page.Items {
		out.Items = append(out.Items, authoringAssetSummaryDTO{AssetKey: string(a.ID), Filename: a.OriginalFilename, MediaType: a.MediaType, ByteSize: a.ByteSize, CreatedAt: a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")})
	}
	writeJSON(w, http.StatusOK, out)
}
func authoringAssetListQuery(r *http.Request) (int, int, error) {
	values := r.URL.Query()
	limit, offset := 0, 0
	for _, item := range []struct {
		name    string
		target  *int
		minimum int
	}{{"limit", &limit, 1}, {"offset", &offset, 0}} {
		raw, ok := values[item.name]
		if !ok {
			continue
		}
		if len(raw) != 1 || raw[0] == "" {
			return 0, 0, errors.New("invalid")
		}
		value, err := strconv.Atoi(raw[0])
		if err != nil || value < item.minimum {
			return 0, 0, errors.New("invalid asset query")
		}
		*item.target = value
	}
	return limit, offset, nil
}
