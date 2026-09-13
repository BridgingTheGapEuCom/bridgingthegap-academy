package platform

import "testing"

func TestLoadConfigRequiresDatabaseAndValidAddresses(t *testing.T) {
	t.Setenv("BTG_LMS_DATABASE_URL", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("accepted missing database URL")
	}
	t.Setenv("BTG_LMS_DATABASE_URL", "postgres://localhost/btg")
	t.Setenv("BTG_LMS_HTTP_ADDR", "invalid-address")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("accepted invalid HTTP address")
	}
}
