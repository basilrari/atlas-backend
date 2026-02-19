package org

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	orgsvc "troo-backend/internal/application/org"
	"troo-backend/internal/domain"
	"troo-backend/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOrgTest(t *testing.T) (*Handlers, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.Org{}, &domain.User{}))

	service := &orgsvc.Service{DB: db}
	handlers := &Handlers{
		Service: service,
		Config: middleware.SessionConfig{
			AllowCrossSiteDev: false,
			IsProduction:      false,
		},
	}
	return handlers, db
}

// TestCreateOrg_MissingFields returns 400.
func TestCreateOrg_MissingFields(t *testing.T) {
	h, _ := setupOrgTest(t)
	app := fiber.New()
	app.Post("/api/v1/orgs/create-org", h.CreateOrg)

	body, _ := json.Marshal(map[string]string{
		"org_name": "Acme Inc",
		// missing country_code
	})
	req := httptest.NewRequest("POST", "/api/v1/orgs/create-org", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// TestViewOrg_NoOrgOnUser returns 403.
func TestViewOrg_NoOrgOnUser(t *testing.T) {
	h, _ := setupOrgTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id":  uuid.New().String(),
			"fullname": "Test User",
			"email":    "test@example.com",
			"role":     "viewer",
			"org_id":   "",
		})
		return c.Next()
	})
	app.Get("/api/v1/orgs/view-org", h.ViewOrg)

	req := httptest.NewRequest("GET", "/api/v1/orgs/view-org", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

// TestUpdateOrg_NoOrgInSession returns 403 when user has no org_id in session.
func TestUpdateOrg_NoOrgInSession(t *testing.T) {
	h, _ := setupOrgTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{"user_id": "11111111-1111-1111-1111-111111111111", "org_id": nil})
		return c.Next()
	})
	app.Patch("/api/v1/orgs/update-org", h.UpdateOrg)

	body, _ := json.Marshal(map[string]string{"org_name": "New Name"})
	req := httptest.NewRequest("PATCH", "/api/v1/orgs/update-org", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}
