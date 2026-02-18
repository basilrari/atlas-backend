package health

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	healthsvc "troo-backend/internal/application/health"
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// Handlers holds dependencies for health endpoints.
type Handlers struct {
	Rdb            *redis.Client
	DB             healthsvc.DBPinger
	HealthAdminKey string
}

// Reset clears health stats in Redis. Requires query key=HEALTH_ADMIN_KEY.
func (h *Handlers) Reset(c *fiber.Ctx) error {
	key := c.Query("key")
	if key == "" || key != h.HealthAdminKey {
		return response.Error(c, "Unauthorized", fiber.StatusForbidden, nil)
	}
	ctx := context.Background()
	keys := []string{middleware.KeyReqTotal, middleware.KeyReqErrors, middleware.KeyResTime, middleware.KeyResCount, middleware.KeyStartTime, middleware.KeyLastReq, middleware.KeyErrorLog}
	if err := h.Rdb.Del(ctx, keys...).Err(); err != nil {
		return response.Error(c, err.Error(), fiber.StatusInternalServerError, nil)
	}
	if err := h.Rdb.Set(ctx, middleware.KeyStartTime, strconv.FormatInt(time.Now().UnixMilli(), 10), 0).Err(); err != nil {
		return response.Error(c, err.Error(), fiber.StatusInternalServerError, nil)
	}
	return response.Success(c, "Stats reset successfully", fiber.Map{"success": true}, nil)
}

// JSON returns health data as JSON (same shape as Express: service + status, runtime, traffic, dependencies).
func (h *Handlers) JSON(c *fiber.Ctx) error {
	ctx := context.Background()
	result := healthsvc.CollectHealth(ctx, h.Rdb, h.DB)
	out := map[string]interface{}{
		"service":      "troo-earth-api",
		"status":       result.Status,
		"runtime":      result.Runtime,
		"traffic":      result.Traffic,
		"dependencies": result.Dependencies,
	}
	return c.JSON(out)
}

// Errors returns the last 50 error log entries from Redis (LRANGE health:global:error_log 0 49).
func (h *Handlers) Errors(c *fiber.Ctx) error {
	ctx := context.Background()
	entries, err := h.Rdb.LRange(ctx, middleware.KeyErrorLog, 0, 49).Result()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON([]interface{}{})
	}
	errors := make([]map[string]interface{}, 0, len(entries))
	for _, s := range entries {
		var m map[string]interface{}
		if _ = json.Unmarshal([]byte(s), &m); m != nil {
			errors = append(errors, m)
		}
	}
	return c.JSON(errors)
}

// Dashboard returns the HTML health status page with embedded health data.
func (h *Handlers) Dashboard(c *fiber.Ctx) error {
	ctx := context.Background()
	result := healthsvc.CollectHealth(ctx, h.Rdb, h.DB)
	html := healthsvc.RenderDashboardHTML(result)
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(html)
}
