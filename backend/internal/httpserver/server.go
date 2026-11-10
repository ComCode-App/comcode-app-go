package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	backgroundsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds"
	backgroundshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/handler"
	backgroundsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/usecase"
	companyanalyticsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics"
	companyanalyticshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/handler"
	companyanalyticsrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/repository"
	companyanalyticsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/usecase"
	assetsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets"
	assetshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/handler"
	assetsrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/repository"
	assetsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/appversion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/cache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	accessmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/access"
	accesshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/access/handler"
	accountsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/accounts"
	accountshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/accounts/handler"
	accountsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/accounts/usecase"
	activitymodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/activity"
	activityhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/activity/handler"
	activityusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/activity/usecase"
	analyticsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/analytics"
	analyticshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/analytics/handler"
	authmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth"
	authhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/identity"
	authrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	authusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	authsettingsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/authsettings"
	authsettingshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/authsettings/handler"
	authsettingsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/authsettings/usecase"
	bankaccountsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts"
	bankaccountshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts/handler"
	bankaccountsrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts/repository"
	bankaccountsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts/usecase"
	banksmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/banks"
	bankshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/banks/handler"
	banksrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/banks/repository"
	banksusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/banks/usecase"
	billingmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing"
	billinghandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/handler"
	billingrepository "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/repository"
	billingusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	bulkmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bulk"
	bulkhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bulk/handler"
	bulkusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bulk/usecase"
	cardsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards"
	cardshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/handler"
	cardsrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/repository"
	cardsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/usecase"
	categoriesmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories"
	categorieshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/handler"
	categoriesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/repository"
	categoriesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/usecase"
	companybrandingmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding"
	companybrandinghandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/handler"
	companybrandingusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/usecase"
	companyprofilesmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles"
	companyprofileshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/handler"
	companyprofilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/usecase"
	structuremodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure"
	structurehandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/handler"
	structurerepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/repository"
	structureusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/usecase"
	errortrackingmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/errortracking"
	errortrackinghandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/errortracking/handler"
	errortrackingusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/errortracking/usecase"
	eventsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/events"
	eventshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/events/handler"
	exportmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/exports"
	exporthandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/exports/handler"
	exportusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/exports/usecase"
	geoipmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/geoip"
	geoiphandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/geoip/handler"
	importmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/imports"
	importhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/imports/handler"
	importusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/imports/usecase"
	githubmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/github"
	githubhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/github/handler"
	githubusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/github/usecase"
	oauthprovidermodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/oauthprovider"
	oauthproviderhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/oauthprovider/handler"
	oauthproviderusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/oauthprovider/usecase"
	paymentintegrationmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/payment"
	paymentintegrationhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/payment/handler"
	paymentintegrationusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/payment/usecase"
	leadsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads"
	leadshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/handler"
	leadsrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/repository"
	leadsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/usecase"
	logsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/logs"
	logshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/logs/handler"
	logsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/logs/usecase"
	notifmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications"
	notifhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/providers"
	notifusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	ordersmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders"
	ordershandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/handler"
	ordersrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	ordersusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/usecase"
	overviewmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview"
	overviewhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/handler"
	overviewrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/repository"
	overviewusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/usecase"
	plansmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/plans"
	planshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/plans/handler"
	plansusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/plans/usecase"
	platformsupportmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport"
	platformsupporthandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/handler"
	platformsupportrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/repository"
	platformsupportusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/usecase"
	productsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products"
	productshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/handler"
	productsrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/repository"
	productsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/usecase"
	profilesmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles"
	profileshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/handler"
	profilesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	profileverificationsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications"
	profileverificationshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications/handler"
	profileverificationsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications/usecase"
	routesmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes"
	routeshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/handler"
	routesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/repository"
	routesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	searchmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/search"
	searchhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/search/handler"
	searchusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/search/usecase"
	settingsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/settings"
	settingshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/settings/handler"
	settingsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/settings/usecase"
	socialplatformsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/socialplatforms"
	socialplatformshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/socialplatforms/handler"
	socialplatformsrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/socialplatforms/repository"
	socialplatformsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/socialplatforms/usecase"
	storagemodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/storage"
	storagehandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/storage/handler"
	storageusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/storage/usecase"
	subscriptionsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/subscriptions"
	subscriptionshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/subscriptions/handler"
	subscriptionsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/subscriptions/usecase"
	utilsmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/utils"
	utilshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/utils/handler"
	webhooksmodule "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks"
	webhookshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks/handler"
	webhooksusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/aggregation"
	analyticsapp "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/application"
	analyticsdelivery "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/delivery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/device"
	analyticsdomain "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	analyticsinfra "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/infrastructure"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/ingestion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/maintenance"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/privacy"
	analyticsrealtime "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/realtime"
	analyticsregistry "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/registry"
	analyticsstorage "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/appleauth"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	bulkadapters "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/events"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/geoip"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	ioadapters "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/mail"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/oidc"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/opengraph"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/outbox"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ratelimit"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	searchadapters "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/stepup"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/realtime"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Deps holds infrastructure clients wired into the HTTP server.
