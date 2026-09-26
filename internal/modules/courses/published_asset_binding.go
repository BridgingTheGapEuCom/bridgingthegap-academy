package courses

import (
	"encoding/hex"
	"mime"
	"strings"
	"unicode"
	"unicode/utf8"
)

// PublishedAssetBinding freezes the storage-neutral meaning of one canonical
// assetKey for an immutable CourseVersion. StorageObjectID is an internal
// delivery locator and is never part of LessonContent or a public read model.
type PublishedAssetBinding struct {
	AssetKey         string
	StorageObjectID  string `json:"-"`
	OriginalFilename string
	MediaType        string
	ByteSize         int64
	SHA256Digest     string
}

func (b PublishedAssetBinding) Validate() error {
	if !uuidPattern.MatchString(b.AssetKey) || !uuidPattern.MatchString(b.StorageObjectID) || b.ByteSize <= 0 || !validPublishedFilename(b.OriginalFilename) {
		return ErrInvalidImmutableCourseVersion
	}
	if len(b.SHA256Digest) != 64 || strings.ToLower(b.SHA256Digest) != b.SHA256Digest {
		return ErrInvalidImmutableCourseVersion
	}
	digest, err := hex.DecodeString(b.SHA256Digest)
	if err != nil || len(digest) != 32 {
		return ErrInvalidImmutableCourseVersion
	}
	mediaType, parameters, err := mime.ParseMediaType(b.MediaType)
	if err != nil || len(parameters) != 0 || len(b.MediaType) > 127 || mediaType != strings.ToLower(mediaType) || mediaType != b.MediaType || !strings.Contains(mediaType, "/") {
		return ErrInvalidImmutableCourseVersion
	}
	return nil
}

func validPublishedFilename(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || value == "." || value == ".." || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 255 {
		return false
	}
	for _, r := range value {
		if r == '/' || r == '\\' || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

type contentAssetReference struct {
	key         string
	mediaPrefix string
}

func contentAssetReferences(content LessonContent) []contentAssetReference {
	references := make([]contentAssetReference, 0)
	for _, block := range content.Blocks {
		switch payload := block.Payload.(type) {
		case ImageBlockPayload:
			references = append(references, contentAssetReference{key: payload.Asset.AssetKey, mediaPrefix: "image/"})
		case VideoBlockPayload:
			references = append(references, contentAssetReference{key: payload.Asset.AssetKey, mediaPrefix: "video/"}, contentAssetReference{key: payload.CaptionsAsset.AssetKey})
			if payload.TranscriptAsset != nil {
				references = append(references, contentAssetReference{key: payload.TranscriptAsset.AssetKey})
			}
		case AudioBlockPayload:
			references = append(references, contentAssetReference{key: payload.Asset.AssetKey, mediaPrefix: "audio/"})
			if payload.TranscriptAsset != nil {
				references = append(references, contentAssetReference{key: payload.TranscriptAsset.AssetKey})
			}
		case DownloadBlockPayload:
			references = append(references, contentAssetReference{key: payload.Asset.AssetKey})
		}
	}
	return references
}
