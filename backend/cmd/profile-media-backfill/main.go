// Command profile-media-backfill removes the storage objects of profiles that were soft-deleted
// before the deleted-profile media cleanup existed (TEC-1177, KVKK; the cleanup itself is TEC-1176).
//
// In the backend image (no source tree, no make):
//
//	/app/profile-media-backfill            # dry run: counts only, deletes nothing
//	/app/profile-media-backfill -apply     # enqueue app:profiles:media_cleanup per deleted profile
//
// With the queue on (QUEUE_ENABLED=true) -apply enqueues the existing cleanup task on the
// maintenance queue (TaskID dedupe) and the worker deletes the objects; with the queue off, or
// with -inline, the cleanup runs in this process. Only profiles that still have an unreferenced
// object are scheduled, so running it again after the cleanup finished is a no-op. -after-id
// resumes from the last_id a previous run printed. Output holds ids, uuids (in logs) and counts.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/logging"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
)

type options struct {
	apply     bool
	inline    bool
	batchSize int
	afterID   int64
	limit     int
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	fs := flag.NewFlagSet("profile-media-backfill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	apply := fs.Bool("apply", false, "schedule the cleanup; without it the run is a dry run that deletes nothing")
	inline := fs.Bool("inline", false, "with -apply: clean up in this process even when the queue is enabled")
	batch := fs.Int("batch-size", profilesusecase.DefaultBackfillBatchSize, "deleted profiles read per query (1-5000)")
	after := fs.Int64("after-id", 0, "resume after this profiles.id (last_id of a previous run)")
	limit := fs.Int("limit", 0, "stop after this many deleted profiles (0 = all)")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 0 {
		return options{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	o := options{apply: *apply, inline: *inline, batchSize: *batch, afterID: *after, limit: *limit}
	switch {
	case o.batchSize < 1 || o.batchSize > 5000:
		return options{}, errors.New("-batch-size must be between 1 and 5000")
	case o.afterID < 0:
		return options{}, errors.New("-after-id must not be negative")
	case o.limit < 0:
		return options{}, errors.New("-limit must not be negative")
	case o.inline && !o.apply:
		return options{}, errors.New("-inline needs -apply")
	}
	return o, nil
}

func main() {
	opts, err := parseOptions(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config load failed: "+err.Error())
		os.Exit(1)
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPostgresPool(ctx, cfg.DB)
	if err != nil {
		log.Error("database_init_failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	queries := database.NewQueries(pool)

	store, err := storage.NewFromConfig(ctx, cfg.Storage)
	if err != nil {
		log.Error("storage_init_failed", "error", err)
		os.Exit(1)
	}
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	err = store.Ping(pingCtx)
	pingCancel()
	if err != nil {
		log.Error("storage_ping_failed", "error", err)
		os.Exit(1)
	}

	mode := "dry-run"
	var enqueuer profilesusecase.MediaEnqueuer
	if opts.apply {
		mode = "apply-inline"
		if cfg.Queue.Enabled && !opts.inline {
			client := queue.NewClient(cfg.Redis)
			defer func() { _ = client.Close() }()
			enqueuer = client
			mode = "apply-queue"
		}
	}
	fmt.Printf("profile-media-backfill mode=%s batch_size=%d after_id=%d limit=%d\n",
		mode, opts.batchSize, opts.afterID, opts.limit)

	backfill := profilesusecase.NewMediaBackfill(store, queries, enqueuer, log)
	sum, err := backfill.Run(ctx, profilesusecase.BackfillOptions{
		Apply:     opts.apply,
		BatchSize: opts.batchSize,
		AfterID:   opts.afterID,
		Limit:     opts.limit,
		OnBatch: func(s profilesusecase.BackfillSummary) {
			fmt.Printf("batch last_id=%d deleted_profiles=%d deletable=%d\n", s.LastID, s.DeletedProfiles, s.Deletable)
		},
	})
	printSummary(os.Stdout, mode, sum)
	if err != nil {
		log.Error("profile_media_backfill_failed", "last_id", sum.LastID, "error", err)
		os.Exit(1)
	}
	if sum.Failed > 0 {
		os.Exit(1)
	}
}

func printSummary(w io.Writer, mode string, s profilesusecase.BackfillSummary) {
	_, _ = fmt.Fprintf(w, "summary mode=%s\n", mode)
	_, _ = fmt.Fprintf(w, "  deleted_profiles=%d with_deletable_media=%d\n", s.DeletedProfiles, s.WithMedia)
	_, _ = fmt.Fprintf(w, "  candidate_objects=%d deletable=%d skipped_referenced=%d\n", s.Candidates, s.Deletable, s.Skipped)
	if s.Apply {
		_, _ = fmt.Fprintf(w, "  enqueued=%d already_queued=%d processed_inline=%d deleted_objects=%d\n",
			s.Enqueued, s.AlreadyQueued, s.ProcessedInline, s.DeletedObjects)
	} else {
		_, _ = fmt.Fprintln(w, "  dry run: nothing was deleted or enqueued (pass -apply)")
	}
	_, _ = fmt.Fprintf(w, "  failed=%d last_id=%d\n", s.Failed, s.LastID)
}
