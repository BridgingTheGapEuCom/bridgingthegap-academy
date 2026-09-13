package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	MinimumPasswordLength = 12
	MaximumPasswordBytes  = 1024
)

var ErrInvalidPassword = errors.New("password must be at least 12 characters and no more than 1024 bytes")
var ErrMalformedPasswordHash = errors.New("malformed password hash")

// Argon2idParameters are stored in each PHC-format hash so default changes do
// not invalidate credentials created by earlier versions of the application.
type Argon2idParameters struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2idParameters use 64 MiB, three iterations, and one lane. This
// is a conservative interactive-login baseline that avoids multiplying memory
// use for concurrent requests. Reassess these defaults with production sizing.
var DefaultArgon2idParameters = Argon2idParameters{
	MemoryKiB:   64 * 1024,
	Iterations:  3,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

type PasswordHasher interface {
	HashPassword([]byte) (PasswordHash, error)
	VerifyPassword(PasswordHash, []byte) (bool, error)
}

type Argon2idHasher struct{ parameters Argon2idParameters }

func NewArgon2idHasher(parameters Argon2idParameters) (Argon2idHasher, error) {
	if err := validateArgon2idParameters(parameters); err != nil {
		return Argon2idHasher{}, err
	}
	return Argon2idHasher{parameters: parameters}, nil
}

func DefaultPasswordHasher() Argon2idHasher {
	hasher, err := NewArgon2idHasher(DefaultArgon2idParameters)
	if err != nil {
		panic(err)
	}
	return hasher
}

func ValidatePassword(plaintext []byte) error {
	if len(plaintext) > MaximumPasswordBytes || !utf8.Valid(plaintext) || utf8.RuneCount(plaintext) < MinimumPasswordLength {
		return ErrInvalidPassword
	}
	return nil
}

func (h Argon2idHasher) HashPassword(plaintext []byte) (PasswordHash, error) {
	if err := ValidatePassword(plaintext); err != nil {
		return PasswordHash{}, err
	}
	salt := make([]byte, h.parameters.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return PasswordHash{}, errors.New("generate password salt")
	}
	derived := argon2.IDKey(plaintext, salt, h.parameters.Iterations, h.parameters.MemoryKiB, h.parameters.Parallelism, h.parameters.KeyLength)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, h.parameters.MemoryKiB, h.parameters.Iterations, h.parameters.Parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived))
	return NewPasswordHash(encoded)
}

func (h Argon2idHasher) VerifyPassword(encoded PasswordHash, plaintext []byte) (bool, error) {
	if err := ValidatePassword(plaintext); err != nil {
		return false, err
	}
	parameters, salt, expected, err := parseArgon2idHash(encoded.Value())
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey(plaintext, salt, parameters.Iterations, parameters.MemoryKiB, parameters.Parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func validateArgon2idParameters(parameters Argon2idParameters) error {
	if parameters.MemoryKiB < 8 || parameters.MemoryKiB > 512*1024 || parameters.Iterations == 0 || parameters.Iterations > 10 || parameters.Parallelism == 0 || parameters.Parallelism > 16 || parameters.MemoryKiB < 8*uint32(parameters.Parallelism) || parameters.SaltLength < 16 || parameters.SaltLength > 64 || parameters.KeyLength < 16 || parameters.KeyLength > 64 {
		return ErrMalformedPasswordHash
	}
	return nil
}

func parseArgon2idHash(encoded string) (Argon2idParameters, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return Argon2idParameters{}, nil, nil, ErrMalformedPasswordHash
	}
	parameters := Argon2idParameters{}
	seen := map[string]bool{}
	for _, parameter := range strings.Split(parts[3], ",") {
		name, value, found := strings.Cut(parameter, "=")
		if !found {
			return Argon2idParameters{}, nil, nil, ErrMalformedPasswordHash
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return Argon2idParameters{}, nil, nil, ErrMalformedPasswordHash
		}
		if seen[name] {
			return Argon2idParameters{}, nil, nil, ErrMalformedPasswordHash
		}
		seen[name] = true
		switch name {
		case "m":
			parameters.MemoryKiB = uint32(parsed)
		case "t":
			parameters.Iterations = uint32(parsed)
		case "p":
			if parsed > 255 {
				return Argon2idParameters{}, nil, nil, ErrMalformedPasswordHash
			}
			parameters.Parallelism = uint8(parsed)
		default:
			return Argon2idParameters{}, nil, nil, ErrMalformedPasswordHash
		}
	}
	if !seen["m"] || !seen["t"] || !seen["p"] {
		return Argon2idParameters{}, nil, nil, ErrMalformedPasswordHash
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return Argon2idParameters{}, nil, nil, ErrMalformedPasswordHash
	}
	expected, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return Argon2idParameters{}, nil, nil, ErrMalformedPasswordHash
	}
	parameters.SaltLength = uint32(len(salt))
	parameters.KeyLength = uint32(len(expected))
	if err := validateArgon2idParameters(parameters); err != nil {
		return Argon2idParameters{}, nil, nil, ErrMalformedPasswordHash
	}
	return parameters, salt, expected, nil
}
