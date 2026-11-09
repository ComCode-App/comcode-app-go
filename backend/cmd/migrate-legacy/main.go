// migrate-legacy imports normalized legacy card identities and verifies their public resolution.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	routerepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	migration "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/legacymigration"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) > 0 && args[0] == "owners" {
		return runOwners(ctx, args[1:], out)
	}
	if len(args) > 0 && args[0] == "sync" {
		return runSync(ctx, args[1:], out)
	}
	if len(args) == 0 || (args[0] != "import" && args[0] != "verify") {
		return errors.New("usage: migrate-legacy owners|sync|import|verify --input FILE --report FILE [options]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	input := fs.String("input", "", "normalized CSV or JSONL file")
	format := fs.String("format", "", "csv or jsonl (default: file extension)")
	report := fs.String("report", "", "new CSV report file (required, refuses overwrite)")
	opts := migration.Options{}
	fs.StringVar(&opts.BatchID, "batch-id", uuid.NewString(), "import batch ID for new records only")
	fs.IntVar(&opts.BatchSize, "batch-size", 500, "rows per transaction (1–10000)")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "read-only planning; import only")
	fs.StringVar(&opts.Account, "account", "holding", "holding or an existing account UUID")
	fs.BoolVar(&opts.MatchOwners, "match-owners", false, "match active users by email to their personal account; fallback to --account")
	fs.StringVar(&opts.FrontendURL, "frontend-url", os.Getenv("PUBLIC_FRONTEND_URL"), "public frontend base URL (required)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *input == "" || *report == "" || opts.FrontendURL == "" {
		return errors.New("input, report and frontend-url (or PUBLIC_FRONTEND_URL) are required; no positional arguments")
	}
	base, parseErr := url.Parse(opts.FrontendURL)
	if parseErr != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Hostname() == "" || base.User != nil || (base.Path != "" && base.Path != "/") || base.RawQuery != "" || base.Fragment != "" {
		return errors.New("frontend-url must be an absolute HTTP(S) origin")
	}
	if args[0] == "verify" && opts.DryRun {
		return errors.New("dry-run applies only to import")
	}
	if opts.BatchSize < 1 || opts.BatchSize > 10000 {
		return errors.New("batch-size must be 1–10000")
	}
	if opts.Account != "holding" {
		if _, err := uuid.Parse(opts.Account); err != nil {
			return errors.New("account must be holding or an account UUID")
		}
	}
	if *format == "" {
		*format = strings.TrimPrefix(strings.ToLower(filepath.Ext(*input)), ".")
	}
	in, err := os.Open(*input)
	if err != nil {
		return fmt.Errorf("input: %w", err)
	}
	defer func() { _ = in.Close() }()
	reader, err := migration.NewReader(in, *format)
	if err != nil {
		return err
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required (load the repo-root .env explicitly)")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return errors.New("invalid DATABASE_URL")
	}
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return errors.New("cannot open database")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return errors.New("cannot connect to database")
	}
	f, err := os.OpenFile(*report, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	repo := migration.NewPostgres(pool)
	var workErr error
	if args[0] == "import" {
		var summary migration.Summary
		summary, workErr = migration.Import(ctx, repo, reader, opts, f, out)
		if _, err := fmt.Fprintf(out, "rows=%d created=%d skipped=%d warnings=%d dry_run=%t batch=%s\nblocked: collides with app path\ncount=%d (see blocked rows in CSV report)\n", summary.Rows, summary.Created, summary.Skipped, summary.Pending, opts.DryRun, opts.BatchID, summary.Blocked); err != nil {
			workErr = errors.Join(workErr, err)
		}
	} else {
		resolver := usecase.NewService(routerepo.NewPostgres(db.New(pool)), opts.FrontendURL, nil)
		_, workErr = migration.Verify(ctx, repo, resolver, reader, opts, f)
	}
	return errors.Join(workErr, f.Close())
}
