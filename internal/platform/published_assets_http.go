package platform

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const publishedAssetCacheControl = "public, max-age=31536000, immutable"

// handlePublishedCourseAsset resolves binary data only through the immutable
// Courses binding for the requested exact public CourseVersion. It deliberately
// has no Authoring dependency or Asset metadata lookup.
func (a *authHTTP) handlePublishedCourseAsset(w http.ResponseWriter, r *http.Request) {
	courseID, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		publishedAssetNotFound(w, r)
		return
	}
	version, err := courses.ParseVersion(chi.URLParam(r, "version"))
	if err != nil {
		publishedAssetNotFound(w, r)
		return
	}
	assetKey, err := assets.ParseAssetID(chi.URLParam(r, "assetKey"))
	if err != nil {
		publishedAssetNotFound(w, r)
		return
	}
	binding, err := a.publishedAssets.Exact(r.Context(), courses.CourseID(courseID.String()), version, string(assetKey))
	if errors.Is(err, courses.ErrNotFound) {
		publishedAssetNotFound(w, r)
		return
	}
	if err != nil {
		publishedAssetUnavailable(w, r)
		return
	}

	stream, err := a.assetStorage.Open(r.Context(), assets.StorageObjectID(binding.StorageObjectID))
	if errors.Is(err, assets.ErrStorageObjectMissing) || errors.Is(err, assets.ErrInvalidStorageObject) {
		publishedAssetNotFound(w, r)
		return
	}
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		publishedAssetUnavailable(w, r)
		return
	}
	defer func() { _ = stream.Close() }()

	etag := `"sha256-` + binding.SHA256Digest + `"`
	disposition := "inline"
	if r.URL.Query().Get("download") == "1" || !publishedAssetInlineMediaType(binding.MediaType) {
		disposition = "attachment"
	}
	contentDisposition := mime.FormatMediaType(disposition, map[string]string{"filename": binding.OriginalFilename})
	if contentDisposition == "" {
		publishedAssetUnavailable(w, r)
		return
	}
	w.Header().Set("Cache-Control", publishedAssetCacheControl)
	w.Header().Set("Content-Type", binding.MediaType)
	w.Header().Set("Content-Length", strconv.FormatInt(binding.ByteSize, 10))
	w.Header().Set("Content-Disposition", contentDisposition)
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if publishedAssetETagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, stream)
}

func publishedAssetNotFound(w http.ResponseWriter, r *http.Request) {
	problem(w, r, http.StatusNotFound, "Not found")
}

func publishedAssetUnavailable(w http.ResponseWriter, r *http.Request) {
	problem(w, r, http.StatusServiceUnavailable, "Published asset unavailable")
}

func publishedAssetInlineMediaType(mediaType string) bool {
	switch mediaType {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/avif":
		return true
	default:
		return strings.HasPrefix(mediaType, "audio/") || strings.HasPrefix(mediaType, "video/")
	}
}

func publishedAssetETagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}
