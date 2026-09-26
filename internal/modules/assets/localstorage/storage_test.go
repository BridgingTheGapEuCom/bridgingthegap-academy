package localstorage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
)

const fixedObjectID = assets.StorageObjectID("44444444-4444-4444-8444-444444444444")

func TestNewRequiresUsableAbsoluteRoot(t *testing.T) {
	if _, err := New(""); !errors.Is(err, assets.ErrBinaryStorage) {
		t.Fatalf("empty root error = %v", err)
	}
	if _, err := New("relative/assets"); !errors.Is(err, assets.ErrBinaryStorage) {
		t.Fatalf("relative root error = %v", err)
	}
	filePath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(filePath, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(filePath); !errors.Is(err, assets.ErrBinaryStorage) {
		t.Fatalf("unusable root error = %v", err)
	}
}

func TestPutOpenRoundTripUsesOpaqueProviderIdentity(t *testing.T) {
	rootPath := t.TempDir()
	storage, err := New(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	content := []byte("binary content that is not derived from ../unsafe-name.txt")

	stored, err := storage.Put(context.Background(), bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assets.ParseStorageObjectID(string(stored.StorageObjectID)); err != nil {
		t.Fatalf("provider returned non-opaque identity: %q", stored.StorageObjectID)
	}
	opened, err := storage.Open(context.Background(), stored.StorageObjectID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	got, err := io.ReadAll(opened)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("round-trip = %q, %v", got, err)
	}
	if stored.ByteSize != int64(len(content)) || string(stored.SHA256Digest) != "cbfa383457a79b865edfb5f078c26edda963c400ee8cbe1c8a675aad80c14c9f" {
		t.Fatalf("measured metadata = %#v", stored)
	}
	if _, err := os.Stat(filepath.Join(rootPath, "unsafe-name.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("caller metadata influenced storage placement")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(rootPath, objectsDirectory, string(stored.StorageObjectID)))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("stored file mode = %v, %v", info.Mode().Perm(), err)
		}
	}
}

func TestOpenAndDiscardRejectMalformedTraversalAndSymlinks(t *testing.T) {
	rootPath := t.TempDir()
	storage, err := New(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	for _, id := range []assets.StorageObjectID{"", "../secret", "44444444-4444-4444-8444-444444444444/../../secret"} {
		if _, err := storage.Open(context.Background(), id); !errors.Is(err, assets.ErrInvalidStorageObject) {
			t.Fatalf("Open(%q) error = %v", id, err)
		}
		if err := storage.DiscardUncommitted(context.Background(), id); !errors.Is(err, assets.ErrInvalidStorageObject) {
			t.Fatalf("DiscardUncommitted(%q) error = %v", id, err)
		}
	}

	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(rootPath, objectsDirectory, string(fixedObjectID))
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := storage.Open(context.Background(), fixedObjectID); !errors.Is(err, assets.ErrBinaryStorage) {
		t.Fatalf("symlink opened: %v", err)
	}
	if err := storage.DiscardUncommitted(context.Background(), fixedObjectID); !errors.Is(err, assets.ErrBinaryStorage) {
		t.Fatalf("symlink discarded as a regular object: %v", err)
	}
}

func TestPutNeverOverwritesAndCleansPartialFiles(t *testing.T) {
	rootPath := t.TempDir()
	storage, err := newStorage(rootPath, func() (assets.StorageObjectID, error) { return fixedObjectID, nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	if _, err := storage.Put(context.Background(), bytes.NewReader([]byte("first"))); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Put(context.Background(), bytes.NewReader([]byte("second"))); !errors.Is(err, assets.ErrBinaryStorage) {
		t.Fatalf("duplicate object write error = %v", err)
	}
	opened, err := storage.Open(context.Background(), fixedObjectID)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(opened)
	_ = opened.Close()
	if string(got) != "first" {
		t.Fatalf("existing object was overwritten: %q", got)
	}

	failingStorage, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = failingStorage.Close() })
	if _, err := failingStorage.Put(context.Background(), &failingReader{}); !errors.Is(err, assets.ErrBinaryStorage) {
		t.Fatalf("partial read error = %v", err)
	}
	for _, directory := range []string{objectsDirectory, temporaryDirectory} {
		entries, err := os.ReadDir(filepath.Join(failingStorage.root.Name(), directory))
		if err != nil || len(entries) != 0 {
			t.Fatalf("%s retained partial files: %v, %v", directory, entries, err)
		}
	}
}

type failingReader struct{ read bool }

func (r *failingReader) Read(buffer []byte) (int, error) {
	if !r.read {
		r.read = true
		copy(buffer, "partial")
		return len("partial"), nil
	}
	return 0, errors.New("interrupted source")
}
