package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// RouteLogger logs each request entry and exit with duration and trace ID.
func RouteLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		traceID := GetTraceID(c)
		if traceID == "" {
			traceID = "no-trace-id"
		}
		start := time.Now()
		log.Info().Str("trace_id", traceID).Str("method", c.Method()).Str("path", c.Path()).Msg("Entering request")
		err := c.Next()
		ms := time.Since(start).Milliseconds()
		log.Info().Str("trace_id", traceID).Str("method", c.Method()).Str("path", c.Path()).Int64("ms", ms).Msg("Exiting request")
		return err
	}
}
