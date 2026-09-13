package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/platform"
	"golang.org/x/term"
)

var version = "dev"

var errPasswordTerminalRequired = errors.New("administrator creation requires an interactive terminal for password entry")
var errPasswordConfirmationMismatch = errors.New("password confirmation does not match")
var errAdministratorDatabaseUnavailable = errors.New("database unavailable")
var errAdministratorSchemaIncompatible = errors.New("database schema is unavailable or incompatible; run btg-lms migrate")
var errAdministratorPersistenceFailure = errors.New("administrator persistence failed; no account was created")

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: btg-lms <serve|migrate|doctor|version|admin create>")
	}
	if args[0] == "version" {
		if len(args) != 1 {
			return errors.New("unexpected arguments")
		}
		fmt.Println(version)
		return nil
	}
	cfg, err := platform.LoadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch args[0] {
	case "serve":
		if len(args) != 1 {
			return errors.New("unexpected arguments")
		}
		return platform.Serve(ctx, cfg, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	case "migrate":
		if len(args) != 1 {
			return errors.New("unexpected arguments")
		}
		return postgres.Migrate(ctx, cfg.DatabaseURL)
	case "doctor":
		if len(args) != 1 {
			return errors.New("unexpected arguments")
		}
		return platform.Doctor(ctx, cfg, os.Stdout)
	case "admin":
		return runAdminCreate(ctx, args[1:], terminalPasswordReader{input: os.Stdin, output: os.Stdout}, os.Stdout, func(ctx context.Context, email string, password []byte) (identity.User, error) {
			pool, err := postgres.OpenPool(ctx, cfg.DatabaseURL)
			if err != nil {
				return identity.User{}, errAdministratorDatabaseUnavailable
			}
			defer pool.Close()
			if err := postgres.SchemaCheck(ctx, pool); err != nil {
				return identity.User{}, errAdministratorSchemaIncompatible
			}
			user, err := platform.BootstrapAdministrator(ctx, pool, email, password, platform.AdministratorBootstrapOptions{})
			if err == nil {
				slog.New(slog.NewTextHandler(os.Stderr, nil)).Info("administrator bootstrap completed", "user_id", user.ID)
			}
			return user, err
		})
	default:
		return errors.New("unknown command: " + args[0])
	}
}

type passwordReader interface {
	ReadPassword(string) ([]byte, error)
}

type terminalPasswordReader struct {
	input  *os.File
	output io.Writer
}

func (r terminalPasswordReader) ReadPassword(prompt string) ([]byte, error) {
	if !term.IsTerminal(int(r.input.Fd())) {
		return nil, errPasswordTerminalRequired
	}
	if _, err := fmt.Fprint(r.output, prompt); err != nil {
		return nil, errors.New("write password prompt")
	}
	password, err := term.ReadPassword(int(r.input.Fd()))
	if _, writeErr := fmt.Fprintln(r.output); err == nil && writeErr != nil {
		return nil, errors.New("write password prompt")
	}
	if err != nil {
		return nil, errors.New("read password")
	}
	return password, nil
}

func runAdminCreate(ctx context.Context, args []string, passwords passwordReader, output io.Writer, create func(context.Context, string, []byte) (identity.User, error)) error {
	flags := flag.NewFlagSet("btg-lms admin create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	email := flags.String("email", "", "administrator email")
	if len(args) == 0 || args[0] != "create" {
		return errors.New("usage: btg-lms admin create --email admin@example.com")
	}
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *email == "" {
		return errors.New("usage: btg-lms admin create --email admin@example.com")
	}
	if _, err := identity.NormalizeEmail(*email); err != nil {
		return errors.New("invalid email")
	}
	password, err := passwords.ReadPassword("Password: ")
	if err != nil {
		if errors.Is(err, errPasswordTerminalRequired) {
			return errPasswordTerminalRequired
		}
		return errors.New("unable to read password")
	}
	defer clear(password)
	confirmation, err := passwords.ReadPassword("Confirm password: ")
	if err != nil {
		return errors.New("unable to read password")
	}
	defer clear(confirmation)
	if len(password) != len(confirmation) || subtle.ConstantTimeCompare(password, confirmation) != 1 {
		return errPasswordConfirmationMismatch
	}
	user, err := create(ctx, *email, password)
	if err != nil {
		return administratorCreateError(err)
	}
	_, err = fmt.Fprintf(output, "Administrator created successfully: %s\n", user.ID)
	return err
}

func administratorCreateError(err error) error {
	switch {
	case errors.Is(err, identity.ErrInvalidPassword):
		return identity.ErrInvalidPassword
	case errors.Is(err, identity.ErrAdministratorAlreadyExists):
		return identity.ErrAdministratorAlreadyExists
	case errors.Is(err, errAdministratorDatabaseUnavailable):
		return errAdministratorDatabaseUnavailable
	case errors.Is(err, errAdministratorSchemaIncompatible):
		return errAdministratorSchemaIncompatible
	case errors.Is(err, platform.ErrAdministratorBootstrapUnavailable):
		return errAdministratorDatabaseUnavailable
	default:
		return errAdministratorPersistenceFailure
	}
}
