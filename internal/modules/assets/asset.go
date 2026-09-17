package assets

import (
	"encoding/hex"
	"errors"
	"mime"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxOriginalFilenameCharacters = 255
	MaxMediaTypeCharacters        = 127
)

type AssetID string
type StorageObjectID string
type SHA256Digest string
type Lifecycle string

const (
	LifecyclePending   Lifecycle = "PENDING"
	LifecycleAvailable Lifecycle = "AVAILABLE"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

var (
	ErrInvalidAsset               = errors.New("invalid asset")
	ErrInvalidLifecycleTransition = errors.New("invalid asset lifecycle transition")
)

// Asset is authoritative internal metadata for one managed binary. Asset ID is
// the stable content reference. StorageObjectID and CreatedByUserID are private
// implementation/provenance values and are not learner-facing metadata.
type Asset struct {
	ID               AssetID
	OwnerDraftID     string `json:"-"`
	OriginalFilename string
	MediaType        string
	ByteSize         int64
	SHA256Digest     SHA256Digest
	StorageObjectID  StorageObjectID `json:"-"`
	Lifecycle        Lifecycle
	CreatedByUserID  string `json:"-"`
	CreatedAt        time.Time
}

// AssetInput creates PENDING metadata only. Later HTTP upload work must derive
// creator identity from the session and authorize OwnerDraftID before
// constructing it; only a BinaryStorage result can complete the record.
type AssetInput struct {
	OwnerDraftID     string `json:"-"`
	OriginalFilename string
	MediaType        string
	CreatedByUserID  string `json:"-"`
}

func ParseLifecycle(value string) (Lifecycle, error) {
	status := Lifecycle(value)
	if status != LifecyclePending && status != LifecycleAvailable {
		return "", ErrInvalidAsset
	}
	return status, nil
}

func ParseSHA256Digest(value string) (SHA256Digest, error) {
	if len(value) != 64 || strings.ToLower(value) != value {
		return "", ErrInvalidAsset
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return "", ErrInvalidAsset
	}
	return SHA256Digest(value), nil
}

// NormalizeMediaType accepts one canonical lower-case MIME type without
// parameters. Upload processing must detect and verify it from the bytes before
// moving an Asset to AVAILABLE; a filename or client declaration is not proof.
func NormalizeMediaType(value string) (string, error) {
	value = strings.TrimSpace(value)
	mediaType, parameters, err := mime.ParseMediaType(value)
	if err != nil || len(parameters) != 0 || len(mediaType) == 0 || len(mediaType) > MaxMediaTypeCharacters || !strings.Contains(mediaType, "/") {
		return "", ErrInvalidAsset
	}
	mediaType = strings.ToLower(mediaType)
	if mime.FormatMediaType(mediaType, nil) != mediaType {
		return "", ErrInvalidAsset
	}
	return mediaType, nil
}

func ValidateOriginalFilename(value string) error {
	if value == "" || value != strings.TrimSpace(value) || value == "." || value == ".." ||
		!utf8.ValidString(value) || utf8.RuneCountInString(value) > MaxOriginalFilenameCharacters {
		return ErrInvalidAsset
	}
	for _, r := range value {
		if r == '/' || r == '\\' || unicode.IsControl(r) {
			return ErrInvalidAsset
		}
	}
	return nil
}

func (a Asset) Validate() error {
	if !validUUID(string(a.ID)) || a.CreatedAt.IsZero() {
		return ErrInvalidAsset
	}
	return validateMetadata(a.OwnerDraftID, a.OriginalFilename, a.MediaType, a.ByteSize, a.SHA256Digest, a.StorageObjectID, a.Lifecycle, a.CreatedByUserID)
}

func (a AssetInput) Validate() error {
	return validateBaseMetadata(a.OwnerDraftID, a.OriginalFilename, a.MediaType, a.CreatedByUserID)
}

func validateMetadata(ownerDraftID, filename, mediaType string, byteSize int64, digest SHA256Digest, objectID StorageObjectID, lifecycle Lifecycle, creatorID string) error {
	if validateBaseMetadata(ownerDraftID, filename, mediaType, creatorID) != nil {
		return ErrInvalidAsset
	}
	switch lifecycle {
	case LifecyclePending:
		if byteSize != 0 || digest != "" || objectID != "" {
			return ErrInvalidAsset
		}
	case LifecycleAvailable:
		if byteSize <= 0 || !validUUID(string(objectID)) {
			return ErrInvalidAsset
		}
		if parsed, err := ParseSHA256Digest(string(digest)); err != nil || parsed != digest {
			return ErrInvalidAsset
		}
	default:
		return ErrInvalidAsset
	}
	return nil
}

func validateBaseMetadata(ownerDraftID, filename, mediaType, creatorID string) error {
	if !validUUID(ownerDraftID) || !validUUID(creatorID) || ValidateOriginalFilename(filename) != nil {
		return ErrInvalidAsset
	}
	normalizedMediaType, err := NormalizeMediaType(mediaType)
	if err != nil || normalizedMediaType != mediaType {
		return ErrInvalidAsset
	}
	return nil
}

func validUUID(value string) bool { return uuidPattern.MatchString(value) }

// MarkAvailable returns a new value and never mutates the pending Asset. The
// storage provider owns the object identity and measured integrity metadata.
func (a Asset) MarkAvailable(stored StoredBinary) (Asset, error) {
	if a.Validate() != nil || a.Lifecycle != LifecyclePending || stored.Validate() != nil {
		return Asset{}, ErrInvalidLifecycleTransition
	}
	result := a
	result.Lifecycle = LifecycleAvailable
	result.ByteSize = stored.ByteSize
	result.SHA256Digest = stored.SHA256Digest
	result.StorageObjectID = stored.StorageObjectID
	return result, nil
}
