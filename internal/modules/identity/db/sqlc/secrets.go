package sqlc

import "fmt"

// sqlc emits exported fields for database values. Keep diagnostic formatting of
// the generated rows and write parameters from exposing credential material.
func (IdentityLocalPasswordCredential) Format(s fmt.State, _ rune) {
	_, _ = s.Write([]byte("IdentityLocalPasswordCredential{[REDACTED]}"))
}

func (CreateLocalPasswordCredentialParams) Format(s fmt.State, _ rune) {
	_, _ = s.Write([]byte("CreateLocalPasswordCredentialParams{[REDACTED]}"))
}

func (ReplaceLocalPasswordHashParams) Format(s fmt.State, _ rune) {
	_, _ = s.Write([]byte("ReplaceLocalPasswordHashParams{[REDACTED]}"))
}

func (IdentitySession) Format(s fmt.State, _ rune) {
	_, _ = s.Write([]byte("IdentitySession{[REDACTED]}"))
}

func (CreateSessionParams) Format(s fmt.State, _ rune) {
	_, _ = s.Write([]byte("CreateSessionParams{[REDACTED]}"))
}

func (InitializeSessionCSRFTokenParams) Format(s fmt.State, _ rune) {
	_, _ = s.Write([]byte("InitializeSessionCSRFTokenParams{[REDACTED]}"))
}
