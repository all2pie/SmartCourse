// Command identity-admin creates an admin account. Run it once per environment:
//
//	task admin:create -- -email admin@example.com -name "Site Admin"
//
// The password is read from the terminal without echo, or from stdin when piped,
// so it never appears in shell history, process lists or the service's environment.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/all2pie/go-smart-course/internal/identity"
	"github.com/all2pie/go-smart-course/internal/platform/config"
	"github.com/all2pie/go-smart-course/internal/platform/postgres"
)

// adminConfig is the subset of Identity's settings this command needs: only the database.
type adminConfig struct {
	DB postgres.Config `envPrefix:"IDENTITY_DB_"`
}

func main() {
	email := flag.String("email", "", "admin email (required)")
	name := flag.String("name", "Administrator", "display name")
	flag.Parse()
	if *email == "" {
		flag.Usage()
		os.Exit(2)
	}

	password, err := readPassword()
	if err != nil {
		log.Fatalf("reading password: %v", err)
	}

	cfg, err := config.Load[adminConfig]()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := postgres.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	svc := identity.NewService(identity.NewRepository(db))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	u, err := svc.CreateAdmin(ctx, *name, *email, password)
	cancel()
	if errors.Is(err, identity.ErrEmailTaken) {
		// Idempotent: safe to re-run from setup scripts.
		fmt.Printf("a user with email %s already exists; nothing changed\n", *email)
		return
	}
	if err != nil {
		log.Fatalf("creating admin: %v", err)
	}

	fmt.Printf("admin created: id=%s email=%s\n", u.ID, u.Email)
}

func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}

	fmt.Fprint(os.Stderr, "Confirm password: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}

	if string(first) != string(second) {
		return "", errors.New("passwords do not match")
	}
	return string(first), nil
}
