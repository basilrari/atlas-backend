package middleware

import (
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

const userLocal = "user"

// RequireAuth ensures a user is in the session. Returns 401 with standard error format if not.
func RequireAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := c.Locals(userLocal)
		if user == nil {
			return response.Unauthorized(c, "Unauthorized")
		}
		// Attach auth context for handlers (same key)
		c.Locals("auth", user)
		return c.Next()
	}
}

// GetUser returns the session user from Locals (nil if not logged in).
func GetUser(c *fiber.Ctx) interface{} {
	return c.Locals(userLocal)
}
