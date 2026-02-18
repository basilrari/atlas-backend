package app

import (
	"net/http"

	"troo-backend/internal/auth"
	"troo-backend/internal/config"
	"troo-backend/internal/constants"
	"troo-backend/internal/database"
	"troo-backend/internal/health"
	"troo-backend/internal/middleware"
	"troo-backend/internal/holdings"
	"troo-backend/internal/invitations"
	"troo-backend/internal/listings"
	"troo-backend/internal/listingevents"
	"troo-backend/internal/marketplace"
	"troo-backend/internal/org"
	"troo-backend/internal/payments"
	"troo-backend/internal/retirements"
	"troo-backend/internal/trading"
	"troo-backend/internal/transactions"
	"troo-backend/internal/uploads"
	"troo-backend/internal/user"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"gorm.io/gorm"
)

// CreateApp builds the Fiber app with all global middleware and route registration.
// Vercel will invoke the returned handler via adaptor.
func CreateApp(cfg *config.Config) (*fiber.App, error) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage:   true,
		ErrorHandler:           middleware.ErrorHandler,
		EnableTrustedProxyCheck: true,
	})

	// CORS (before session)
	app.Use(middleware.CORS(middleware.CORSConfig{
		AllowedSuffix: cfg.FrontendURLEndsWith,
		DevPassword:   cfg.DevPassword,
	}))

	// Stripe webhook placeholder — mounted early (before session/JSON parser, Express parity).
	// DB is set after database init below; handler reads raw body + stripe-signature header.
	stripeWebhook := &payments.WebhookHandler{WebhookSecret: cfg.StripeWebhookSecret}
	app.Post("/api/v1/stripe/webhook", func(c *fiber.Ctx) error {
		return stripeWebhook.HandleWebhook(c)
	})

	// Session (Redis); need Redis client for health marker too
	sessionHandler, rdb, err := middleware.Session(middleware.SessionConfig{
		Secret:            cfg.SessionSecret,
		RedisURL:          cfg.RedisURL,
		AllowCrossSiteDev: cfg.AllowCrossSiteDev,
		IsProduction:      cfg.Env == "production",
	})
	if err != nil {
		return nil, err
	}
	app.Use(sessionHandler)

	// Health request marker (after session)
	app.Use(middleware.HealthMarker(rdb))

	// Response formatter (inject helpers)
	app.Use(middleware.ResponseFormatter())

	// Tracing + route logger
	app.Use(middleware.Tracing())
	app.Use(middleware.RouteLogger())

	// Session user in Locals (Express: res.locals.user = req.session.user)
	app.Use(func(c *fiber.Ctx) error {
		user := c.Locals("user")
		if user == nil {
			c.Locals("user", nil)
		}
		return c.Next()
	})

	// --- Routes (no auth) ---
	// Health module (GET /, GET /reset, GET /health/json, GET /health/errors)
	healthHandlers := &health.Handlers{
		Rdb:            rdb,
		DB:             nil, // optional; wire when DB is available
		HealthAdminKey: cfg.HealthAdminKey,
	}
	app.Get("/", healthHandlers.Dashboard)
	app.Get("/reset", healthHandlers.Reset)
	app.Get("/health/json", healthHandlers.JSON)
	app.Get("/health/errors", healthHandlers.Errors)

	// Auth (no auth middleware): POST login, GET me, DELETE logout
	var db *gorm.DB
	if cfg.DatabaseURL != "" {
		var errDB error
		db, errDB = database.Open(cfg.DatabaseURL)
		if errDB != nil {
			return nil, errDB
		}
	}
	// db may be nil if DATABASE_URL not set (e.g. tests); Login will return 500
	sessionCfg := middleware.SessionConfig{
		Secret:            cfg.SessionSecret,
		RedisURL:          cfg.RedisURL,
		AllowCrossSiteDev: cfg.AllowCrossSiteDev,
		IsProduction:      cfg.Env == "production",
	}
	var userFinder auth.UserFinder
	if db != nil {
		userFinder = &auth.GormUserFinder{DB: db}
	}
	authHandlers := &auth.Handlers{
		UserFinder: userFinder,
		Rdb:        rdb,
		Config:     sessionCfg,
	}
	authGroup := app.Group("/api/v1/auth")
	authGroup.Post("/login", authHandlers.Login)
	authGroup.Get("/me", authHandlers.Me)
	authGroup.Delete("/logout", authHandlers.Logout)

	// Wire DB into Stripe webhook handler (initialized above, before session)
	if db != nil {
		stripeWebhook.DB = db
	}

	// --- Protected modules (auth required) ---
	if db != nil && rdb != nil {
		// User module
		userService := &user.Service{DB: db, Rdb: rdb}
		userHandlers := &user.Handlers{Service: userService, Config: sessionCfg}
		userGroup := app.Group("/api/v1/users", middleware.RequireAuth())
		userGroup.Post("/create-user", userHandlers.CreateUser)
		userGroup.Put("/update-user/:id", userHandlers.UpdateUser)
		userGroup.Get("/view-user/:id", userHandlers.ViewUser)
		userGroup.Patch("/update-role", middleware.AuthorizePermission(constants.AssignRole), userHandlers.UpdateRole)
		userGroup.Delete("/remove-user", middleware.AuthorizePermission(constants.RemoveUser), userHandlers.RemoveUser)

		// Org module
		orgService := &org.Service{DB: db}
		orgHandlers := &org.Handlers{Service: orgService, Config: sessionCfg}
		orgGroup := app.Group("/api/v1/orgs", middleware.RequireAuth())
		orgGroup.Post("/create-org", orgHandlers.CreateOrg)
		orgGroup.Get("/view-org", orgHandlers.ViewOrg)
		orgGroup.Patch("/update-org/:id", orgHandlers.UpdateOrg)

		// Uploads module
		supabaseClient := &uploads.HTTPClient{
			BaseURL:   cfg.SupabaseURL,
			SecretKey: cfg.SupabaseSecretKey,
		}
		uploadService := &uploads.Service{
			Client:      supabaseClient,
			SupabaseURL: cfg.SupabaseURL,
		}
		uploadHandlers := &uploads.Handlers{Service: uploadService}
		uploadGroup := app.Group("/api/v1/uploads", middleware.RequireAuth())
		uploadGroup.Post("/org-logo", uploadHandlers.UploadOrgLogo)
		uploadGroup.Post("/org-doc", uploadHandlers.UploadOrgDoc)

		// Holdings module
		holdingsService := &holdings.Service{DB: db}
		holdingsHandlers := &holdings.Handlers{Service: holdingsService}
		holdingsGroup := app.Group("/api/v1/holdings", middleware.RequireAuth())
		holdingsGroup.Get("/view-holdings", holdingsHandlers.ViewHoldings)
		holdingsGroup.Post("/view-project", holdingsHandlers.ViewProject)

		// Marketplace module (special response shape)
		marketplaceService := &marketplace.Service{DB: db, ICR: nil}
		marketplaceHandlers := &marketplace.Handlers{Service: marketplaceService}
		marketplaceGroup := app.Group("/api/v1/marketplace", middleware.RequireAuth())
		marketplaceGroup.Get("/projects", marketplaceHandlers.GetAllProjects)
		marketplaceGroup.Get("/projects/:id", marketplaceHandlers.GetProjectByID)
		marketplaceGroup.Post("/admin-sync", marketplaceHandlers.AdminSync)

		// Listings module (all 10 routes, auth required)
		listingsService := &listings.Service{DB: db}
		listingsHandlers := &listings.Handlers{Service: listingsService}
		listingsGroup := app.Group("/api/v1/listings", middleware.RequireAuth())
		listingsGroup.Post("/create-listing", listingsHandlers.CreateListing)
		listingsGroup.Get("/get-all-listings", listingsHandlers.GetAllListings)
		listingsGroup.Get("/get-org-listings", listingsHandlers.GetOrgListings)
		listingsGroup.Get("/get-listing/:listing_id", listingsHandlers.GetListingByID)
		listingsGroup.Get("/get-all-active-listings", listingsHandlers.GetAllActiveListings)
		listingsGroup.Get("/get-all-closed-listings", listingsHandlers.GetAllClosedListings)
		listingsGroup.Get("/get-org-active-listings", listingsHandlers.GetOrgActiveListings)
		listingsGroup.Get("/get-org-closed-listings", listingsHandlers.GetOrgClosedListings)
		listingsGroup.Put("/edit-listing", listingsHandlers.EditListing)
		listingsGroup.Post("/cancel-listing", listingsHandlers.CancelListing)

		// Invitations module: public route (no auth) + private routes (auth + permissions)
		invService := &invitations.Service{DB: db}
		invHandlers := &invitations.Handlers{Service: invService, Config: sessionCfg}
		app.Post("/api/v1/invitations/public/check-token", invHandlers.CheckToken)
		invGroup := app.Group("/api/v1/invitations", middleware.RequireAuth())
		invGroup.Post("/create-invite", middleware.AuthorizePermission(constants.InviteUser), invHandlers.SendInvite)
		invGroup.Post("/accept-invite", invHandlers.AcceptInvite)
		invGroup.Patch("/revoke-invite", middleware.AuthorizePermission(constants.InviteUser), invHandlers.RevokeInvite)
		invGroup.Get("/view-invites", middleware.AuthorizePermission(constants.ViewData), invHandlers.ListOrgInvitations)
		invGroup.Post("/resend-invite", middleware.AuthorizePermission(constants.InviteUser), invHandlers.ResendInvite)

		// Trading module (4 routes, each with specific permission)
		tradingService := &trading.Service{DB: db}
		tradingHandlers := &trading.Handlers{
			Service:       tradingService,
			StripeCreator: &trading.RealStripeCreator{SecretKey: cfg.StripeSecretKey},
		}
		tradingGroup := app.Group("/api/v1/trading", middleware.RequireAuth())
		tradingGroup.Post("/buy-credits", middleware.AuthorizePermission(constants.BuyCredits), tradingHandlers.BuyCredits)
		tradingGroup.Post("/sell-credits", middleware.AuthorizePermission(constants.SellCredits), tradingHandlers.SellCredits)
		tradingGroup.Post("/retire-credits", middleware.AuthorizePermission(constants.RetireCredits), tradingHandlers.RetireCredits)
		tradingGroup.Post("/transfer-credits", middleware.AuthorizePermission(constants.TransferCredits), tradingHandlers.TransferCredits)

		// Retirements module (2 routes, auth required)
		retirementService := &retirements.Service{DB: db}
		retirementHandlers := &retirements.Handlers{Service: retirementService}
		retirementGroup := app.Group("/api/v1/retirements", middleware.RequireAuth())
		retirementGroup.Get("/view-org", retirementHandlers.ViewOrg)
		retirementGroup.Post("/view-one", retirementHandlers.ViewOne)

		// Transactions module (1 route, auth required)
		txService := &transactions.Service{DB: db}
		txHandlers := &transactions.Handlers{Service: txService}
		txGroup := app.Group("/api/v1/transactions", middleware.RequireAuth())
		txGroup.Get("/get-transactions", txHandlers.GetTransactions)

		// ListingEvents module (1 route, auth required)
		leService := &listingevents.Service{DB: db}
		leHandlers := &listingevents.Handlers{Service: leService}
		leGroup := app.Group("/api/v1/listing-events", middleware.RequireAuth())
		leGroup.Get("/get-org-listing-events", leHandlers.GetOrgListingEvents)
	}

	return app, nil
}

// Handler returns an http.Handler for Vercel (Fiber app as net/http handler).
func Handler(app *fiber.App) http.Handler {
	return adaptor.FiberApp(app)
}
