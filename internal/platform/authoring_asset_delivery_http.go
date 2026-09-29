package platform

import (
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/go-chi/chi/v5"
)

const draftAssetCacheControl = "private, no-cache"

func (h *authHTTP) handleAuthoringDraftAsset(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	assetID, err := assets.ParseAssetID(chi.URLParam(r, "assetId"))
	if err != nil {
		authoringDraftAssetNotFound(w, r)
		return
	}
	asset, err := h.authoringAssetDelivery.Exact(r.Context(), actor, draftID, assetID)
	if errors.Is(err, authoring.ErrNotFound) || errors.Is(err, authoring.ErrAuthorizationDenied) {
		authoringDraftAssetNotFound(w, r)
		return
	}
	if err != nil {
		authoringDraftAssetUnavailable(w, r)
		return
	}

	seekableStorage, ok := h.assetStorage.(assets.SeekableBinaryStorage)
	if !ok {
		authoringDraftAssetUnavailable(w, r)
		return
	}
	stream, err := seekableStorage.OpenSeekable(r.Context(), asset.StorageObjectID)
	if errors.Is(err, assets.ErrStorageObjectMissing) || errors.Is(err, assets.ErrInvalidStorageObject) {
		authoringDraftAssetNotFound(w, r)
		return
	}
	if err != nil {
		if r.Context().Err() == nil {
			authoringDraftAssetUnavailable(w, r)
		}
		return
	}
	defer func() { _ = stream.Close() }()
	actualSize, seekErr := stream.Seek(0, io.SeekEnd)
	if seekErr != nil || actualSize != asset.ByteSize {
		authoringDraftAssetUnavailable(w, r)
		return
	}
	if _, seekErr = stream.Seek(0, io.SeekStart); seekErr != nil {
		authoringDraftAssetUnavailable(w, r)
		return
	}

	disposition := "inline"
	if r.URL.Query().Get("download") == "1" || !publishedAssetInlineMediaType(asset.MediaType) {
		disposition = "attachment"
	}
	contentDisposition := mime.FormatMediaType(disposition, map[string]string{"filename": asset.OriginalFilename})
	if contentDisposition == "" {
		authoringDraftAssetUnavailable(w, r)
		return
	}
	etag := `"sha256-` + string(asset.SHA256Digest) + `"`
	w.Header().Set("Cache-Control", draftAssetCacheControl)
	w.Header().Set("Content-Type", asset.MediaType)
	w.Header().Set("Content-Disposition", contentDisposition)
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if publishedAssetETagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	http.ServeContent(w, r, asset.OriginalFilename, asset.CreatedAt, stream)
}

func authoringDraftAssetNotFound(w http.ResponseWriter, r *http.Request) {
	problem(w, r, http.StatusNotFound, "Not found")
}

func authoringDraftAssetUnavailable(w http.ResponseWriter, r *http.Request) {
	problem(w, r, http.StatusServiceUnavailable, "Draft asset unavailable")
}
