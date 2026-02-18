package health

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHealthHandlers(t *testing.T) (*Handlers, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		rdb.Close()
		mr.Close()
	})
	return &Handlers{
		Rdb:            rdb,
		DB:             nil,
		HealthAdminKey: "test-admin-key",
	}, mr
}

func TestReset_Unauthorized(t *testing.T) {
	h, _ := setupHealthHandlers(t)
	app := fiber.New()
	app.Get("/reset", h.Reset)

	// No key
	resp, err := app.Test(httptest.NewRequest("GET", "/reset", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, "error", out["status"])
	assert.Equal(t, "Unauthorized", out["error"].(map[string]interface{})["message"])

	// Wrong key
	req := httptest.NewRequest("GET", "/reset?key=wrong", nil)
	resp2, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp2.StatusCode)
}

func TestReset_Success(t *testing.T) {
	h, _ := setupHealthHandlers(t)
	app := fiber.New()
	app.Get("/reset", h.Reset)

	// Set a key so we can verify Del
	ctx := context.Background()
	require.NoError(t, h.Rdb.Set(ctx, "health:global:req_total", "5", 0).Err())
	req := httptest.NewRequest("GET", "/reset?key=test-admin-key", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, "success", out["status"])
	assert.Equal(t, "Stats reset successfully", out["message"])
	// Redis keys cleared; start_time set
	_, err = h.Rdb.Get(ctx, "health:global:req_total").Result()
	assert.Error(t, err)
	_, err = h.Rdb.Get(ctx, "health:global:start_time").Result()
	assert.NoError(t, err)
}

func TestJSON_ReturnsStructure(t *testing.T) {
	h, _ := setupHealthHandlers(t)
	app := fiber.New()
	app.Get("/health/json", h.JSON)

	req := httptest.NewRequest("GET", "/health/json", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, "troo-earth-api", out["service"])
	assert.Contains(t, out, "status")
	assert.Contains(t, out, "runtime")
	assert.Contains(t, out, "traffic")
	assert.Contains(t, out, "dependencies")
}

func TestErrors_ReturnsArray(t *testing.T) {
	h, _ := setupHealthHandlers(t)
	app := fiber.New()
	app.Get("/health/errors", h.Errors)

	// Empty list
	req := httptest.NewRequest("GET", "/health/errors", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	var arr []interface{}
	require.NoError(t, json.Unmarshal(body, &arr))
	assert.Empty(t, arr)

	// With one error entry
	ctx := context.Background()
	h.Rdb.LPush(ctx, "health:global:error_log", `{"time":"2024-01-01T12:00:00Z","path":"/api","method":"GET","message":"test"}`)
	resp2, err := app.Test(httptest.NewRequest("GET", "/health/errors", nil))
	require.NoError(t, err)
	body2, _ := io.ReadAll(resp2.Body)
	var arr2 []map[string]interface{}
	require.NoError(t, json.Unmarshal(body2, &arr2))
	assert.Len(t, arr2, 1)
	assert.Equal(t, "test", arr2[0]["message"])
}

func TestDashboard_ReturnsHTML(t *testing.T) {
	h, _ := setupHealthHandlers(t)
	app := fiber.New()
	app.Get("/", h.Dashboard)

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	assert.Contains(t, html, "Troo Earth · API Status")
	assert.Contains(t, html, "All Systems Operational")
	assert.Contains(t, html, "/health/json")
	assert.Contains(t, html, "/health/errors")
}
