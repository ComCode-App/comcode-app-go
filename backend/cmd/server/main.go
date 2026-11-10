package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/cache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/httpserver"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/logging"
	logsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/logs/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/providers"
	notifusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/errreport"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/events"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/mail"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/metrics"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/release"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/realtime"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("config load failed: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logging.New(cfg.Log.Level, cfg.Log.Format)
	// Baked release (TEC-1094) names Sentry events unless SENTRY_RELEASE overrides it.
	cfg.Observe.SentryRelease = release.SentryRelease(cfg.Observe.SentryRelease)
	rel := release.Current()
	log.Info("app_release", "version", rel.Version, "commit", rel.ShortCommit(), "built_at", rel.BuiltAt)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	flushErrors, err := errreport.Init(errreport.Config{
		DSN:         cfg.Observe.SentryDSN,
		Environment: cfg.Observe.SentryEnvironment,
		Release:     cfg.Observe.SentryRelease,
		ServerName:  cfg.App.Name + "-api",
	}, log)
	if err != nil {
		log.Error("error_reporting_init_failed", "error", err)
	}
	defer flushErrors()
	metrics.Serve(ctx, cfg.Observe.MetricsAddr, log)

	db, err := database.NewPostgresPool(ctx, cfg.DB)
	if err != nil {
		log.Error("database_init_failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	rdb, err := cache.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		log.Error("redis_init_failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	store, err := storage.NewFromConfig(ctx, cfg.Storage)
	if err != nil {
		log.Error("storage_init_failed", "error", err)
		os.Exit(1)
	}
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	if err := store.Ping(pingCtx); err != nil {
		pingCancel()
		log.Error("storage_ping_failed", "error", err)
		os.Exit(1)
	}
	pingCancel()

	var publisher realtime.Publisher
	if cfg.Centrifugo.Enabled {
		publisher = realtime.NewCentrifugoClient(cfg.Centrifugo)
	} else {
		publisher = realtime.NoopPublisher{}
	}

	queries := database.NewQueries(db)
	logsSvc := logsusecase.New(queries)
	persist := logging.Attach(log, logsSvc)
	log = persist.Logger()
	defer persist.Close()

	// Deploy provenance (TEC-1094): upsert this build's app_releases row. Fail-soft.
	release.RecordBoot(ctx, queries, log)

	eventBus := events.NewBus(log)
	mailer := mail.NewSMTPSender(cfg.SMTP)
	provs := []providers.Provider{
		providers.InappProvider{},
		providers.EmailProvider{Mail: mailer},
		providers.RealtimeProvider{Pub: publisher},
		providers.NoopProvider{Name: "sms", Log: log},
		providers.NoopProvider{Name: "push", Log: log},
	}

	var queueClient *queue.Client
	var worker *queue.Worker
	if cfg.Queue.Enabled {
		queueClient = queue.NewClient(cfg.Redis)
		notifSvc := notifusecase.New(queries, queueClient, provs, log).
			WithEncryptionKey(cfg.App.Env, cfg.Encryption.Key).
			WithExpo(notifusecase.ExpoConfig{AccessToken: cfg.Expo.AccessToken})
		if cfg.Queue.WorkerInProcess {
			worker = queue.NewWorker(cfg, log, notifSvc.Deliver).
				WithMobilePush(notifSvc.SendMobilePush, notifSvc.ProcessPushReceipts).
				WithLogPurge(logsSvc.ApplyDueRules)
			if n, err := notifSvc.ReclaimStuck(ctx, notifusecase.DefaultStuckProcessingMinutes); err != nil {
				log.Error("notification_reclaim_failed", "error", err)
			} else if n > 0 {
				log.Info("notification_reclaim_completed", "count", n)
			}
		}
	}

	srv, err := httpserver.New(cfg, log, httpserver.Deps{
		DB:       db,
		Queries:  queries,
		Redis:    rdb,
		Queue:    queueClient,
		Storage:  store,
		Realtime: publisher,
		Worker:   worker,
		Events:   eventBus,
	})
	if err != nil {
		log.Error("httpserver_init_failed", "error", err)
		os.Exit(1)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown_signal_received")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server_failed", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown_failed", "error", err)
		os.Exit(1)
	}
	log.Info("server_stopped")
}
