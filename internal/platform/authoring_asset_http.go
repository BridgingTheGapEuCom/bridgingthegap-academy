package platform

import (
	"errors"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
)

const authoringAssetMultipartOverheadBytes int64 = 64 * 1024

type authoringAssetDTO struct {
	AssetKey  string           `json:"assetKey"`
	Filename  string           `json:"filename"`
	MediaType string           `json:"mediaType"`
	ByteSize  int64            `json:"byteSize"`
	Status    assets.Lifecycle `json:"status"`
}

func (h *authHTTP) handleAuthoringAssetUpload(w http.ResponseWriter, r *http.Request) {
	draftID, actor, ok := authoringRequest(w, r)
	if !ok {
		return
	}
	if h.authoringAssetUploads == nil || h.assetMaxBytes <= 0 {
		problem(w, r, http.StatusInternalServerError, "Asset upload unavailable")
		return
	}
	upload, err := h.authoringAssetUploads.AuthorizeUpload(r.Context(), actor, draftID)
	if err != nil {
		authoringAssetProblem(w, r, err)
		return
	}

	filename, content, err := singleAssetMultipart(w, r, h.assetMaxBytes)
	if err != nil {
		authoringAssetProblem(w, r, err)
		return
	}
	defer func() { _ = content.Close() }()
	if assets.ValidateOriginalFilename(filename) != nil {
		authoringAssetProblem(w, r, assets.ErrInvalidAsset)
		return
	}
	asset, err := upload(r.Context(), filename, content)
	if err != nil {
		authoringAssetProblem(w, r, err)
		return
	}
	if asset.Lifecycle != assets.LifecycleAvailable || asset.Validate() != nil {
		problem(w, r, http.StatusInternalServerError, "Asset upload failed")
		return
	}
	writeJSON(w, http.StatusCreated, authoringAssetDTO{
		AssetKey: string(asset.ID), Filename: asset.OriginalFilename,
		MediaType: asset.MediaType, ByteSize: asset.ByteSize, Status: asset.Lifecycle,
	})
}

func singleAssetMultipart(w http.ResponseWriter, r *http.Request, maxAssetBytes int64) (string, io.ReadCloser, error) {
	mediaType, parameters, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || parameters["boundary"] == "" {
		return "", nil, assets.ErrInvalidAssetContent
	}
	limit := maxAssetBytes
	if limit <= math.MaxInt64-authoringAssetMultipartOverheadBytes {
		limit += authoringAssetMultipartOverheadBytes
	} else {
		limit = math.MaxInt64
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	reader := multipart.NewReader(r.Body, parameters["boundary"])
	part, err := reader.NextPart()
	if err != nil {
		return "", nil, multipartError(err)
	}
	disposition, values, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	filename, hasFilename := values["filename"]
	if err != nil || disposition != "form-data" || values["name"] != "file" || !hasFilename || filename == "" {
		_ = part.Close()
		return "", nil, assets.ErrInvalidAssetContent
	}
	return filename, &singleMultipartFile{part: part, reader: reader}, nil
}

type singleMultipartFile struct {
	part    *multipart.Part
	reader  *multipart.Reader
	checked bool
}

func (f *singleMultipartFile) Read(buffer []byte) (int, error) {
	n, err := f.part.Read(buffer)
	if n > 0 {
		// Readers may return final data together with io.EOF. Suppress that EOF
		// once so the next read can verify that no second multipart part exists.
		if errors.Is(err, io.EOF) {
			return n, nil
		}
		return n, err
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, multipartError(err)
	}
	if f.checked {
		return 0, io.EOF
	}
	f.checked = true
	next, nextErr := f.reader.NextPart()
	if next != nil {
		_ = next.Close()
		return 0, assets.ErrInvalidAssetContent
	}
	if errors.Is(nextErr, io.EOF) {
		return 0, io.EOF
	}
	return 0, multipartError(nextErr)
}

func (f *singleMultipartFile) Close() error { return f.part.Close() }

func multipartError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return assets.ErrAssetTooLarge
	}
	return assets.ErrInvalidAssetContent
}

func authoringAssetProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authoring.ErrNotFound), errors.Is(err, authoring.ErrAuthorizationDenied):
		problem(w, r, http.StatusNotFound, "Not found")
	case errors.Is(err, assets.ErrAssetTooLarge):
		problemCode(w, r, http.StatusRequestEntityTooLarge, "Asset upload too large", "asset_too_large")
	case errors.Is(err, assets.ErrInvalidAsset), errors.Is(err, assets.ErrInvalidAssetContent):
		problemCode(w, r, http.StatusBadRequest, "Invalid asset upload", "invalid_asset_upload")
	default:
		problem(w, r, http.StatusInternalServerError, "Asset upload failed")
	}
}
