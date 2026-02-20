package router

import (
	"net/http"

	"github.com/redis/go-redis/v9"
	authsvc "troo-backend/internal/application/auth"
	emailsvc "troo-backend/internal/application/emails"
	holdsvc "troo-backend/internal/application/holdings"
	invsvc "troo-backend/internal/application/invitations"
	lesvc "troo-backend/internal/application/listingevents"
	listsvc "troo-backend/internal/application/listings"
	mktsvc "troo-backend/internal/application/marketplace"
	orgsvc "troo-backend/internal/application/org"
	retsvc "troo-backend/internal/application/retirements"
	tradesvc "troo-backend/internal/application/trading"
	txsvc "troo-backend/internal/application/transactions"
	uploadsvc "troo-backend/internal/application/uploads"
	usersvc "troo-backend/internal/application/user"
	"troo-backend/internal/config"
	"troo-backend/internal/infrastructure/database"
	authhandler "troo-backend/internal/interfaces/handlers/auth"
	healthhandler "troo-backend/internal/interfaces/handlers/health"
	holdhandler "troo-backend/internal/interfaces/handlers/holdings"
	invhandler "troo-backend/internal/interfaces/handlers/invitations"
	lehandler "troo-backend/internal/interfaces/handlers/listingevents"
	listhandler "troo-backend/internal/interfaces/handlers/listings"
	mkthandler "troo-backend/internal/interfaces/handlers/marketplace"
	orghandler "troo-backend/internal/interfaces/handlers/org"
	payhandler "troo-backend/internal/interfaces/handlers/payments"
	rethandler "troo-backend/internal/interfaces/handlers/retirements"
	tradehandler "troo-backend/internal/interfaces/handlers/trading"
	txhandler "troo-backend/internal/interfaces/handlers/transactions"
	uploadhandler "troo-backend/internal/interfaces/handlers/uploads"
	userhandler "troo-backend/internal/interfaces/handlers/user"
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/constants"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"gorm.io/gorm"
)

type gormDBPinger struct {
	db *gorm.DB
}

