package middleware

import (
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// ErrorHandler is the global error handler. Returns the standard error format.
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"
	details := map[string]interface{}{}

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}
	// Optional: expose stack in non-production via details
	// details["stack"] = ...

	return response.Error(c, message, code, details)
}
