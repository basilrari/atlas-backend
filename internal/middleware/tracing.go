package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const traceIDHeader = "X-Trace-Id"
const traceIDLocal = "trace_id"

// Tracing adds a trace ID to the request and response.
func Tracing() fiber.Handler {
	return func(c *fiber.Ctx) error {
		traceID := uuid.New().String()
		c.Locals(traceIDLocal, traceID)
		c.Set(traceIDHeader, traceID)
		return c.Next()
	}
}

// GetTraceID returns the trace ID from context.
func GetTraceID(c *fiber.Ctx) string {
	if id, ok := c.Locals(traceIDLocal).(string); ok {
		return id
	}
	return ""
}
