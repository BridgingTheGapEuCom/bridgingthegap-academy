package platform

import (
	"context"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

func TestBootstrapClearsPasswordWhenDatabaseIsUnavailable(t *testing.T) {
	password := []byte("private administrator passphrase")
	user, err := BootstrapAdministrator(context.Background(), nil, "admin@example.com", password, AdministratorBootstrapOptions{})
	if err != ErrAdministratorBootstrapUnavailable || user != (identity.User{}) {
		t.Fatalf("unavailable database did not fail safely: %v", err)
	}
	for _, value := range password {
		if value != 0 {
			t.Fatal("administrator password was retained after early failure")
		}
	}
}
