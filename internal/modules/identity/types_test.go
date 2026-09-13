package identity

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{" Ada.Example@BTG.org ", "ada.example@btg.org"},
		{"mixed+TAG@Example.COM", "mixed+tag@example.com"},
	} {
		got, err := NormalizeEmail(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("NormalizeEmail(%q) = %q, %v; want %q", tc.input, got, err, tc.want)
		}
	}
	for _, input := range []string{"", " \t", "missing-at", "@example.org", "local@", "local@@example.org", "a b@example.org", "a@example.org\n" + "x"} {
		if _, err := NormalizeEmail(input); err == nil {
			t.Fatalf("expected invalid email for %q", input)
		}
	}
}

func TestStatusAndRoleValidation(t *testing.T) {
	for _, status := range []UserStatus{UserActive, UserSuspended} {
		if got, err := ParseUserStatus(string(status)); err != nil || got != status {
			t.Fatalf("status %q rejected: %v", status, err)
		}
	}
	if _, err := ParseUserStatus("DISABLED"); err == nil {
		t.Fatal("unknown status accepted")
	}
	if got, err := ParseGlobalRole("ADMINISTRATOR"); err != nil || got != RoleAdministrator {
		t.Fatalf("administrator rejected: %v", err)
	}
	if _, err := ParseGlobalRole("COURSE_AUTHOR"); err == nil {
		t.Fatal("course role accepted as global")
	}
}

func TestSecretsRedactFormatting(t *testing.T) {
	hash, err := NewPasswordHash("sensitive-test-hash")
	if err != nil {
		t.Fatal(err)
	}
	digest, err := NewSessionTokenDigest([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []any{hash, LocalPasswordCredential{PasswordHash: hash}, digest, Session{TokenDigest: digest}} {
		for _, format := range []string{"%v", "%+v", "%#v"} {
			printed := fmt.Sprintf(format, v)
			if strings.Contains(printed, "sensitive-test-hash") || strings.Contains(printed, "12345678901234567890123456789012") {
				t.Fatalf("secret leaked through %s", format)
			}
		}
		var output bytes.Buffer
		slog.New(slog.NewTextHandler(&output, nil)).Info("diagnostic", "value", v)
		if strings.Contains(output.String(), "sensitive-test-hash") || strings.Contains(output.String(), "12345678901234567890123456789012") {
			t.Fatal("secret leaked through slog")
		}
	}
}
