package courses

import (
	"context"
	"errors"
	"testing"
)

type publishedAssetBindingRepositoryFake struct {
	binding PublishedAssetBinding
	err     error
	course  CourseID
	version Version
	asset   string
}

func (f *publishedAssetBindingRepositoryFake) GetPublishedAssetBindingByCourseAndVersionAndAssetKey(_ context.Context, courseID CourseID, version Version, assetKey string) (PublishedAssetBinding, error) {
	f.course, f.version, f.asset = courseID, version, assetKey
	return f.binding, f.err
}

func validPublishedAssetBinding() PublishedAssetBinding {
	return PublishedAssetBinding{
		AssetKey:         "50000000-0000-4000-8000-000000000001",
		StorageObjectID:  "60000000-0000-4000-8000-000000000001",
		OriginalFilename: "diagram.png",
		MediaType:        "image/png",
		ByteSize:         42,
		SHA256Digest:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
}

func TestPublishedAssetReadServiceResolvesOnlyExactPublishedCoordinates(t *testing.T) {
	version, err := ParseVersion("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	repository := &publishedAssetBindingRepositoryFake{binding: validPublishedAssetBinding()}
	service := NewPublishedAssetReadService(repository)
	binding, err := service.Exact(context.Background(), "10000000-0000-4000-8000-000000000001", version, validPublishedAssetBinding().AssetKey)
	if err != nil {
		t.Fatal(err)
	}
	if binding.StorageObjectID != validPublishedAssetBinding().StorageObjectID || repository.course != "10000000-0000-4000-8000-000000000001" || repository.version != version || repository.asset != binding.AssetKey {
		t.Fatalf("binding was not resolved through exact public coordinates: %#v %#v", binding, repository)
	}
}

func TestPublishedAssetReadServiceRejectsMalformedOrInvalidStoredBindings(t *testing.T) {
	version, err := ParseVersion("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*publishedAssetBindingRepositoryFake, *string){
		"malformed asset key":    func(_ *publishedAssetBindingRepositoryFake, assetKey *string) { *assetKey = "not-an-asset" },
		"invalid stored binding": func(repository *publishedAssetBindingRepositoryFake, _ *string) { repository.binding.ByteSize = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			repository := &publishedAssetBindingRepositoryFake{binding: validPublishedAssetBinding()}
			assetKey := repository.binding.AssetKey
			mutate(repository, &assetKey)
			_, err := NewPublishedAssetReadService(repository).Exact(context.Background(), "10000000-0000-4000-8000-000000000001", version, assetKey)
			if name == "malformed asset key" && !errors.Is(err, ErrNotFound) {
				t.Fatalf("malformed key error = %v, want not found", err)
			}
			if name == "invalid stored binding" && !errors.Is(err, ErrInvalidImmutableCourseVersion) {
				t.Fatalf("invalid binding error = %v, want invalid immutable course version", err)
			}
		})
	}
}
