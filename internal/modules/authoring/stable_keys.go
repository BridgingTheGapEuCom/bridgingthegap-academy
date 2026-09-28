package authoring

import (
	"crypto/rand"
	"encoding/hex"
)

// newGeneratedStructureKey creates opaque, bounded keys for immutable
// authoring structure identities. The prefix distinguishes the entity type
// without deriving identity from mutable author-authored text.
func newGeneratedStructureKey(prefix string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "-" + hex.EncodeToString(bytes), nil
}

func newModuleStableKey() (string, error) { return newGeneratedStructureKey("module") }

func newLessonStableKey() (string, error) { return newGeneratedStructureKey("lesson") }
