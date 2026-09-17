package localstorage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/google/uuid"
)

const (
	objectsDirectory   = "objects"
	temporaryDirectory = ".tmp"
)

type idGenerator func() (assets.StorageObjectID, error)

// Storage is a local-disk BinaryStorage provider. Its root and all filesystem
// names remain implementation-private; callers see only opaque object IDs.
type Storage struct {
	root       *os.Root
	generateID idGenerator
}

var _ assets.BinaryStorage = (*Storage)(nil)

func New(rootPath string) (*Storage, error) {
	return newStorage(rootPath, randomObjectID)
}

func newStorage(rootPath string, generateID idGenerator) (*Storage, error) {
	if rootPath == "" || !filepath.IsAbs(rootPath) || generateID == nil {
		return nil, assets.ErrBinaryStorage
	}
	cleaned := filepath.Clean(rootPath)
	if err := os.MkdirAll(cleaned, 0o700); err != nil {
		return nil, assets.ErrBinaryStorage
	}
	root, err := os.OpenRoot(cleaned)
	if err != nil {
		return nil, assets.ErrBinaryStorage
	}
	for _, directory := range []string{objectsDirectory, temporaryDirectory} {
		if err := root.MkdirAll(directory, 0o700); err != nil {
			_ = root.Close()
			return nil, assets.ErrBinaryStorage
		}
		if err := root.Chmod(directory, 0o700); err != nil {
			_ = root.Close()
			return nil, assets.ErrBinaryStorage
		}
	}
	return &Storage{root: root, generateID: generateID}, nil
}

func (s *Storage) Close() error { return s.root.Close() }

func (s *Storage) Put(ctx context.Context, source io.Reader) (assets.StoredBinary, error) {
	if source == nil {
		return assets.StoredBinary{}, assets.ErrBinaryStorage
	}
	objectID, err := s.generateID()
	if err != nil {
		return assets.StoredBinary{}, assets.ErrBinaryStorage
	}
	if _, err := assets.ParseStorageObjectID(string(objectID)); err != nil {
		return assets.StoredBinary{}, assets.ErrBinaryStorage
	}
	temporaryID, err := randomObjectID()
	if err != nil {
		return assets.StoredBinary{}, assets.ErrBinaryStorage
	}
	temporaryName := temporaryDirectory + "/" + string(temporaryID) + ".part"
	file, err := s.root.OpenFile(temporaryName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return assets.StoredBinary{}, assets.ErrBinaryStorage
	}
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = file.Close()
			_ = s.root.Remove(temporaryName)
		}
	}()

	digest := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, digest), contextReader{ctx: ctx, reader: source})
	if copyErr != nil {
		return assets.StoredBinary{}, storageError(copyErr)
	}
	if written <= 0 {
		return assets.StoredBinary{}, assets.ErrInvalidAssetContent
	}
	if err := file.Sync(); err != nil {
		return assets.StoredBinary{}, assets.ErrBinaryStorage
	}
	if err := file.Close(); err != nil {
		return assets.StoredBinary{}, assets.ErrBinaryStorage
	}
	if err := ctx.Err(); err != nil {
		return assets.StoredBinary{}, err
	}

	finalName := objectName(objectID)
	if err := s.root.Link(temporaryName, finalName); err != nil {
		return assets.StoredBinary{}, assets.ErrBinaryStorage
	}
	if err := s.root.Remove(temporaryName); err != nil {
		_ = s.root.Remove(finalName)
		return assets.StoredBinary{}, assets.ErrBinaryStorage
	}
	removeTemporary = false
	if err := syncDirectory(s.root, objectsDirectory); err != nil {
		_ = s.root.Remove(finalName)
		return assets.StoredBinary{}, assets.ErrBinaryStorage
	}
	return assets.StoredBinary{
		StorageObjectID: objectID,
		ByteSize:        written,
		SHA256Digest:    assets.SHA256Digest(hex.EncodeToString(digest.Sum(nil))),
	}, nil
}

func (s *Storage) Open(ctx context.Context, id assets.StorageObjectID) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := assets.ParseStorageObjectID(string(id)); err != nil {
		return nil, assets.ErrInvalidStorageObject
	}
	name := objectName(id)
	info, err := s.root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, assets.ErrStorageObjectMissing
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, assets.ErrBinaryStorage
	}
	file, err := s.root.Open(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, assets.ErrStorageObjectMissing
	}
	if err != nil {
		return nil, assets.ErrBinaryStorage
	}
	return file, nil
}

func (s *Storage) DiscardUncommitted(ctx context.Context, id assets.StorageObjectID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := assets.ParseStorageObjectID(string(id)); err != nil {
		return assets.ErrInvalidStorageObject
	}
	name := objectName(id)
	info, err := s.root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return assets.ErrBinaryStorage
	}
	if err := s.root.Remove(name); err != nil {
		return assets.ErrBinaryStorage
	}
	return syncDirectory(s.root, objectsDirectory)
}

func objectName(id assets.StorageObjectID) string {
	return objectsDirectory + "/" + string(id)
}

func randomObjectID() (assets.StorageObjectID, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return assets.StorageObjectID(id.String()), nil
}

func syncDirectory(root *os.Root, name string) error {
	directory, err := root.Open(name)
	if err != nil {
		return assets.ErrBinaryStorage
	}
	defer func() { _ = directory.Close() }()
	if err := directory.Sync(); err != nil {
		return assets.ErrBinaryStorage
	}
	return nil
}

func storageError(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), errors.Is(err, assets.ErrAssetTooLarge):
		return err
	default:
		return assets.ErrBinaryStorage
	}
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
