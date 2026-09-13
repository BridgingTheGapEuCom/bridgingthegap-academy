package sqlc

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestGeneratedSensitiveTypesRedactFormatting(t *testing.T) {
	secret := "sensitive-test-value"
	for _, value := range []any{
		IdentityLocalPasswordCredential{PasswordHash: secret},
		CreateLocalPasswordCredentialParams{PasswordHash: secret},
		ReplaceLocalPasswordHashParams{PasswordHash: secret},
		IdentitySession{TokenDigest: []byte(secret)},
		CreateSessionParams{TokenDigest: []byte(secret)},
	} {
		for _, format := range []string{"%v", "%+v", "%#v"} {
			printed := fmt.Sprintf(format, value)
			if strings.Contains(printed, secret) || !strings.Contains(printed, "[REDACTED]") {
				t.Fatalf("sensitive generated type leaked through %s", format)
			}
		}
		var output bytes.Buffer
		slog.New(slog.NewTextHandler(&output, nil)).Info("diagnostic", "value", value)
		if strings.Contains(output.String(), secret) || !strings.Contains(output.String(), "[REDACTED]") {
			t.Fatal("sensitive generated type leaked through slog")
		}
	}
}
