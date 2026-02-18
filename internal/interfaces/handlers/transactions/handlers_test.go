package transactions

import (
	"net/http/httptest"
	"testing"

	txsvc "troo-backend/internal/application/transactions"
	"troo-backend/internal/domain"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTxTest(t *testing.T) (*Handlers, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.Transaction{}, &domain.Org{}, &domain.IcrProject{}))
	svc := &txsvc.Service{DB: db}
	h := &Handlers{Service: svc}
	return h, db
}

func TestGetTransactions_MissingOrgID(t *testing.T) {
	h, _ := setupTxTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  "",
		})
		return c.Next()
	})
	app.Get("/get-transactions", h.GetTransactions)

	req := httptest.NewRequest("GET", "/get-transactions", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestGetTransactions_EmptyResult(t *testing.T) {
	h, _ := setupTxTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  uuid.New().String(),
		})
		return c.Next()
	})
	app.Get("/get-transactions", h.GetTransactions)

	req := httptest.NewRequest("GET", "/get-transactions", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}