type Deps struct {
	DB       *pgxpool.Pool
	Queries  *db.Queries
	Redis    *redis.Client
	Queue    *queue.Client
	Storage  storage.Driver
	Realtime realtime.Publisher
	Worker   *queue.Worker
	Events   events.Bus
}

// Server is the HTTP composition root for infrastructure routes.
type Server struct {
	cfg           config.Config
	log           *slog.Logger
	db            *pgxpool.Pool
	queries       *db.Queries
	redis         *redis.Client
	queueClient   *queue.Client
	storage       storage.Driver
	realtime      realtime.Publisher
	worker        *queue.Worker
	events        events.Bus
	outboxPub     *outbox.Publisher
	outboxStop    func()
	errTracking   *errortrackingusecase.Service
	errTrackStop  context.CancelFunc
	searchClient  *searchengine.Client
	searchIndexer *searchengine.Indexer
	// backgrounds renders missing design thumbnails at startup (TEC-599).
	backgrounds  *backgroundsusecase.Service
	githubSvc    *githubusecase.Service
	oauthProvSvc *oauthproviderusecase.Service
	authSettings *authsettingsusecase.Service
	http         *http.Server
}

// New wires router and middleware for the API skeleton.
func New(cfg config.Config, log *slog.Logger, deps Deps) (*Server, error) {
	mux := http.NewServeMux()
	eventBus := deps.Events
	if eventBus == nil {
		eventBus = events.NewBus(log)
	}

	s := &Server{
		cfg:         cfg,
		log:         log,
		db:          deps.DB,
		queries:     deps.Queries,
		redis:       deps.Redis,
		queueClient: deps.Queue,
		storage:     deps.Storage,
		realtime:    deps.Realtime,
		worker:      deps.Worker,
		events:      eventBus,
	}

	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /v1/app/config", s.handleAppConfig)
	s.mountDocs(mux)

	tokens, err := jwt.NewManager(cfg.JWT.AccessSecret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	if err != nil {
		return nil, fmt.Errorf("httpserver: jwt: %w", err)
	}
	if err := tokens.SetMobileRefreshTTL(cfg.JWT.RefreshTTLMobile); err != nil {
		return nil, fmt.Errorf("httpserver: jwt: %w", err)
	}

	mailer := mail.NewSMTPSender(cfg.SMTP)
	// The exchange visitor copy attaches the owner's vCard at send time (TEC-940); the profiles
	// service is set below once it exists. Used when the queue is off (inline delivery).
	exchangeCopyFiles := &leadsusecase.CopyAttachments{Gone: profilesusecase.ErrNotFound}
	provs := []providers.Provider{
		providers.InappProvider{},
		providers.EmailProvider{Mail: mailer, Attachments: exchangeCopyFiles},
		providers.RealtimeProvider{Pub: deps.Realtime},
		providers.NoopProvider{Name: "sms", Log: log},
		providers.NoopProvider{Name: "push", Log: log},
	}
	notifSvc := notifusecase.New(deps.Queries, deps.Queue, provs, log)
	notifSvc.WithActionSigner(cfg.JWT.AccessSecret, 0)
	notifSvc.WithEncryptionKey(cfg.App.Env, cfg.Encryption.Key)
	notifSvc.WithVAPID(notifusecase.VAPIDConfig{
		PublicKey:  cfg.VAPID.PublicKey,
		PrivateKey: cfg.VAPID.PrivateKey,
		Subject:    cfg.VAPID.Subject,
	})

	repo := authrepo.NewPostgres(deps.DB, deps.Queries)
	uc := authusecase.New(repo, tokens)
	uc.SetNotifier(notifSvc)
	uc.SetLogger(log)

	searchReg := searchengine.NewRegistry(searchadapters.All(deps.Queries)...)
	searchClient := searchengine.NewClient(cfg.Search, log)
	searchIndexer := searchengine.NewIndexer(searchClient, searchReg, deps.Queue, log)
	s.searchClient = searchClient
	s.searchIndexer = searchIndexer
	uc.SetSearchIndexer(searchIndexer)

	rtIssuer, err := realtime.NewTokenIssuer(cfg.Centrifugo)
	if err != nil {
		return nil, fmt.Errorf("httpserver: realtime tokens: %w", err)
	}
	uc.SetRealtimeHints(authusecase.RealtimeHints{
		Enabled: rtIssuer.Enabled(),
		WSURL:   cfg.Centrifugo.WSURL,
	})

	activityRec := activity.NewRecorder(deps.Queries, log)

	stepUpStore := stepup.NewStore(deps.Redis, cfg.App.Env)
	var stepUpWebAuthn *stepup.WebAuthn
	stepUpOrigins := cfg.CORS.AllowedOrigins
	if len(stepUpOrigins) == 0 {
		stepUpOrigins = []string{strings.TrimRight(cfg.Auth.FrontendURL, "/")}
	}
	if webauthn, err := stepup.NewWebAuthn(stepup.WebAuthnConfig{
		RPID:          cfg.Auth.WebAuthnRPID,
		RPDisplayName: cfg.App.Name,
		RPOrigins:     stepUpOrigins,
	}); err == nil {
		stepUpWebAuthn = webauthn
	} else {
		log.Warn("stepup_webauthn_disabled", "error", err)
	}
	secretBox, err := crypto.NewSecretBox(cfg.Encryption.Key)
	if err != nil {
		return nil, fmt.Errorf("httpserver: encryption: %w", err)
	}
	stepUpSvc := stepup.NewService(deps.Queries, stepUpStore, stepUpWebAuthn, repo)
	stepUpSvc.SetSecretBox(secretBox)

	githubSvc := githubusecase.New(deps.Queries, secretBox)
	// Outbound webhooks (TEC-603): endpoints, signed deliveries and the retry queue.
	webhooksSvc := webhooksusecase.New(deps.Queries, secretBox, deps.Queue, log).
		AllowInsecure(cfg.App.Env == "development" || cfg.App.Env == "test")
	oauthProvSvc := oauthproviderusecase.New(deps.Queries, secretBox)
	authSettingsSvc := authsettingsusecase.New(deps.Queries)
	uc.SetAuthSettings(authSettingsSvc)
	uc.SetAccessPolicy(stepUpSvc)
	uc.SetReviewAccounts(cfg.Auth.ReviewAccounts)
	if n := len(cfg.Auth.ReviewAccounts); n > 0 {
		// Count only: the emails and codes are secrets (TEC-1182).
		log.Info("auth_review_accounts_enabled", "count", n)
	}
	uc.SetSecretBox(secretBox, cfg.App.Name)
	entitlementSvc := entitlement.New(deps.Queries)
	entitlementSvc.SetUsageAlerter(notifmodule.NewQuotaAlerter(notifSvc, deps.Queries, log))
	uc.SetEntitlement(authusecase.NewEntitlementAdapter(entitlementSvc))
	// Native Apple / Google id_token sign-in (TEC-1180): audiences come from the admin panel.
	uc.SetNativeOAuth(oidc.New(oidc.DefaultProviders(), nil), oauthProvSvc)
	oauthUC := authusecase.NewOAuth(repo, secretBox)
	// Apple authorization code exchange and token revocation (TEC-1181), signed with the panel .p8.
	appleTokens := authusecase.NewAppleTokens(repo, secretBox, oauthProvSvc, appleauth.NewTokenClient(appleauth.DefaultBaseURL, nil), log)
	uc.SetAppleTokens(appleTokens)
	oauthUC.SetAppleTokens(appleTokens)
	s.githubSvc = githubSvc
	s.oauthProvSvc = oauthProvSvc
	s.authSettings = authSettingsSvc

	h := authhandler.New(uc, oauthUC, githubSvc, oauthProvSvc, authSettingsSvc, cfg.Auth.AdapterSecret, stepUpSvc, activityRec)
	h.SetRateLimiter(ratelimit.New(deps.Redis, cfg.App.Env))
	loader := identity.Loader{UC: uc}
	authmodule.RegisterRoutes(mux, h, tokens, loader, stepUpSvc)
	realtime.RegisterRoutes(mux, realtime.NewHandler(rtIssuer, uc), tokens, loader)

	nh := notifhandler.New(notifSvc)
	notifmodule.RegisterRoutes(mux, nh, tokens, loader)
	notifmodule.RegisterEventHandlers(eventBus, notifSvc, log)
	notifmodule.RegisterBillingHandlers(eventBus, notifSvc, deps.Queries)

	// Resolver cache (TEC-557): routes read it, route/card/profile writes invalidate it.
	routeCache := routecache.New(deps.Redis, cfg.App.Env, routecache.DefaultTTL)
	profilesRepo := profilesrepo.NewPostgres(deps.DB, deps.Queries)
	profilesSvc := profilesusecase.NewService(profilesRepo, deps.Storage)
	exchangeCopyFiles.VCards = profilesSvc
	profilesSvc.SetQuotas(entitlementSvc)
	profilesSvc.SetPublicPages(profilesRepo)
	profilesSvc.SetRouteCache(routeCache)
	profilesSvc.SetActivity(activityRec)
	backgroundsSvc := backgroundsusecase.New(deps.Queries)
	backgroundsSvc.SetFeatures(entitlementSvc)
	if deps.Storage != nil {
		backgroundsSvc.SetThumbnails(deps.Storage)
	}
	s.backgrounds = backgroundsSvc
	profilesSvc.SetBackgrounds(backgroundsSvc)
	profilesSvc.SetFeatures(entitlementSvc)
	// Company media library and branding (TEC-946): employee public pages inherit the company look.
	companyBrandingSvc := companybrandingusecase.New(deps.Queries, deps.Storage)
	companyBrandingSvc.SetFeatures(entitlementSvc)
	profilesSvc.SetAccountThemes(companyBrandingSvc)
	// Verified profile badge (TEC-549).
	profileVerificationsSvc := profileverificationsusecase.New(deps.DB, deps.Queries, notifSvc, log)
	profileVerificationsSvc.SetProfileCache(routeCache)
	profileVerificationsSvc.SetFrontendURL(cfg.Auth.FrontendURL)
	profilesSvc.SetVerifications(profileVerificationsSvc)
	profilesSvc.SetSearchIndexer(searchIndexer)
	// A deleted profile's media leaves object storage too (TEC-1176, KVKK): queued on the
	// maintenance queue, inline when the queue is off.
	var profileMediaCleaner *profilesusecase.MediaCleaner
	if deps.Storage != nil {
		profileMediaCleaner = profilesusecase.NewMediaCleaner(deps.Storage, deps.Queries, deps.Queue, log)
		profilesSvc.SetMediaCleaner(profileMediaCleaner)
	}
	// Employee profiles of company accounts (TEC-573): bulk and import adapters write through
	// the profiles service.
	companyBulk := companyprofilesusecase.NewBulkAdapter(deps.Queries, profilesSvc, deps.Storage, companyBrandingSvc)
	companyBulk.SetFeatures(entitlementSvc)
	// Company email invites (TEC-945); the import invites employees without a user.
	invitesSvc := accountsusecase.NewInvites(deps.DB, deps.Queries, notifSvc, cfg.Auth.FrontendURL, log)
	uc.SetInviteChecker(invitesSvc)
	companyImport := companyprofilesusecase.NewImportAdapter(deps.Queries, profilesSvc)
	companyImport.SetInviter(invitesSvc)

	ioReg := ioengine.NewRegistry(
		ioadapters.NewUsers(deps.Queries),
		ioadapters.NewRoles(deps.Queries),
		ioadapters.NewNotifications(deps.Queries),
		ioadapters.NewActivity(deps.Queries),
		ioadapters.NewOrderManifest(deps.Queries, cfg.Auth.FrontendURL),
		ioadapters.NewLeads(deps.Queries),
		companyImport,
	)
	exportSvc := exportusecase.New(deps.Queries, deps.Storage, ioReg, deps.Queue, notifSvc, activityRec, log)
	importSvc := importusecase.New(deps.Queries, deps.Storage, ioReg, deps.Queue, notifSvc, activityRec, log)
	bulkReg := bulkengine.NewRegistry(
		bulkadapters.NewUsers(deps.Queries),
		bulkadapters.NewRoles(deps.Queries),
		companyBulk,
	)
	bulkSvc := bulkusecase.New(deps.Queries, bulkReg, deps.Queue, notifSvc, activityRec, cfg.Bulk, log)
	logsSvc := logsusecase.New(deps.Queries)
	geoSvc := geoip.NewService(deps.Queries, secretBox, deps.Redis, log)
	cachedRegistry := analyticsregistry.NewCachedStore(deps.Queries, 5*time.Minute)
	eventValidator := analyticsdomain.NewValidator(cachedRegistry)
	uaParser := device.NewParser()
	queuePub := analyticsdelivery.NewAsynqPublisher(deps.Queue)
	eventStore := analyticsstorage.NewPostgresStore(deps.DB, deps.Queries)
	counters := analyticsrealtime.NewCounterManager(deps.Redis)
	aggregator := aggregation.NewAggregator(deps.Queries, log)
	maintMgr := maintenance.NewPartitionManager(deps.DB, deps.Queries, log)
	workerProc := analyticsapp.NewWorkerProcessor(eventStore, counters, aggregator, maintMgr, log)
	analyticsRepo := analyticsinfra.NewPostgresAnalyticsRepository(deps.DB, deps.Queries, counters)
	analyticsSvc := analyticsapp.NewAnalyticsService(analyticsRepo, deps.Queries, deps.Redis, log)
	// The salt rotation period follows the analytics settings (TEC-936).
	privacyHasher := privacy.NewRotatingSaltHasher(cfg.Encryption.Key, privacy.DefaultRotationDays).
		WithSchedule(analyticsSvc.SaltRotationSchedule)
	collector := ingestion.NewCollector(eventValidator, geoSvc, privacyHasher, uaParser, queuePub, eventStore, log)

	if s.worker != nil {
		s.worker.WithExport(exportSvc.ProcessExport).
			WithImport(importSvc.ProcessImport).
			WithBulk(bulkSvc.ProcessBulk).
			WithLogPurge(func(ctx context.Context) error {
				if _, err := webhooksSvc.Prune(ctx, 0); err != nil {
					log.Warn("webhook_prune_failed", "error", err)
				}
				if _, err := webhooksSvc.Sweep(ctx); err != nil {
					log.Warn("webhook_sweep_failed", "error", err)
				}
				return logsSvc.ApplyDueRules(ctx)
			}).
			WithWebhooks(webhooksSvc.Deliver).
			WithAnalytics(workerProc.ProcessEvent, workerProc.ProcessMaintenance).
			WithGeoIP(func(ctx context.Context) error {
				_, err := geoSvc.UpdateNow(ctx)
				return err
			})
		if profileMediaCleaner != nil {
			s.worker.WithProfileMediaCleanup(profileMediaCleaner.Process)
		}
		if searchIndexer != nil {
			s.worker.WithSearch(
				searchIndexer.ProcessUpsert,
				searchIndexer.ProcessDelete,
				searchIndexer.ProcessReindex,
			)
		}
	}
	exportHandler := exporthandler.New(exportSvc)
	exportmodule.RegisterRoutes(mux, exportHandler, tokens, loader, stepUpSvc)
	importmodule.RegisterRoutes(mux, importhandler.New(importSvc), tokens, loader)
	bulkmodule.RegisterRoutes(mux, bulkhandler.New(bulkSvc), tokens, loader)
	companyprofilesmodule.RegisterRoutes(mux, companyprofileshandler.New(
		companyprofilesusecase.NewService(deps.Queries), companyBulk,
	), tokens, loader)
	companybrandingmodule.RegisterRoutes(mux, companybrandinghandler.New(companyBrandingSvc), tokens, loader)
	sysSvc := settingsusecase.NewSystemService(cfg, deps.DB, deps.Redis, deps.Storage, searchClient)
	settingsmodule.RegisterRoutes(mux, settingshandler.New(settingsusecase.New(deps.Queries), deps.Storage, sysSvc), tokens, loader)
	accessmodule.RegisterRoutes(mux, accesshandler.New(stepUpSvc, activityRec), tokens, loader)
	authsettingsmodule.RegisterRoutes(mux, authsettingshandler.New(authSettingsSvc, activityRec), tokens, loader)
	s.errTracking = errortrackingusecase.New(deps.Queries, secretBox, deps.Redis, log, errortrackingusecase.Options{
		FallbackDSN: cfg.Observe.SentryDSN,
		Environment: cfg.Observe.SentryEnvironment,
		Release:     cfg.Observe.SentryRelease,
		Channel:     errortrackingusecase.ChannelName(cfg.App.Env),
	})
	errortrackingmodule.RegisterRoutes(mux, errortrackinghandler.New(s.errTracking), tokens, loader)
	githubmodule.RegisterRoutes(mux, githubhandler.New(githubSvc, activityRec), tokens, loader)
	oauthprovidermodule.RegisterRoutes(mux, oauthproviderhandler.New(oauthProvSvc, activityRec), tokens, loader)
	activitymodule.RegisterRoutes(mux, activityhandler.New(activityusecase.New(deps.Queries)), tokens, loader)
	logsmodule.RegisterRoutes(mux, logshandler.New(logsSvc), tokens, loader)
	searchSvc := searchusecase.New(searchClient, searchReg, log)
	searchmodule.RegisterRoutes(mux, searchhandler.New(searchSvc), tokens, loader)
	storagemodule.RegisterRoutes(
		mux,
		storagehandler.New(storageusecase.New(deps.Storage, deps.Queries, log), cfg.Storage.MaxBytes),
		tokens,
		loader,
	)
	ogClient := opengraph.NewClient()
	plansmodule.RegisterRoutes(mux, planshandler.New(plansusecase.New(deps.DB, deps.Queries)), tokens, loader)
	socialPlatformsRepo := socialplatformsrepo.NewPostgres(deps.DB, deps.Queries)
	socialPlatformsSvc := socialplatformsusecase.New(socialPlatformsRepo, deps.Storage, ogClient)
	socialplatformsmodule.RegisterRoutes(mux, socialplatformshandler.New(socialPlatformsSvc), tokens, loader)
	utilsmodule.RegisterRoutes(mux, utilshandler.New(ogClient), tokens, loader)
	banksRepo := banksrepo.NewPostgres(deps.DB, deps.Queries)
	banksSvc := banksusecase.NewService(banksRepo, deps.Storage)
	banksmodule.RegisterRoutes(mux, bankshandler.New(banksSvc), tokens, loader)
	bankAccountsRepo := bankaccountsrepo.NewPostgres(deps.DB, deps.Queries)
	bankAccountsSvc := bankaccountsusecase.NewService(bankAccountsRepo)
	bankaccountsmodule.RegisterRoutes(mux, bankaccountshandler.New(bankAccountsSvc), tokens, loader)
	profilesHandler := profileshandler.New(profilesSvc)
	profilesmodule.RegisterRoutes(mux, profilesHandler, tokens, loader)
	profileverificationsmodule.RegisterRoutes(mux, profileverificationshandler.New(profileVerificationsSvc), tokens, loader)
	categoriesRepo := categoriesrepo.NewPostgres(deps.Queries)
	categoriesSvc := categoriesusecase.NewService(categoriesRepo)
	categoriesmodule.RegisterRoutes(mux, categorieshandler.New(categoriesSvc), tokens, loader)
	productsRepo := productsrepo.NewPostgres(deps.Queries)
	productsSvc := productsusecase.NewService(productsRepo, categoriesRepo, deps.Storage)
	productsSvc.SetSearchIndexer(searchIndexer)
	productsmodule.RegisterRoutes(mux, productshandler.New(productsSvc), tokens, loader)
	ordersRepo := ordersrepo.NewPostgres(deps.DB, deps.Queries)
	ordersSvc := ordersusecase.NewService(ordersRepo, productsRepo, notifSvc, deps.Storage, cfg.Auth.FrontendURL)
	ordersSvc.SetRouteCache(routeCache)
	ordersSvc.SetSearchIndexer(searchIndexer)
	ordersSvc.SetWebhooks(webhooksSvc)
	ordersHandler := ordershandler.New(ordersSvc)
	ordersHandler.SetActivity(activityRec)
	ordersHandler.SetStepUp(stepUpSvc)
	ordersmodule.RegisterRoutes(mux, ordersHandler, tokens, loader)
	routesRepo := routesrepo.NewPostgres(deps.Queries)
	routesSvc := routesusecase.NewService(routesRepo, cfg.Auth.FrontendURL, collector)
	routesSvc.SetQR(routesRepo, deps.Storage)
	profilesHandler.SetQRTracker(routesSvc)
	routesSvc.SetQuotas(entitlementSvc)
	routesSvc.SetWebhooks(webhooksSvc)
	routesSvc.SetFeatures(entitlementSvc)
	routesSvc.SetCache(routeCache)
	routesSvc.SetActivity(activityRec)
	profilesHandler.SetLegacyResolver(routesSvc)
	profilesSvc.SetLinkPresentations(routesSvc)
	routesHandler := routeshandler.New(routesSvc)
	routesHandler.SetQRPreviewLimiter(ratelimit.New(deps.Redis, cfg.App.Env))
	routesHandler.SetPasscodeLimiter(ratelimit.New(deps.Redis, cfg.App.Env))
	routesmodule.RegisterRoutes(mux, routesHandler, tokens, loader)
	cardsRepo := cardsrepo.NewPostgres(deps.Queries)
	cardsSvc := cardsusecase.NewService(cardsRepo, cfg.Auth.FrontendURL)
	cardsSvc.SetQuotas(entitlementSvc)
	cardsSvc.SetRouteCache(routeCache)
	cardsSvc.SetSearchIndexer(searchIndexer)
	cardsmodule.RegisterRoutes(mux, cardshandler.New(cardsSvc, ratelimit.New(deps.Redis, cfg.App.Env)), tokens, loader)
	// Platform card and route support screens (TEC-950).
	supportSvc := platformsupportusecase.New(platformsupportrepo.NewPostgres(deps.DB, deps.Queries), routeCache, activityRec)
	supportSvc.SetSearchIndexer(searchIndexer)
	platformsupportmodule.RegisterRoutes(mux, platformsupporthandler.New(supportSvc), tokens, loader, stepUpSvc)
	assetsSvc := assetsusecase.New(assetsrepo.New(deps.DB), entitlementSvc, routeCache)
	assetsSvc.SetOrders(func(q *db.Queries) assetsusecase.OrderCreator {
		return ordersusecase.NewService(
			ordersrepo.NewPostgres(nil, q), productsrepo.NewPostgres(q),
			nil, deps.Storage, cfg.Auth.FrontendURL,
		)
	}, ordersSvc.PublishCreatedForAccount)
	assetsmodule.RegisterRoutes(mux, assetshandler.New(assetsSvc), tokens, loader)

	subscriptionsmodule.RegisterRoutes(mux, subscriptionshandler.New(subscriptionsusecase.New(deps.Queries)), tokens, loader)
	backgroundsmodule.RegisterRoutes(mux, backgroundshandler.New(backgroundsSvc), tokens, loader)
	structureRepo := structurerepo.New(deps.DB, deps.Queries)
	structuremodule.RegisterRoutes(mux, structurehandler.New(
		structureusecase.New(structureRepo),
		structureusecase.NewCatalog(structureRepo),
		structureusecase.NewAdmin(structureRepo),
	), tokens, loader)
	accountsSvc := accountsusecase.New(deps.Queries)
	accountsSvc.SetSearchIndexer(searchIndexer)
	accountsSvc.SetInvites(invitesSvc)
	accountsmodule.RegisterRoutes(mux, accountshandler.New(accountsSvc, activityRec, entitlementSvc), tokens, loader)
	invitesHandler := accountshandler.NewInvites(invitesSvc, activityRec)
	invitesHandler.SetRateLimiter(ratelimit.New(deps.Redis, cfg.App.Env))
	accountsmodule.RegisterInviteRoutes(mux, invitesHandler, tokens, loader)
	eventsHandler := eventshandler.New(collector, cachedRegistry)
	eventsHandler.SetRateLimiter(ratelimit.New(deps.Redis, cfg.App.Env))
	eventsmodule.RegisterRoutes(mux, eventsHandler, tokens, loader)
	leadsSvc := leadsusecase.New(leadsrepo.NewPostgres(deps.Queries), notifSvc, log).
		WithVisitorCopy(cfg.JWT.AccessSecret, cfg.Auth.FrontendURL).
		SetQuotas(entitlementSvc).
		SetWebhooks(webhooksSvc)
	exportHandler.SetLeadsGate(leadsSvc)
	webhooksHandler := webhookshandler.New(webhooksSvc)
	webhooksHandler.SetActivity(activityRec)
	webhooksmodule.RegisterRoutes(mux, webhooksHandler, tokens, loader)
	leadsmodule.RegisterRoutes(mux, leadshandler.New(leadsSvc, ratelimit.New(deps.Redis, cfg.App.Env)), tokens, loader)
	analyticsmodule.RegisterRoutes(mux, analyticshandler.New(analyticsSvc, analyticshandler.Access{
		Ownership: analyticshandler.NewSubjectOwnership(deps.Queries),
		History:   entitlementSvc,
	}), tokens, loader)
	companyanalyticsmodule.RegisterRoutes(mux, companyanalyticshandler.New(
		companyanalyticsusecase.New(companyanalyticsrepo.New(deps.DB), entitlementSvc),
	), tokens, loader)
	geoipmodule.RegisterRoutes(mux, geoiphandler.New(geoSvc), tokens, loader)
	overviewSvc := overviewusecase.New(
		overviewrepo.NewPostgres(deps.Queries),
		overviewusecase.NewRedisCache(deps.Redis, cfg.App.Env, overviewusecase.CacheTTL),
	)
	overviewmodule.RegisterRoutes(mux, overviewhandler.New(overviewSvc), tokens, loader)

	paymentReader := payment.NewDBSettingsReader(deps.Queries, secretBox)
	paymentRegistry := payment.NewRegistry(paymentReader)
	paymentintegrationmodule.RegisterRoutes(
		mux,
		paymentintegrationhandler.New(paymentintegrationusecase.New(deps.Queries, secretBox), activityRec),
		tokens,
		loader,
	)
	apiBaseURL := strings.TrimRight(cfg.Auth.FrontendURL, "/") + "/api"
	billingSvc := billingusecase.New(deps.Queries, paymentRegistry, cfg.Auth.FrontendURL, apiBaseURL, log, billingrepository.NewPostgres(deps.DB))
	billingSvc.SetOrderPayments(ordersSvc)
	billingSvc.SetActivity(activityRec)
	billingSvc.SetStorage(deps.Storage)
	// Moving an online-paid order to refunded refunds it at the provider first (TEC-935).
	ordersSvc.SetPaymentRefunder(billingSvc)
	billingHandler := billinghandler.New(
		billingSvc,
		cfg.Auth.FrontendURL,
		log,
	)
	billingmodule.RegisterRoutes(mux, billingHandler, tokens, loader)
	billingmodule.RegisterRefundRoutes(mux, billingHandler, tokens, loader, stepUpSvc)

	outboxStore := outbox.NewStore(deps.DB, deps.Queries)
	outboxPub := outbox.NewPublisher(outboxStore, eventBus, log)
	s.outboxPub = outboxPub

	trustedProxies, err := clientip.ParseTrusted(cfg.HTTP.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("httpserver: TRUSTED_PROXIES: %w", err)
	}

	s.http = &http.Server{
		Addr: cfg.HTTP.Addr,
		// RequestID is outermost so 5xx logs, error reports, and panics carry the id.
		Handler: middleware.Chain(middleware.CaptureRoute(mux),
			middleware.RequestID,
			middleware.ServerErrors(log),
			middleware.Recover,
			clientip.New(trustedProxies).Middleware,
			// Outdated mobile apps (X-Client + X-App-Version below mobile_min_version) get 426.
			appversion.Gate(appversion.Cached(settingsusecase.New(deps.Queries).MinMobileVersion, 30*time.Second, nil)),
		),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}
	return s, nil
}

// Start listens until the server stops. Optionally starts an in-process queue worker.
func (s *Server) Start() error {
	if s.errTracking != nil && s.queries != nil {
		ctx, cancel := context.WithCancel(context.Background())
		s.errTrackStop = cancel
		s.errTracking.Start(ctx)
	}
	if s.outboxPub != nil {
		s.outboxStop = s.outboxPub.StartRun(context.Background())
	}
	if s.worker != nil {
		go func() {
			if err := s.worker.Start(); err != nil {
				s.log.Error("queue_worker_failed", "error", err)
			}
		}()
	}
	if s.searchIndexer != nil {
		go s.searchIndexer.Bootstrap(context.Background())
	}
	if s.backgrounds != nil {
		go s.backgrounds.BackfillThumbnails(context.Background())
	}
	s.log.Info("http_listen", "addr", s.cfg.HTTP.Addr, "env", s.cfg.App.Env)
	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("httpserver: listen: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the HTTP server, queue worker, and queue client.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.errTrackStop != nil {
		s.errTrackStop()
	}
	if s.outboxStop != nil {
		s.outboxStop()
	}
	if s.worker != nil {
		s.worker.Shutdown()
	}
	if s.queueClient != nil {
		_ = s.queueClient.Close()
	}
	return s.http.Shutdown(ctx)
}

func (s *Server) handleAppConfig(w http.ResponseWriter, r *http.Request) {
	settingsSvc := settingsusecase.New(s.queries)
	letterhead, _ := settingsSvc.PublicLetterhead(r.Context())
	// Mobile app version gate and store links (TEC-1178); empty values when unset or unreadable.
	mobile, _ := settingsSvc.Mobile(r.Context())

	registrationEnabled := false
	passwordLogin := true
	passwordRegister := false
	passkeyLogin := true
	if s.authSettings != nil {
		if p, err := s.authSettings.Policy(r.Context()); err == nil {
			registrationEnabled = p.RegistrationEnabled
			passwordLogin = p.PasswordLoginEnabled
			passwordRegister = p.RegistrationEnabled && p.PasswordRegisterEnabled
			passkeyLogin = p.PasskeyLoginEnabled
		}
	}

	githubLogin, githubRegister := false, false
	if s.githubSvc != nil {
		githubLogin, _ = s.githubSvc.IsAuthEnabled(r.Context())
		reg, _ := s.githubSvc.IsRegisterEnabled(r.Context())
		githubRegister = registrationEnabled && reg
	}

	method := func(login, register bool) map[string]bool {
		return map[string]bool{"login": login, "register": register}
	}
	oauthMethod := func(provider string) map[string]bool {
		login, register := false, false
		if s.oauthProvSvc != nil {
			login, _ = s.oauthProvSvc.IsLoginEnabled(r.Context(), provider)
			reg, _ := s.oauthProvSvc.IsRegisterEnabled(r.Context(), provider)
			register = registrationEnabled && reg
		}
		return method(login, register)
	}

	responseJSON := map[string]any{
		"mode":                 "platform",
		"name":                 s.cfg.App.Name,
		"vapid_configured":     s.cfg.VAPID.PublicKey != "" && s.cfg.VAPID.PrivateKey != "",
		"github_auth_enabled":  githubLogin, // legacy
		"registration_enabled": registrationEnabled,
		"auth_methods": map[string]any{
			"password": method(passwordLogin, passwordRegister),
			"passkey":  map[string]bool{"login": passkeyLogin},
			"github":   method(githubLogin, githubRegister),
			"google":   oauthMethod("google"),
			"facebook": oauthMethod("facebook"),
			"apple":    oauthMethod("apple"),
		},
		"letterhead": letterhead,
		"mobile":     mobile,
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    responseJSON,
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := map[string]string{
		"postgres": "ok",
		"redis":    "ok",
	}
	ready := true

	if err := database.Ping(ctx, s.db); err != nil {
		checks["postgres"] = "down"
		ready = false
	}
	if err := cache.Ping(ctx, s.redis); err != nil {
		checks["redis"] = "down"
		ready = false
	}

	if s.storage != nil {
		if err := s.storage.Ping(ctx); err != nil {
			s.log.Warn("readyz_storage_soft_check", "error", err)
		}
	}
	if s.searchClient != nil && s.searchClient.Enabled() {
		if err := s.searchClient.Ping(ctx); err != nil {
			checks["meilisearch"] = "down"
			s.log.Warn("readyz_meilisearch_soft_check", "error", err)
		} else {
			checks["meilisearch"] = "ok"
		}
	}

	status := http.StatusOK
	state := "ready"
	if !ready {
		status = http.StatusServiceUnavailable
		state = "not_ready"
	}

	writeJSON(w, status, map[string]any{
		"status": state,
		"checks": checks,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
