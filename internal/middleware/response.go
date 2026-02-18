package middleware

import (
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// ResponseFormatter injects standardized success/error helpers into Locals
// so handlers can use them. Actual helpers are in internal/pkg/response.
func ResponseFormatter() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Expose response helpers via Locals for handlers that want to call them by name.
		// Handlers should typically import response pkg and call response.Success(c, ...) etc.
		c.Locals("response_success", func(msg string, data interface{}, meta interface{}) error {
			return response.Success(c, msg, data, meta)
		})
		c.Locals("response_success_created", func(msg string, data interface{}, meta interface{}) error {
			return response.SuccessCreated(c, msg, data, meta)
		})
		c.Locals("response_error", func(msg string, code int, details interface{}) error {
			return response.Error(c, msg, code, details)
		})
		return c.Next()
	}
}
