package bootstrap

import (
	"troo-backend/internal/config"
	"troo-backend/internal/interfaces/router"

	"github.com/gofiber/fiber/v2"
)

// New creates the Fiber app for Vercel serverless (api handler imports this package, not internal).
func New() (*fiber.App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	app, _, _, err := router.CreateApp(cfg)
	return app, err
}
