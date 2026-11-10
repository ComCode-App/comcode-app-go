package main

import (
	"context"
	billingrepository "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/repository"
	billingusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	webhooksusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/cache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/logging"
	accountsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/accounts/usecase"
	backgroundsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/usecase"
	bulkusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bulk/usecase"
	companybrandingusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/usecase"
	companyprofilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/usecase"
	errortrackingusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/errortracking/usecase"
	exportusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/exports/usecase"
	importusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/imports/usecase"
	leadsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/usecase"
	logsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/logs/usecase"
	notifmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/providers"
	notifusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	profilesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	profileverificationsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/aggregation"
	analyticsapp "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/application"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/maintenance"
	analyticsrealtime "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/realtime"
	analyticsstorage "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	bulkadapters "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/errreport"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/events"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/geoip"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	ioadapters "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/mail"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/metrics"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/outbox"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/release"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	searchadapters "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine/adapters"
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
	if !cfg.Queue.Enabled {
		log.Error("queue_disabled")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	flushErrors, err := errreport.Init(errreport.Config{
		DSN:         cfg.Observe.SentryDSN,
		Environment: cfg.Observe.SentryEnvironment,
		Release:     cfg.Observe.SentryRelease,
		ServerName:  cfg.App.Name + "-worker",
	}, log)
	if err != nil {
		log.Error("error_reporting_init_failed", "error", err)
	}
	defer flushErrors()
	metrics.Serve(ctx, cfg.Observe.MetricsAddr, log)

	pool, err := database.NewPostgresPool(ctx, cfg.DB)
	if err != nil {
		log.Error("database_init_failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	store, err := storage.NewFromConfig(ctx, cfg.Storage)
	if err != nil {
		log.Error("storage_init_failed", "error", err)
		os.Exit(1)
	}

	var publisher realtime.Publisher
	if cfg.Centrifugo.Enabled {
		publisher = realtime.NewCentrifugoClient(cfg.Centrifugo)
	} else {
		publisher = realtime.NoopPublisher{}
	}

	queries := database.NewQueries(pool)

	rdb, err := cache.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		log.Error("redis_init_failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	secretBox, err := crypto.NewSecretBox(cfg.Encryption.Key)
	if err != nil {
		log.Error("encryption_init_failed", "error", err)
		os.Exit(1)
	}

	// Panel-managed DSN (TEC-952): replaces the SENTRY_DSN bootstrap value once loaded and
	// follows later changes without a restart.
	errortrackingusecase.New(queries, secretBox, rdb, log, errortrackingusecase.Options{
		FallbackDSN: cfg.Observe.SentryDSN,
		Environment: cfg.Observe.SentryEnvironment,
		Release:     cfg.Observe.SentryRelease,
		Channel:     errortrackingusecase.ChannelName(cfg.App.Env),
	}).Start(ctx)

	geoSvc := geoip.NewService(queries, secretBox, rdb, log)
	eventStore := analyticsstorage.NewPostgresStore(pool, queries)
	counters := analyticsrealtime.NewCounterManager(rdb)
	aggregator := aggregation.NewAggregator(queries, log)
	maintMgr := maintenance.NewPartitionManager(pool, queries, log)
	workerProc := analyticsapp.NewWorkerProcessor(eventStore, counters, aggregator, maintMgr, log)

	mailer := mail.NewSMTPSender(cfg.SMTP)
	// The exchange visitor copy attaches the owner's vCard at send time (TEC-940); the profiles
	// service is set below once it exists.
	exchangeCopyFiles := &leadsusecase.CopyAttachments{Gone: profilesusecase.ErrNotFound}
	provs := []providers.Provider{
		providers.InappProvider{},
		providers.EmailProvider{Mail: mailer, Attachments: exchangeCopyFiles},
		providers.RealtimeProvider{Pub: publisher},
		providers.NoopProvider{Name: "sms", Log: log},
		providers.NoopProvider{Name: "push", Log: log},
	}
	notifSvc := notifusecase.New(queries, nil, provs, log).
		WithEncryptionKey(cfg.App.Env, cfg.Encryption.Key)
	notifSvc.WithVAPID(notifusecase.VAPIDConfig{
		PublicKey:  cfg.VAPID.PublicKey,
		PrivateKey: cfg.VAPID.PrivateKey,
		Subject:    cfg.VAPID.Subject,
	})
	notifSvc.WithExpo(notifusecase.ExpoConfig{AccessToken: cfg.Expo.AccessToken})

	eventBus := events.NewBus(log)
	notifmodule.RegisterEventHandlers(eventBus, notifSvc, log)
	outboxStore := outbox.NewStore(pool, queries)
	outboxPub := outbox.NewPublisher(outboxStore, eventBus, log)
	outboxStop := outboxPub.StartRun(ctx)
	defer outboxStop()

	activityRec := activity.NewRecorder(queries, log)
	// Employee profiles of company accounts (TEC-573): the worker writes through the same
	// profiles service as the API, including quotas, public pages and cache invalidation.
	profilesRepo := profilesrepo.NewPostgres(pool, queries)
	profilesSvc := profilesusecase.NewService(profilesRepo, store)
	exchangeCopyFiles.VCards = profilesSvc
	profilesSvc.SetQuotas(entitlement.New(queries))
	profilesSvc.SetPublicPages(profilesRepo)
	profilesSvc.SetRouteCache(routecache.New(rdb, cfg.App.Env, routecache.DefaultTTL))
	profilesSvc.SetActivity(activityRec)
	profilesSvc.SetBackgrounds(backgroundsusecase.New(queries))
	// Bulk logo actions reference the company branding (TEC-946).
	companyBrandingSvc := companybrandingusecase.New(queries, store)
	profilesSvc.SetAccountThemes(companyBrandingSvc)
	// Bulk edits and imports that rename a verified profile revoke its badge (TEC-549).
	profileVerificationsSvc := profileverificationsusecase.New(pool, queries, notifSvc, log)
	profileVerificationsSvc.SetProfileCache(routecache.New(rdb, cfg.App.Env, routecache.DefaultTTL))
	profileVerificationsSvc.SetFrontendURL(cfg.Auth.FrontendURL)
	profilesSvc.SetVerifications(profileVerificationsSvc)
	// Employees without a user get the profile and an email invite (TEC-945).
	companyImport := companyprofilesusecase.NewImportAdapter(queries, profilesSvc)
	companyImport.SetInviter(accountsusecase.NewInvites(pool, queries, notifSvc, cfg.Auth.FrontendURL, log))
	ioReg := ioengine.NewRegistry(
		ioadapters.NewUsers(queries),
		ioadapters.NewRoles(queries),
		ioadapters.NewNotifications(queries),
		ioadapters.NewActivity(queries),
		ioadapters.NewOrderManifest(queries, cfg.Auth.FrontendURL),
		ioadapters.NewLeads(queries),
		companyImport,
	)
	exportSvc := exportusecase.New(queries, store, ioReg, nil, notifSvc, activityRec, log)
	importSvc := importusecase.New(queries, store, ioReg, nil, notifSvc, activityRec, log)
	bulkReg := bulkengine.NewRegistry(
		bulkadapters.NewUsers(queries),
		bulkadapters.NewRoles(queries),
		companyprofilesusecase.NewBulkAdapter(queries, profilesSvc, store, companyBrandingSvc),
	)
	bulkSvc := bulkusecase.New(queries, bulkReg, nil, notifSvc, activityRec, cfg.Bulk, log)
	logsSvc := logsusecase.New(queries)
	searchReg := searchengine.NewRegistry(searchadapters.All(queries)...)
	searchClient := searchengine.NewClient(cfg.Search, log)
	searchIndexer := searchengine.NewIndexer(searchClient, searchReg, nil, log)
	profilesSvc.SetSearchIndexer(searchIndexer) // company bulk/import jobs
	persist := logging.Attach(log, logsSvc)
	log = persist.Logger()
	defer persist.Close()

	billingSvc := billingusecase.New(queries, payment.NewRegistry(payment.NewDBSettingsReader(queries, secretBox)),
		cfg.Auth.FrontendURL, "", log, billingrepository.NewPostgres(pool))
	notifmodule.RegisterBillingHandlers(eventBus, notifSvc, queries)
	// Outbound webhooks (TEC-603): the worker sends deliveries and schedules its own retries.
	queueClient := queue.NewClient(cfg.Redis)
	defer func() { _ = queueClient.Close() }()
	// Deliveries run inline here; mobile pushes go through the queue so Expo outages retry.
	notifSvc.WithPushQueue(queueClient)
	// Deleted profiles' media cleanup (TEC-1176): import rollbacks delete profiles here too.
	profileMediaCleaner := profilesusecase.NewMediaCleaner(store, queries, queueClient, log)
	profilesSvc.SetMediaCleaner(profileMediaCleaner)
	webhooksSvc := webhooksusecase.New(queries, secretBox, queueClient, log).
		AllowInsecure(cfg.App.Env == "development" || cfg.App.Env == "test")
	worker := queue.NewWorker(cfg, log, notifSvc.Deliver).
		WithWebhooks(webhooksSvc.Deliver).
		WithProfileMediaCleanup(profileMediaCleaner.Process).
		WithExport(exportSvc.ProcessExport).
		WithImport(importSvc.ProcessImport).
		WithBulk(bulkSvc.ProcessBulk).
		WithMobilePush(notifSvc.SendMobilePush, notifSvc.ProcessPushReceipts).
		WithLogPurge(func(ctx context.Context) error {
			if _, err := webhooksSvc.Prune(ctx, 0); err != nil {
				log.Warn("webhook_prune_failed", "error", err)
			}
			if _, err := webhooksSvc.Sweep(ctx); err != nil {
				log.Warn("webhook_sweep_failed", "error", err)
			}
			if n, err := notifSvc.PrunePushData(ctx); err != nil {
				log.Warn("push_cleanup_failed", "error", err)
			} else if n.Tickets+n.DisabledDevices+n.StaleDevices > 0 {
				log.Info("push_cleanup_completed", "tickets", n.Tickets,
					"disabled_devices", n.DisabledDevices, "stale_devices", n.StaleDevices)
			}
			return logsSvc.ApplyDueRules(ctx)
		}).
		WithBilling(func(ctx context.Context) error {
			if !cfg.Billing.RenewalEnabled {
				return nil
			}
			return billingSvc.ProcessRenewals(ctx, time.Now().UTC(), billingusecase.RenewalPolicy{
				RetryDays: cfg.Billing.RetryDays, GraceDays: cfg.Billing.GraceDays,
			})
		}).
		WithAnalytics(workerProc.ProcessEvent, workerProc.ProcessMaintenance).
		WithGeoIP(func(ctx context.Context) error {
			_, err := geoSvc.UpdateNow(ctx)
			return err
		}).
		WithSearch(
			searchIndexer.ProcessUpsert,
			searchIndexer.ProcessDelete,
			searchIndexer.ProcessReindex,
		)

	// Analytics partitions/retention also run on boot; the scheduler repeats them every 6h.
	go func() {
		if err := workerProc.ProcessMaintenance(ctx); err != nil {
			log.Error("analytics_maintenance_boot_failed", "error", err)
		}
	}()

	if searchIndexer.Enabled() {
		go searchIndexer.Bootstrap(ctx)
	}

	scheduler, err := queue.StartLogPurgeScheduler(cfg, log)
	if err != nil {
		log.Error("log_purge_scheduler_failed", "error", err)
		os.Exit(1)
	}
	go func() {
		if err := scheduler.Run(); err != nil {
			log.Error("log_purge_scheduler_stopped", "error", err)
		}
	}()

	log.Info("worker_started",
		"app", cfg.App.Name,
		"env", cfg.App.Env,
		"concurrency", cfg.Queue.Concurrency,
	)

	if n, err := notifSvc.ReclaimStuck(ctx, notifusecase.DefaultStuckProcessingMinutes); err != nil {
		log.Error("notification_reclaim_failed", "error", err)
	} else if n > 0 {
		log.Info("notification_reclaim_completed", "count", n)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- worker.Start()
	}()

	select {
	case <-ctx.Done():
		log.Info("worker_shutdown_signal")
		worker.Shutdown()
	case err := <-errCh:
		if err != nil {
			log.Error("worker_failed", "error", err)
			os.Exit(1)
		}
	}
	log.Info("worker_stopped")
}
