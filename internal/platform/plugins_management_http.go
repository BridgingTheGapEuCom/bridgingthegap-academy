package platform

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"sort"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
	"github.com/go-chi/chi/v5"
)

// pluginManagementAPI exposes installation administration state. It deliberately
// projects RegistryService results instead of reading persistence or evaluating
// trust in transport code.
type pluginManagementAPI struct {
	registry *plugins.RegistryService
}

const pluginPackageUploadMediaType = "application/zip"
const maxPluginKeyBodyBytes = 16 << 10

type pluginManagementEntrypointResponse struct {
	ID          string             `json:"id"`
	Type        plugins.PluginType `json:"type"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
}

type pluginManagementReleaseResponse struct {
	PluginID           plugins.PluginID                     `json:"pluginId"`
	Version            string                               `json:"version"`
	ArtifactDigest     string                               `json:"artifactDigest"`
	Name               string                               `json:"name"`
	Description        string                               `json:"description"`
	PublisherName      string                               `json:"publisherName"`
	Homepage           string                               `json:"homepage,omitempty"`
	Entrypoints        []pluginManagementEntrypointResponse `json:"entrypoints"`
	CurrentTrust       *plugins.TrustLevel                  `json:"currentTrust,omitempty"`
	ValidationStatus   string                               `json:"validationStatus"`
	SignatureStatus    string                               `json:"signatureStatus"`
	ApprovalState      string                               `json:"approvalState"`
	Enabled            bool                                 `json:"enabled"`
	ExecutionPermitted bool                                 `json:"executionPermitted"`
	InstalledAt        time.Time                            `json:"installedAt"`
}

type pluginVerificationKeyRequest struct {
	KeyID            string             `json:"keyId"`
	PublicKey        string             `json:"publicKey"`
	Purpose          plugins.KeyPurpose `json:"purpose"`
	AllowedPluginIDs []plugins.PluginID `json:"allowedPluginIds"`
}

type pluginVerificationKeyResponse struct {
	KeyID            string             `json:"keyId"`
	Purpose          plugins.KeyPurpose `json:"purpose"`
	AllowedPluginIDs []plugins.PluginID `json:"allowedPluginIds"`
	Enabled          bool               `json:"enabled"`
	Fingerprint      string             `json:"fingerprint"`
}

func (h *pluginManagementAPI) list(w http.ResponseWriter, r *http.Request) {
	values, err := h.releases(r)
	if err != nil {
		pluginManagementProblem(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"plugins": values})
}

func (h *pluginManagementAPI) detail(w http.ResponseWriter, r *http.Request) {
	pluginID := plugins.PluginID(chi.URLParam(r, "pluginId"))
	version := chi.URLParam(r, "version")
	values, err := h.releases(r)
	if err != nil {
		pluginManagementProblem(w, r, err)
		return
	}
	for _, value := range values {
		if value.PluginID == pluginID && value.Version == version {
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, value)
			return
		}
	}
	problem(w, r, http.StatusNotFound, "Not found")
}

// register accepts a raw ZIP package rather than a JSON wrapper. The canonical
// package reader owns all hostile-archive, manifest, checksum, and signature
// validation; this transport layer never accepts claimed package coordinates.
func (h *pluginManagementAPI) register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.registry == nil {
		pluginManagementProblem(w, r, plugins.ErrStorage)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != pluginPackageUploadMediaType {
		problemCode(w, r, http.StatusBadRequest, "Invalid plugin package", "invalid_archive")
		return
	}
	limits := plugins.DefaultLimits()
	if r.ContentLength > limits.MaxCompressedBytes {
		problemCode(w, r, http.StatusRequestEntityTooLarge, "Plugin package too large", "package_too_large")
		return
	}
	pkg, err := plugins.NewPackageReader(limits).Read(r.Context(), http.MaxBytesReader(w, r.Body, limits.MaxCompressedBytes))
	if err != nil {
		pluginPackageProblem(w, r, err)
		return
	}
	release, created, err := h.registry.Register(r.Context(), pkg)
	if err != nil {
		pluginPackageProblem(w, r, err)
		return
	}
	value, err := h.release(r, release.Release.PluginID, release.Release.Version)
	if err != nil {
		pluginManagementProblem(w, r, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	writeJSON(w, status, value)
}

func (h *pluginManagementAPI) approve(w http.ResponseWriter, r *http.Request) {
	h.releaseMutation(w, r, func(id plugins.PluginID, version string) error {
		_, err := h.registry.ApproveRelease(r.Context(), id, version)
		return err
	})
}

func (h *pluginManagementAPI) revokeApproval(w http.ResponseWriter, r *http.Request) {
	h.releaseMutation(w, r, func(id plugins.PluginID, version string) error {
		return h.registry.RevokeReleaseApproval(r.Context(), id, version)
	})
}

func (h *pluginManagementAPI) enable(w http.ResponseWriter, r *http.Request) {
	h.releaseMutation(w, r, func(id plugins.PluginID, version string) error {
		_, err := h.registry.Enable(r.Context(), id, version)
		return err
	})
}

func (h *pluginManagementAPI) disable(w http.ResponseWriter, r *http.Request) {
	h.releaseMutation(w, r, func(id plugins.PluginID, version string) error {
		_, err := h.registry.Disable(r.Context(), id, version)
		return err
	})
}

func (h *pluginManagementAPI) releaseMutation(w http.ResponseWriter, r *http.Request, mutate func(plugins.PluginID, string) error) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.registry == nil || !emptyPluginManagementBody(w, r) {
		problem(w, r, http.StatusBadRequest, "Invalid request")
		return
	}
	id := plugins.PluginID(chi.URLParam(r, "pluginId"))
	version := chi.URLParam(r, "version")
	if err := mutate(id, version); err != nil {
		pluginLifecycleProblem(w, r, err)
		return
	}
	value, err := h.release(r, id, version)
	if err != nil {
		pluginManagementProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (h *pluginManagementAPI) keys(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		pluginManagementProblem(w, r, plugins.ErrStorage)
		return
	}
	keys, err := h.registry.Keys(r.Context())
	if err != nil {
		pluginLifecycleProblem(w, r, err)
		return
	}
	values := make([]pluginVerificationKeyResponse, 0, len(keys))
	for _, key := range keys {
		values = append(values, verificationKeyResponse(key))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"keys": values})
}

func (h *pluginManagementAPI) addKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.registry == nil {
		pluginManagementProblem(w, r, plugins.ErrStorage)
		return
	}
	var input pluginVerificationKeyRequest
	if decodeAuthoringBody(w, r, maxPluginKeyBodyBytes, &input) != nil {
		problemCode(w, r, http.StatusBadRequest, "Invalid verification key", "invalid_key")
		return
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(input.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		problemCode(w, r, http.StatusBadRequest, "Invalid verification key", "invalid_key")
		return
	}
	allowed := append([]plugins.PluginID(nil), input.AllowedPluginIDs...)
	sort.Slice(allowed, func(i, j int) bool { return allowed[i] < allowed[j] })
	key, err := h.registry.AddKey(r.Context(), plugins.VerificationKey{ID: input.KeyID, PublicKey: ed25519.PublicKey(publicKey), Purpose: input.Purpose, AllowedPluginIDs: allowed, Enabled: true})
	if err != nil {
		pluginLifecycleProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, verificationKeyResponse(key))
}

func (h *pluginManagementAPI) setKeyState(w http.ResponseWriter, r *http.Request, enabled bool) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.registry == nil || !emptyPluginManagementBody(w, r) {
		problem(w, r, http.StatusBadRequest, "Invalid request")
		return
	}
	key, err := h.registry.SetKeyState(r.Context(), chi.URLParam(r, "keyId"), enabled)
	if err != nil {
		pluginLifecycleProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, verificationKeyResponse(key))
}

func verificationKeyResponse(key plugins.VerificationKey) pluginVerificationKeyResponse {
	digest := sha256.Sum256(key.PublicKey)
	return pluginVerificationKeyResponse{KeyID: key.ID, Purpose: key.Purpose, AllowedPluginIDs: append([]plugins.PluginID(nil), key.AllowedPluginIDs...), Enabled: key.Enabled, Fingerprint: "sha256:" + hex.EncodeToString(digest[:])}
}

func emptyPluginManagementBody(w http.ResponseWriter, r *http.Request) bool {
	if r.ContentLength > 0 {
		return false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
	return err == nil && len(body) == 0
}

func (h *pluginManagementAPI) releases(r *http.Request) ([]pluginManagementReleaseResponse, error) {
	if h == nil || h.registry == nil {
		return nil, plugins.ErrStorage
	}
	values, err := h.registry.ManagementReleases(r.Context())
	if err != nil {
		return nil, err
	}
	result := make([]pluginManagementReleaseResponse, 0, len(values))
	for _, value := range values {
		release := value.Release
		entrypoints := make([]pluginManagementEntrypointResponse, 0, len(release.Manifest.Entrypoints))
		for _, entry := range release.Manifest.Entrypoints {
			entrypoints = append(entrypoints, pluginManagementEntrypointResponse{ID: entry.ID, Type: entry.Type, Name: entry.Name})
		}
		sort.Slice(entrypoints, func(i, j int) bool { return entrypoints[i].ID < entrypoints[j].ID })

		response := pluginManagementReleaseResponse{
			PluginID:           release.Release.PluginID,
			Version:            release.Release.Version,
			ArtifactDigest:     release.Release.ArtifactDigest,
			Name:               release.Manifest.Name,
			Description:        release.Manifest.Description,
			PublisherName:      release.Manifest.Publisher.Name,
			Homepage:           release.Manifest.Homepage,
			Entrypoints:        entrypoints,
			ValidationStatus:   value.ValidationStatus,
			SignatureStatus:    managementSignatureStatus(release, value.ValidationStatus),
			ApprovalState:      managementApprovalState(value.Approvals),
			Enabled:            release.Enabled,
			ExecutionPermitted: value.ValidationStatus != plugins.ValidationStatusSignatureInvalid && h.registry.ExecutionPermitted(r.Context(), release),
			InstalledAt:        release.InstalledAt,
		}
		if value.ValidationStatus != plugins.ValidationStatusSignatureInvalid {
			trust := release.CurrentTrust
			response.CurrentTrust = &trust
		}
		result = append(result, response)
	}
	return result, nil
}

func (h *pluginManagementAPI) release(r *http.Request, pluginID plugins.PluginID, version string) (pluginManagementReleaseResponse, error) {
	values, err := h.releases(r)
	if err != nil {
		return pluginManagementReleaseResponse{}, err
	}
	for _, value := range values {
		if value.PluginID == pluginID && value.Version == version {
			return value, nil
		}
	}
	return pluginManagementReleaseResponse{}, plugins.ErrNotFound
}

func managementSignatureStatus(release plugins.InstalledRelease, validationStatus string) string {
	if validationStatus == plugins.ValidationStatusSignatureInvalid {
		return "INVALID"
	}
	if release.Signature == nil {
		return "UNSIGNED"
	}
	return "VALID"
}

func managementApprovalState(approvals []plugins.Approval) string {
	for _, approval := range approvals {
		if approval.Active() {
			return "ACTIVE"
		}
	}
	if len(approvals) > 0 {
		return "REVOKED"
	}
	return "NONE"
}

func pluginManagementProblem(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, plugins.ErrNotFound) {
		problem(w, r, http.StatusNotFound, "Not found")
		return
	}
	problem(w, r, http.StatusServiceUnavailable, "Plugin management unavailable")
}

func pluginPackageProblem(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, plugins.ErrReleaseConflict) {
		problemCode(w, r, http.StatusConflict, "Plugin release conflict", "release_conflict")
		return
	}
	if errors.Is(err, plugins.ErrStorage) {
		problem(w, r, http.StatusServiceUnavailable, "Plugin management unavailable")
		return
	}
	for _, code := range []plugins.ErrorCode{
		plugins.ErrInvalidArchive,
		plugins.ErrPackageTooLarge,
		plugins.ErrInvalidManifest,
		plugins.ErrUnsupportedFormat,
		plugins.ErrUnsupportedVersion,
		plugins.ErrChecksumMismatch,
		plugins.ErrInvalidSignature,
	} {
		if plugins.IsPackageError(err, code) {
			status := http.StatusBadRequest
			if code == plugins.ErrPackageTooLarge {
				status = http.StatusRequestEntityTooLarge
			}
			problemCode(w, r, status, "Invalid plugin package", string(code))
			return
		}
	}
	problemCode(w, r, http.StatusBadRequest, "Invalid plugin package", "invalid_package")
}

func pluginLifecycleProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, plugins.ErrNotFound):
		problem(w, r, http.StatusNotFound, "Not found")
	case errors.Is(err, plugins.ErrKeyIDConflict), errors.Is(err, plugins.ErrApprovalConflict):
		problemCode(w, r, http.StatusConflict, "Plugin lifecycle conflict", "lifecycle_conflict")
	case errors.Is(err, plugins.ErrEnableDenied):
		problemCode(w, r, http.StatusConflict, "Plugin release cannot be enabled", "enable_denied")
	case errors.Is(err, plugins.ErrInvalidKey):
		problemCode(w, r, http.StatusBadRequest, "Invalid verification key", "invalid_key")
	case errors.Is(err, plugins.ErrInvalidApproval), errors.Is(err, plugins.ErrInvalidRelease):
		problemCode(w, r, http.StatusBadRequest, "Invalid plugin lifecycle request", "invalid_lifecycle")
	default:
		problem(w, r, http.StatusServiceUnavailable, "Plugin management unavailable")
	}
}
