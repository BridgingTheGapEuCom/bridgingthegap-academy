package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

type passwordReaderFake struct {
	passwords [][]byte
	err       error
	calls     int
}

func (f *passwordReaderFake) ReadPassword(string) ([]byte, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	password := append([]byte(nil), f.passwords[f.calls-1]...)
	return password, nil
}

func TestAdminCreateAcceptsEmailAndDoesNotExposePassword(t *testing.T) {
	reader := &passwordReaderFake{passwords: [][]byte{[]byte("correct horse battery staple"), []byte("correct horse battery staple")}}
	var output bytes.Buffer
	var receivedEmail string
	var receivedPassword []byte
	err := runAdminCreate(context.Background(), []string{"create", "--email", "Admin@example.com"}, reader, &output, func(_ context.Context, email string, password []byte) (identity.User, error) {
		receivedEmail = email
		receivedPassword = append([]byte(nil), password...)
		return identity.User{ID: "00000000-0000-0000-0000-000000000001"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if receivedEmail != "Admin@example.com" || string(receivedPassword) != "correct horse battery staple" {
		t.Fatal("CLI did not pass supplied input to the service")
	}
	if !strings.Contains(output.String(), "Administrator created successfully: 00000000-0000-0000-0000-000000000001") || strings.Contains(output.String(), "correct horse battery staple") {
		t.Fatalf("unsafe success output: %q", output.String())
	}
}

func TestAdminCreateRejectsMismatchAndInvalidEmailBeforePersistence(t *testing.T) {
	called := false
	create := func(context.Context, string, []byte) (identity.User, error) {
		called = true
		return identity.User{}, nil
	}
	reader := &passwordReaderFake{passwords: [][]byte{[]byte("correct horse battery staple"), []byte("different horse battery staple")}}
	if err := runAdminCreate(context.Background(), []string{"create", "--email", "admin@example.com"}, reader, &bytes.Buffer{}, create); !errors.Is(err, errPasswordConfirmationMismatch) || called {
		t.Fatalf("mismatch was not rejected safely: %v", err)
	}
	if err := runAdminCreate(context.Background(), []string{"create", "--email", "invalid"}, reader, &bytes.Buffer{}, create); err == nil || reader.calls != 2 || called {
		t.Fatalf("invalid email reached password reader or service: %v", err)
	}
}

func TestAdminCreateMapsDuplicateAndRedactsFailures(t *testing.T) {
	for _, createErr := range []error{identity.ErrAdministratorAlreadyExists, errors.New("plaintext secret must not escape")} {
		reader := &passwordReaderFake{passwords: [][]byte{[]byte("correct horse battery staple"), []byte("correct horse battery staple")}}
		var output bytes.Buffer
		err := runAdminCreate(context.Background(), []string{"create", "--email", "admin@example.com"}, reader, &output, func(context.Context, string, []byte) (identity.User, error) { return identity.User{}, createErr })
		if err == nil || strings.Contains(err.Error(), "plaintext secret") || strings.Contains(output.String(), "correct horse battery staple") {
			t.Fatalf("unsafe command failure: %v output=%q", err, output.String())
		}
	}
}

func TestAdminCreateRequiresInteractivePasswordReader(t *testing.T) {
	reader := &passwordReaderFake{err: errPasswordTerminalRequired}
	err := runAdminCreate(context.Background(), []string{"create", "--email", "admin@example.com"}, reader, &bytes.Buffer{}, func(context.Context, string, []byte) (identity.User, error) { return identity.User{}, nil })
	if !errors.Is(err, errPasswordTerminalRequired) {
		t.Fatalf("noninteractive input did not fail clearly: %v", err)
	}
}

func TestTerminalPasswordReaderRejectsNonterminalInput(t *testing.T) {
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = input.Close() }()
	defer func() { _ = writer.Close() }()
	_, err = terminalPasswordReader{input: input, output: &bytes.Buffer{}}.ReadPassword("Password: ")
	if !errors.Is(err, errPasswordTerminalRequired) {
		t.Fatalf("pipe input was accepted as an interactive terminal: %v", err)
	}
}

type failingPasswordReader struct {
	values [][]byte
	failAt int
	calls  int
}

func (r *failingPasswordReader) ReadPassword(string) ([]byte, error) {
	r.calls++
	value := r.values[r.calls-1]
	if r.calls == r.failAt {
		return value, errors.New("private read failure")
	}
	return value, nil
}

func TestAdminCreateClearsPasswordBytesOnReaderFailure(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		first := []byte("correct horse battery staple")
		second := []byte("correct horse battery staple")
		reader := &failingPasswordReader{values: [][]byte{first, second}, failAt: failAt}
		err := runAdminCreate(context.Background(), []string{"create", "--email", "admin@example.com"}, reader, &bytes.Buffer{}, func(context.Context, string, []byte) (identity.User, error) {
			t.Fatal("persistence was called after password read failed")
			return identity.User{}, nil
		})
		if err == nil || strings.Contains(err.Error(), "private read failure") {
			t.Fatalf("password reader error was exposed: %v", err)
		}
		for _, value := range reader.values[:reader.calls] {
			if !bytes.Equal(value, make([]byte, len(value))) {
				t.Fatal("password bytes from failed read were retained")
			}
		}
	}
}
