package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	migration "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/legacymigration"
	"github.com/jackc/pgx/v5/pgxpool"
)

func runOwners(ctx context.Context, args []string, out io.Writer) error {
	rollback := len(args) > 0 && args[0] == "rollback"
	if rollback {
		args = args[1:]
	}
	fs := flag.NewFlagSet("owners", flag.ContinueOnError)
	opts := migration.OwnersOptions{Client: &migration.ExportClient{}}
	undo := migration.OwnersRollbackOptions{}
	reportPath := ""
	if rollback {
		fs.StringVar(
			&undo.Batch,
			"batch",
			"",
			"assignment batch UUID",
		)
		fs.BoolVar(
			&undo.All,
			"all",
			false,
			"roll back all batches, newest first",
		)
		fs.BoolVar(
			&undo.Force,
			"force",
			false,
			"delete created accounts even if they contain later data; preserves row conflicts",
		)
	} else {
		fs.StringVar(
			&opts.UsersCSV,
			"users-csv",
			"",
			"offline users CSV[.gz]",
		)
		fs.StringVar(
			&opts.Client.URL,
			"export-url",
			os.Getenv("LEGACY_EXPORT_URL"),
			"export.php URL",
		)
		fs.StringVar(
			&opts.Client.Token,
			"export-token",
			"",
			"prefer LEGACY_EXPORT_TOKEN env",
		)
		fs.StringVar(
			&opts.OverridesCSV,
			"overrides",
			"",
			"legacy_user_id,group CSV",
		)
		fs.StringVar(
			&reportPath,
			"report",
			"",
			"new private report CSV (required)",
		)
		fs.BoolVar(
			&opts.DryRun,
			"dry-run",
			false,
			"report only; no database writes",
		)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("owners accepts no positional arguments")
	}
	if opts.Client.Token == "" {
		opts.Client.Token = os.Getenv("LEGACY_EXPORT_TOKEN")
	}
	if !rollback && (reportPath == "" || (opts.UsersCSV == "" && (opts.Client.URL == "" || opts.Client.Token == ""))) {
		return errors.New("owners requires --report and --users-csv or LEGACY_EXPORT_URL/LEGACY_EXPORT_TOKEN")
	}
	if os.Getenv("DATABASE_URL") == "" {
		return errors.New("DATABASE_URL required")
	}
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return errors.New("cannot open database")
	}
	defer pool.Close()
	repo := migration.NewPostgres(pool)
	if rollback {
		summary, runErr := migration.RollbackOwners(ctx, repo, undo)
		return errors.Join(runErr, json.NewEncoder(out).Encode(summary))
	}
	f, err := os.OpenFile(reportPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	summary, runErr := migration.Owners(
		ctx,
		repo,
		opts,
		f,
	)
	return errors.Join(runErr, f.Close(), json.NewEncoder(out).Encode(summary))
}
