package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CORSConfig holds CORS configuration (suffix + dev password).
type CORSConfig struct {
	AllowedSuffix string
	DevPassword   string
}

// CORS returns a Fiber handler that allows origins ending with AllowedSuffix
// or requests with the correct dev-password header. Credentials allowed.
func CORS(cfg CORSConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		// No origin (e.g. same-origin or tools): allow
		if origin == "" {
			return c.Next()
		}
		// Preflight from localhost in dev
		if c.Method() == fiber.MethodOptions && (strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:")) {
			setCORSHeaders(c, origin)
			return c.SendStatus(fiber.StatusNoContent)
		}
		// Suffix match (e.g. .troo.earth)
		if cfg.AllowedSuffix != "" && strings.HasSuffix(strings.ToLower(origin), strings.ToLower(cfg.AllowedSuffix)) {
			setCORSHeaders(c, origin)
			return c.Next()
		}
		// Dev password header
		if cfg.DevPassword != "" && c.Get("dev-password") == cfg.DevPassword {
			setCORSHeaders(c, origin)
			return c.Next()
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status": "error",
			"error": fiber.Map{
				"message":    "Not allowed by CORS",
				"statusCode": 403,
				"details":    fiber.Map{},
			},
		})
	}
}

func setCORSHeaders(c *fiber.Ctx, origin string) {
	c.Set("Access-Control-Allow-Origin", origin)
	c.Set("Access-Control-Allow-Credentials", "true")
	c.Set("Access-Control-Allow-Headers", "Content-Type, dev-password")
}
