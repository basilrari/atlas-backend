package middleware

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// Redis keys matching Express health utils (must match for shared dashboard).
// Exported for use by health handlers (reset, collectHealth).
const (
	KeyReqTotal  = "health:global:req_total"
	KeyReqErrors = "health:global:req_errors"
	KeyResTime   = "health:global:res_time_total"
	KeyResCount  = "health:global:res_count"
	KeyStartTime = "health:global:start_time"
	KeyLastReq   = "health:global:last_request"
	KeyErrorLog  = "health:global:error_log"
)

// HealthMarker records request stats in Redis (skip /, /health*, favicon).
func HealthMarker(rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()
		if path == "/" || strings.HasPrefix(path, "/health") || strings.HasPrefix(path, "/favicon") {
			return c.Next()
		}

		start := time.Now()
		lastReq := map[string]interface{}{
			"time":   time.Now(),
			"ip":     c.IP(),
			"path":   c.OriginalURL(),
			"method": c.Method(),
		}
		b, _ := json.Marshal(lastReq)
		ctx := context.Background()
		_, _ = rdb.Set(ctx, KeyLastReq, b, 0).Result()
		_, _ = rdb.Incr(ctx, KeyReqTotal).Result()

		err := c.Next()

		ms := time.Since(start).Milliseconds()
		_, _ = rdb.Incr(ctx, KeyResCount).Result()
		_, _ = rdb.IncrByFloat(ctx, KeyResTime, float64(ms)).Result()
		if c.Response().StatusCode() >= 500 {
			_, _ = rdb.Incr(ctx, KeyReqErrors).Result()
		}
		return err
	}
}
