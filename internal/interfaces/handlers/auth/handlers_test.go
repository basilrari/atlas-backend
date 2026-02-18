package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	authsvc "troo-backend/internal/application/auth"
	"troo-backend/internal/domain"
	"troo-backend/internal/middleware"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeUserFinder for tests: returns configured user or error.
type fakeUserFinder struct {
	user *domain.User
	err  error
}

func (f *fakeUserFinder) FindByEmailAndPassword(email, password string) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.user != nil && f.user.Email == email && password == "password123" {
		return f.user, nil
	}
	if f.user != nil && f.user.Email == email {
		return nil, authsvc.ErrIncorrectPassword
	}
	return nil, authsvc.ErrInvalidEmail
}

func setupAuthHandlers(t *testing.T, finder authsvc.UserFinder) (*Handlers, *redis.Client) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		rdb.Close()
		mr.Close()
	})
	h := &Handlers{
		UserFinder: finder,
		Rdb:        rdb,
		Config: middleware.SessionConfig{
			AllowCrossSiteDev: false,
			IsProduction:      false,
		},
	}
	return h, rdb
}

func TestLogin_EmptyBody(t *testing.T) {
	h, _ := setupAuthHandlers(t, &fakeUserFinder{user: &domain.User{}})
	app := fiber.New()
	app.Post("/login", h.Login)

	req := httptest.NewRequest("POST", "/login", nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestLogin_MissingCredentials(t *testing.T) {
	h, _ := setupAuthHandlers(t, &fakeUserFinder{})
	app := fiber.New()
	app.Post("/login", h.Login)

	body, _ := json.Marshal(map[string]string{"email": "a@b.com"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestLogin_InvalidEmail(t *testing.T) {
	h, _ := setupAuthHandlers(t, &fakeUserFinder{}) // no user, so any email returns ErrInvalidEmail
	app := fiber.New()
	app.Post("/login", h.Login)

	body, _ := json.Marshal(map[string]string{"email": "nonexistent@example.com", "password": "any"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestLogin_IncorrectPassword(t *testing.T) {
	uid := uuid.New()
	h, _ := setupAuthHandlers(t, &fakeUserFinder{user: &domain.User{UserID: uid, Email: "test@example.com", Fullname: "Test User", Role: "viewer"}})
	app := fiber.New()
	app.Post("/login", h.Login)

	body, _ := json.Marshal(map[string]string{"email": "test@example.com", "password": "wrong"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestLogin_Success(t *testing.T) {
	uid := uuid.New()
	h, rdb := setupAuthHandlers(t, &fakeUserFinder{user: &domain.User{UserID: uid, Email: "test@example.com", Fullname: "Test User", Role: "viewer"}})
	app := fiber.New()
	app.Post("/login", h.Login)

	body, _ := json.Marshal(map[string]string{"email": "test@example.com", "password": "password123"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	b, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "success", out["status"])
	assert.Equal(t, "Login successful", out["message"])
	data, _ := out["data"].(map[string]interface{})
	require.NotNil(t, data)
	user, _ := data["user"].(map[string]interface{})
	require.NotNil(t, user)
	assert.Equal(t, "test@example.com", user["email"])
	assert.Equal(t, "Test User", user["fullname"])

	cookies := resp.Header.Values("Set-Cookie")
	assert.NotEmpty(t, cookies)
	assert.Contains(t, cookies[0], "troo.sid=")

	keys, err := rdb.Keys(context.Background(), "user_sessions:*").Result()
	require.NoError(t, err)
	assert.NotEmpty(t, keys, "expected user_sessions:* key in Redis")
}

func TestLogin_NilUserFinder(t *testing.T) {
	h, _ := setupAuthHandlers(t, nil)
	app := fiber.New()
	app.Post("/login", h.Login)

	body, _ := json.Marshal(map[string]string{"email": "a@b.com", "password": "pass"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestMe_NoSession(t *testing.T) {
	h, _ := setupAuthHandlers(t, &fakeUserFinder{})
	app := fiber.New()
	app.Get("/me", h.Me)

	req := httptest.NewRequest("GET", "/me", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestMe_WithSessionUserInLocals(t *testing.T) {
	h, _ := setupAuthHandlers(t, &fakeUserFinder{})
	app := fiber.New()
	app.Get("/me", func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id":  "550e8400-e29b-41d4-a716-446655440000",
			"fullname": "Test",
			"email":    "test@example.com",
			"role":     "viewer",
			"org_id":   nil,
		})
		return h.Me(c)
	})

	req := httptest.NewRequest("GET", "/me", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	b, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "success", out["status"])
	assert.Equal(t, "Authenticated", out["message"])
	data, _ := out["data"].(map[string]interface{})
	user, _ := data["user"].(map[string]interface{})
	assert.Equal(t, "test@example.com", user["email"])
}

func TestLogout_NoSession(t *testing.T) {
	h, _ := setupAuthHandlers(t, &fakeUserFinder{})
	app := fiber.New()
	app.Delete("/logout", h.Logout)

	req := httptest.NewRequest("DELETE", "/logout", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	cookies := resp.Header.Values("Set-Cookie")
	assert.NotEmpty(t, cookies)
}