func (g *gormDBPinger) Ping() error {
	if g == nil || g.db == nil {
		return nil
	}
	sqlDB, err := g.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func CreateApp(cfg *config.Config) (*fiber.App, *gorm.DB, *redis.Client, error) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage:   true,
		ErrorHandler:            middleware.ErrorHandler,
		EnableTrustedProxyCheck: true,
	})

	app.Use(middleware.CORS(middleware.CORSConfig{
		AllowedSuffix: cfg.FrontendURLEndsWith,
		DevPassword:   cfg.DevPassword,
	}))

	stripeWebhook := &payhandler.WebhookHandler{WebhookSecret: cfg.StripeWebhookSecret}
	app.Post("/api/v1/stripe/webhook", func(c *fiber.Ctx) error {
		return stripeWebhook.HandleWebhook(c)
	})

	sessionHandler, redisClient, err := middleware.Session(middleware.SessionConfig{
		Secret:            cfg.SessionSecret,
		RedisURL:          cfg.RedisURL,
		AllowCrossSiteDev: cfg.AllowCrossSiteDev,
		IsProduction:      cfg.Env == "production",
	})
	if err != nil {
		return nil, nil, nil, err
	}
	rdb := redisClient
	app.Use(sessionHandler)
	app.Use(middleware.HealthMarker(rdb))
	app.Use(middleware.ResponseFormatter())
	app.Use(middleware.Tracing())
	app.Use(middleware.RouteLogger())

	app.Use(func(c *fiber.Ctx) error {
		user := c.Locals("user")
		if user == nil {
			c.Locals("user", nil)
		}
		return c.Next()
	})

	hh := &healthhandler.Handlers{
		Rdb:            rdb,
		DB:             nil,
		HealthAdminKey: cfg.HealthAdminKey,
	}
	app.Get("/", hh.Dashboard)
	app.Get("/reset", hh.Reset)
	app.Get("/health/json", hh.JSON)
	app.Get("/health/errors", hh.Errors)

	var db *gorm.DB
	if cfg.DatabaseURL != "" {
		var errDB error
		db, errDB = database.Open(cfg.DatabaseURL)
		if errDB != nil {
			return nil, nil, nil, errDB
		}
		hh.DB = &gormDBPinger{db: db}
	}

	sessionCfg := middleware.SessionConfig{
		Secret:            cfg.SessionSecret,
		RedisURL:          cfg.RedisURL,
		AllowCrossSiteDev: cfg.AllowCrossSiteDev,
		IsProduction:      cfg.Env == "production",
	}

	var userFinder authsvc.UserFinder
	if db != nil {
		userFinder = &authsvc.GormUserFinder{DB: db}
	}
	ah := &authhandler.Handlers{
		UserFinder: userFinder,
		Rdb:        rdb,
		Config:     sessionCfg,
	}
	authGroup := app.Group("/api/v1/auth")
	authGroup.Post("/login", ah.Login)
	authGroup.Get("/me", ah.Me)
	authGroup.Delete("/logout", ah.Logout)

	if db != nil {
		stripeWebhook.DB = db
	}

	if db != nil && rdb != nil {
		// User (with optional welcome email via Brevo, same env as Express: SENDINBLUE_API_KEY, MAIL_FROM)
		var emailSender emailsvc.Sender
		if cfg.SendinblueAPIKey != "" {
			emailSender = &emailsvc.BrevoClient{APIKey: cfg.SendinblueAPIKey, MailFrom: cfg.MailFrom}
		}
		us := &usersvc.Service{DB: db, Rdb: rdb, EmailSender: emailSender}
		uh := &userhandler.Handlers{Service: us, Config: sessionCfg}
		// create-user is public (registration); same as Express
		app.Post("/api/v1/users/create-user", uh.CreateUser)
		ug := app.Group("/api/v1/users", middleware.RequireAuth())
		ug.Put("/update-user", uh.UpdateUser)
		ug.Get("/view-user", uh.ViewUser)
		ug.Patch("/update-role", middleware.AuthorizePermission(constants.AssignRole), uh.UpdateRole)
		ug.Delete("/remove-user", middleware.AuthorizePermission(constants.RemoveUser), uh.RemoveUser)

		// Org
		os := &orgsvc.Service{DB: db}
		oh := &orghandler.Handlers{Service: os, Config: sessionCfg}
		og := app.Group("/api/v1/orgs", middleware.RequireAuth())
		og.Post("/create-org", oh.CreateOrg)
		og.Get("/view-org", oh.ViewOrg)
		og.Patch("/update-org", oh.UpdateOrg)

		// Uploads — sign URL uses SUPABASE_URL (e.g. https://xwsiuytkbefejvoqpjyg.supabase.co/storage/v1/...)
		sc := &uploadsvc.HTTPClient{BaseURL: cfg.SupabaseURL, SecretKey: cfg.SupabaseSecretKey}
		upsvc := &uploadsvc.Service{Client: sc, SupabaseURL: cfg.SupabaseURL}
		uph := &uploadhandler.Handlers{Service: upsvc}
		upg := app.Group("/api/v1/uploads", middleware.RequireAuth())
		upg.Post("/org-logo", uph.UploadOrgLogo)
		upg.Post("/org-doc", uph.UploadOrgDoc)

		// Holdings
		hs := &holdsvc.Service{DB: db}
		holdh := &holdhandler.Handlers{Service: hs}
		hg := app.Group("/api/v1/holdings", middleware.RequireAuth())
		hg.Get("/view-holdings", holdh.ViewHoldings)
		hg.Post("/view-project", holdh.ViewProject)

		// Marketplace
		ms := &mktsvc.Service{DB: db, ICR: nil}
		mh := &mkthandler.Handlers{Service: ms}
		mg := app.Group("/api/v1/marketplace", middleware.RequireAuth())
		mg.Get("/projects", mh.GetAllProjects)
		mg.Get("/projects/:id", mh.GetProjectByID)
		mg.Post("/admin-sync", mh.AdminSync)

		// Listings
		ls := &listsvc.Service{DB: db}
		lh := &listhandler.Handlers{Service: ls}
		lg := app.Group("/api/v1/listings", middleware.RequireAuth())
		lg.Post("/create-listing", lh.CreateListing)
		lg.Get("/get-all-listings", lh.GetAllListings)
		lg.Get("/get-org-listings", lh.GetOrgListings)
		lg.Get("/get-listing/:listing_id", lh.GetListingByID)
		lg.Get("/get-all-active-listings", lh.GetAllActiveListings)
		lg.Get("/get-all-closed-listings", lh.GetAllClosedListings)
		lg.Get("/get-org-active-listings", lh.GetOrgActiveListings)
		lg.Get("/get-org-closed-listings", lh.GetOrgClosedListings)
		lg.Put("/edit-listing", lh.EditListing)
		lg.Post("/cancel-listing", lh.CancelListing)

		// Invitations
		is := &invsvc.Service{DB: db, EmailSender: emailSender, InviteBaseURL: cfg.InviteBaseURL}
		ih := &invhandler.Handlers{Service: is, Config: sessionCfg}
		app.Post("/api/v1/invitations/public/check-token", ih.CheckToken)
		ig := app.Group("/api/v1/invitations", middleware.RequireAuth())
		ig.Post("/create-invite", middleware.AuthorizePermission(constants.InviteUser), ih.SendInvite)
		ig.Post("/accept-invite", ih.AcceptInvite)
		ig.Patch("/revoke-invite", middleware.AuthorizePermission(constants.InviteUser), ih.RevokeInvite)
		ig.Get("/view-invites", middleware.AuthorizePermission(constants.ViewData), ih.ListOrgInvitations)
		ig.Post("/resend-invite", middleware.AuthorizePermission(constants.InviteUser), ih.ResendInvite)

		// Trading
		ts := &tradesvc.Service{DB: db}
		th := &tradehandler.Handlers{
			Service:       ts,
			StripeCreator: &tradehandler.RealStripeCreator{SecretKey: cfg.StripeSecretKey},
		}
		tg := app.Group("/api/v1/trading", middleware.RequireAuth())
		tg.Post("/buy-credits", middleware.AuthorizePermission(constants.BuyCredits), th.BuyCredits)
		tg.Post("/sell-credits", middleware.AuthorizePermission(constants.SellCredits), th.SellCredits)
		tg.Post("/retire-credits", middleware.AuthorizePermission(constants.RetireCredits), th.RetireCredits)
		tg.Post("/transfer-credits", middleware.AuthorizePermission(constants.TransferCredits), th.TransferCredits)

		// Retirements
		rs := &retsvc.Service{DB: db}
		rh := &rethandler.Handlers{Service: rs}
		rg := app.Group("/api/v1/retirements", middleware.RequireAuth())
		rg.Get("/view-org", rh.ViewOrg)
		rg.Post("/view-one", rh.ViewOne)

		// Transactions
		txs := &txsvc.Service{DB: db}
		txh := &txhandler.Handlers{Service: txs}
		txg := app.Group("/api/v1/transactions", middleware.RequireAuth())
		txg.Get("/get-transactions", txh.GetTransactions)

		// ListingEvents
		les := &lesvc.Service{DB: db}
		leh := &lehandler.Handlers{Service: les}
		leg := app.Group("/api/v1/listing-events", middleware.RequireAuth())
		leg.Get("/get-org-listing-events", leh.GetOrgListingEvents)
	}

	return app, db, rdb, nil
}

func Handler(app *fiber.App) http.Handler {
	return adaptor.FiberApp(app)
}
