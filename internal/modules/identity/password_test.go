package identity

import (
	"errors"
	"strings"
	"testing"
)

func testPasswordHasher(t *testing.T) Argon2idHasher {
	t.Helper()
	hasher, err := NewArgon2idHasher(Argon2idParameters{MemoryKiB: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	if err != nil {
		t.Fatal(err)
	}
	return hasher
}

func TestArgon2idHashAndVerify(t *testing.T) {
	hasher := testPasswordHasher(t)
	password := []byte("a secure test passphrase")
	first, err := hasher.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := hasher.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first.Value() == second.Value() {
		t.Fatal("identical passwords received identical salts")
	}
	if !strings.HasPrefix(first.Value(), "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Fatalf("unexpected PHC encoding: %q", first.Value())
	}
	if strings.Contains(first.Value(), string(password)) {
		t.Fatal("encoded hash contains plaintext password")
	}
	valid, err := hasher.VerifyPassword(first, password)
	if err != nil || !valid {
		t.Fatalf("correct password did not verify: %v", err)
	}
	valid, err = hasher.VerifyPassword(first, []byte("a different test passphrase"))
	if err != nil || valid {
		t.Fatalf("incorrect password verified: %v", err)
	}
}

func TestDefaultArgon2idHasherUsesDocumentedParameters(t *testing.T) {
	hasher := DefaultPasswordHasher()
	hash, err := hasher.HashPassword([]byte("correct horse battery staple"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash.Value(), "$argon2id$v=19$m=65536,t=3,p=1$") {
		t.Fatalf("default hash does not preserve documented parameters: %q", hash.Value())
	}
}

func TestArgon2idHashRejectsInvalidPasswords(t *testing.T) {
	hasher := testPasswordHasher(t)
	for _, password := range [][]byte{[]byte("too short"), make([]byte, MaximumPasswordBytes+1), {0xff, 0xfe, 0xfd}} {
		if _, err := hasher.HashPassword(password); !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("invalid password was accepted: %v", err)
		}
	}
	if _, err := hasher.HashPassword([]byte("correct horse battery staple")); err != nil {
		t.Fatalf("acceptable passphrase rejected: %v", err)
	}
	if _, err := hasher.HashPassword([]byte(strings.Repeat("a", MaximumPasswordBytes))); err != nil {
		t.Fatalf("maximum-size password rejected: %v", err)
	}
}

func TestArgon2idVerifyRejectsMalformedEncodings(t *testing.T) {
	hasher := testPasswordHasher(t)
	for _, encoded := range []string{
		"not-a-hash",
		"$argon2i$v=19$m=8192,t=1,p=1$MTIzNDU2Nzg5MDEyMzQ1Ng$YWJjZGVmZ2hpamtsbW5vcA",
		"$argon2id$v=19$m=8192,t=1,p=1,p=1$MTIzNDU2Nzg5MDEyMzQ1Ng$YWJjZGVmZ2hpamtsbW5vcA",
		"$argon2id$v=19$m=999999,t=1,p=1$MTIzNDU2Nzg5MDEyMzQ1Ng$YWJjZGVmZ2hpamtsbW5vcA",
		"$argon2id$v=19$m=8192,t=1,p=257$MTIzNDU2Nzg5MDEyMzQ1Ng$YWJjZGVmZ2hpamtsbW5vcA",
	} {
		hash, _ := NewPasswordHash(encoded)
		if _, err := hasher.VerifyPassword(hash, []byte("correct horse battery staple")); !errors.Is(err, ErrMalformedPasswordHash) {
			t.Fatalf("malformed encoding %q returned %v", encoded, err)
		}
	}
}

func TestArgon2idParameterValidation(t *testing.T) {
	for _, parameters := range []Argon2idParameters{
		{MemoryKiB: 7, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32},
		{MemoryKiB: 8192, Iterations: 0, Parallelism: 1, SaltLength: 16, KeyLength: 32},
		{MemoryKiB: 8192, Iterations: 1, Parallelism: 1, SaltLength: 8, KeyLength: 32},
	} {
		if _, err := NewArgon2idHasher(parameters); !errors.Is(err, ErrMalformedPasswordHash) {
			t.Fatalf("invalid parameters accepted: %+v", parameters)
		}
	}
}
