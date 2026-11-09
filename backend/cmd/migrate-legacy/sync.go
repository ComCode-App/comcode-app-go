package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/cache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	migration "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/legacymigration"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"os"
	"strconv"
	"time"
)

func runSync(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	opts := migration.SyncOptions{Options: migration.Options{BatchID: uuid.NewString(), Account: "holding"}, Media: &migration.MediaSync{}, Client: &migration.ExportClient{}}
	fs.StringVar(&opts.UsersCSV, "users-csv", "", "offline users CSV[.gz]")
	fs.StringVar(&opts.HitsCSV, "hits-csv", "", "offline hits CSV[.gz]")
	fs.StringVar(&opts.MainTypesCSV, "main-link-types-csv", "", "offline main_link_types CSV[.gz]")
	fs.StringVar(&opts.Media.Dir, "media-dir", "", "extracted media folder (public_html prefix optional)")
	fs.StringVar(&opts.StateDir, "state-dir", "./var/legacy-sync", "private checkpoint/report directory")
	fs.StringVar(&opts.Client.URL, "export-url", os.Getenv("LEGACY_EXPORT_URL"), "export.php URL")
	fs.StringVar(&opts.Client.Token, "export-token", "", "prefer LEGACY_EXPORT_TOKEN env")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "report only; no database/storage/state writes")
	fs.BoolVar(&opts.MatchOwners, "match-owners", false, "match active personal accounts by email")
	fs.StringVar(&opts.Account, "account", "holding", "holding or existing account UUID")
	fs.BoolVar(&opts.PromoteOwnHash, "promote-own-hash", false, "move imported profiles whose username is their own hash from legacy-<uuid> to it (needs REDIS_ADDR)")
	reportPath := fs.String("report", "", "new report CSV path (default in state-dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if opts.Client.Token == "" {
		opts.Client.Token = os.Getenv("LEGACY_EXPORT_TOKEN")
	}
	if fs.NArg() != 0 {
		return errors.New("sync does not accept positional arguments")
	}
	if opts.UsersCSV == "" && (opts.Client.URL == "" || opts.Client.Token == "") {
		return errors.New("users-csv or LEGACY_EXPORT_URL and LEGACY_EXPORT_TOKEN required")
	}
	if os.Getenv("DATABASE_URL") == "" {
		return errors.New("DATABASE_URL required")
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		return errors.New("invalid DATABASE_URL")
	}
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return errors.New("cannot open database")
	}
	defer pool.Close()
	if !opts.DryRun {
		driver, err := storage.NewFromConfig(ctx, config.StorageFromEnv())
		if err != nil {
			return errors.New("cannot initialize storage driver")
		}
		opts.Media.Driver = driver
	}
	if opts.PromoteOwnHash && !opts.DryRun {
		// Promotions must evict cached legacy routes, or /{hash} would bounce off the retired name.
		if os.Getenv("REDIS_ADDR") == "" {
			return errors.New("--promote-own-hash requires REDIS_ADDR for route cache invalidation")
		}
		redisDB, err := strconv.Atoi(cmp.Or(os.Getenv("REDIS_DB"), "0"))
		if err != nil {
			return errors.New("invalid REDIS_DB")
		}
		rdb, err := cache.NewRedisClient(ctx, config.RedisConfig{Addr: os.Getenv("REDIS_ADDR"), Password: os.Getenv("REDIS_PASSWORD"), DB: redisDB})
		if err != nil {
			return errors.New("cannot reach the route cache (Redis)")
		}
		defer func() { _ = rdb.Close() }()
		opts.RouteCache = routecache.New(rdb, cmp.Or(os.Getenv("APP_ENV"), "development"), routecache.DefaultTTL)
	}
	if *reportPath == "" {
		if opts.DryRun {
			return errors.New("dry-run requires --report outside the state directory")
		}
		if err = os.MkdirAll(opts.StateDir, 0700); err != nil {
			return err
		}
		*reportPath = opts.StateDir + "/run-" + time.Now().UTC().Format("20060102T150405.000000000") + ".csv"
	}
	f, err := os.OpenFile(*reportPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	summary, runErr := migration.Sync(ctx, migration.NewPostgres(pool), opts, f)
	closeErr := f.Close()
	data, err := json.Marshal(summary)
	if err != nil {
		return errors.Join(runErr, closeErr, err)
	}
	_, printErr := fmt.Fprintf(out, "%s\ndry_run=%t report=%s\n", data, opts.DryRun, *reportPath)
	if runErr == nil {
		summaryFile, err := os.OpenFile(*reportPath+".summary.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return errors.Join(closeErr, printErr, err)
		}
		_, writeErr := summaryFile.Write(data)
		return errors.Join(closeErr, printErr, writeErr, summaryFile.Close())
	}
	return errors.Join(runErr, closeErr, printErr)
}
