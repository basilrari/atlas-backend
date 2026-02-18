package holdings

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"troo-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupHoldingsTest(t *testing.T) (*Handlers, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Org{}, &models.User{}, &models.Holding{}, &models.IcrProject{}))
	svc := &Service{DB: db}
	h := &Handlers{Service: svc}
	return h, db
}

// ViewHoldings: invalid org_id in session → 400.
func TestViewHoldings_InvalidOrgID(t *testing.T) {
	h, _ := setupHoldingsTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  "not-a-uuid",
		})
		return c.Next()
	})
	app.Get("/api/v1/holdings/view-holdings", h.ViewHoldings)

	req := httptest.NewRequest("GET", "/api/v1/holdings/view-holdings", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ViewProject: missing/invalid holding_id → 400.
func TestViewProject_InvalidHoldingID(t *testing.T) {
	h, _ := setupHoldingsTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  uuid.New().String(),
		})
		return c.Next()
	})
	app.Post("/api/v1/holdings/view-project", h.ViewProject)

	body, _ := json.Marshal(map[string]string{"holding_id": "not-a-uuid"})
	req := httptest.NewRequest("POST", "/api/v1/holdings/view-project", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

