// Command create-super-admin bootstraps a platform super_admin user.
//
// Usage (repo root, local development):
//
//	make create-super-admin SA_EMAIL=admin@example.com SA_PASSWORD='Password1'
//
// In the backend image (no source tree, no make), with the password in the
// environment so it stays out of the process list:
//
//	SA_PASSWORD=... /app/create-super-admin -email admin@example.com
//
// -email and a password (-password or SA_PASSWORD) are required; the password
// must pass the platform password policy. There are no built-in defaults, so a
// bare run cannot create an admin with a known password.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database"
	authrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/password"
)

type options struct {
	email    string
	password string
	name     string
	surname  string
}

// parseOptions reads flags (and SA_EMAIL / SA_PASSWORD as fallbacks) and
// validates them. It never fills in a default email or password.
func parseOptions(args []string, getenv func(string) string, stderr io.Writer) (options, error) {
	fs := flag.NewFlagSet("create-super-admin", flag.ContinueOnError)
	fs.SetOutput(stderr)
	email := fs.String("email", "", "super admin email (required; or set SA_EMAIL)")
	pass := fs.String("password", "", "super admin password (required; or set SA_PASSWORD); min 8 chars with upper, lower and digit")
	name := fs.String("name", "Platform", "first name")
	surname := fs.String("surname", "Admin", "last name")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 0 {
		return options{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	o := options{
		email:    strings.ToLower(strings.TrimSpace(*email)),
		password: *pass,
		name:     strings.TrimSpace(*name),
		surname:  strings.TrimSpace(*surname),
	}
	if o.email == "" {
		o.email = strings.ToLower(strings.TrimSpace(getenv("SA_EMAIL")))
	}
	if o.password == "" {
		o.password = getenv("SA_PASSWORD")
	}
	if o.email == "" {
		return options{}, errors.New("email is required (-email or SA_EMAIL)")
	}
	if !strings.Contains(o.email, "@") {
		return options{}, fmt.Errorf("email %q is not valid", o.email)
	}
	if o.password == "" {
		return options{}, errors.New("password is required (-password or SA_PASSWORD)")
	}
	if err := password.ValidatePassword(o.password); err != nil {
		return options{}, err
	}
	if o.name == "" || o.surname == "" {
		return options{}, errors.New("name and surname must not be empty")
	}
	return o, nil
}

func main() {
	opts, err := parseOptions(os.Args[1:], os.Getenv, os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fatal("%v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		fatal("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.NewPostgresPool(ctx, cfg.DB)
	if err != nil {
		fatal("database: %v", err)
	}
	defer pool.Close()

	hash, err := password.Hash(opts.password)
	if err != nil {
		fatal("password: %v", err)
	}

	repo := authrepo.NewPostgres(pool, database.NewQueries(pool))
	user, created, err := repo.UpsertSuperAdmin(ctx, opts.email, opts.name, opts.surname, hash)
	if err != nil {
		fatal("%v", err)
	}

	action := "updated"
	if created {
		action = "created"
	}
	fmt.Printf("super_admin %s successfully\n", action)
	fmt.Printf("  uuid:   %s\n", user.UUID)
	fmt.Printf("  email:  %s\n", user.Email)
	fmt.Printf("  name:   %s %s\n", user.Name, user.Surname)
	fmt.Printf("  status: %s\n", user.Status)
	fmt.Printf("  role:   super_admin\n")
}

func fatal(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "create-super-admin: "+format+"\n", args...)
	os.Exit(1)
}
