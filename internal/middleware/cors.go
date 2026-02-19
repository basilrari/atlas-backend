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

		allowed := false
		if c.Method() == fiber.MethodOptions && (strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:")) {
			allowed = true
		}
		if !allowed && cfg.AllowedSuffix != "" && strings.HasSuffix(strings.ToLower(origin), strings.ToLower(cfg.AllowedSuffix)) {
			allowed = true
		}
		if !allowed && cfg.DevPassword != "" && c.Get("dev-password") == cfg.DevPassword {
			allowed = true
		}

		if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status": "error",
				"error": fiber.Map{
					"message":    "Not allowed by CORS",
					"statusCode": 403,
					"details":    fiber.Map{},
				},
			})
		}

		setCORSHeaders(c, origin)

		// Preflight (OPTIONS) must get 2xx and no body so the browser allows the actual request.
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	}
}

func setCORSHeaders(c *fiber.Ctx, origin string) {
	c.Set("Access-Control-Allow-Origin", origin)
	c.Set("Access-Control-Allow-Credentials", "true")
	c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, X-Requested-With, dev-password")
	c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	c.Set("Access-Control-Max-Age", "86400")
}
