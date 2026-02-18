package trading

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	tradesvc "troo-backend/internal/application/trading"
	"troo-backend/internal/domain"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeStripe struct{}

func (f *fakeStripe) Create(amountCents int64, currency string, metadata map[string]string) (*StripePaymentIntentResult, error) {
	return &StripePaymentIntentResult{
		ID:           "pi_test_123",
		ClientSecret: "pi_test_123_secret_abc",
	}, nil
}

func setupTradingTest(t *testing.T) (*Handlers, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&domain.Listing{}, &domain.Holding{}, &domain.Org{},
		&domain.Transaction{}, &domain.RetirementCertificate{},
		&domain.IcrProject{},
	))
	svc := &tradesvc.Service{DB: db}
	h := &Handlers{Service: svc, StripeCreator: &fakeStripe{}}
	return h, db
}

func TestBuyCredits_MissingFields(t *testing.T) {
	h, _ := setupTradingTest(t)
	app := fiber.New()
	app.Post("/buy-credits", h.BuyCredits)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest("POST", "/buy-credits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestBuyCredits_ReturnsPaymentIntent(t *testing.T) {
	h, _ := setupTradingTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  uuid.New().String(),
		})
		return c.Next()
	})
	app.Post("/buy-credits", h.BuyCredits)

	body, _ := json.Marshal(map[string]interface{}{
		"listing_id": uuid.New().String(),
		"amount":     25.50,
	})
	req := httptest.NewRequest("POST", "/buy-credits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "success", result["status"])
	data, _ := result["data"].(map[string]interface{})
	assert.Equal(t, "pi_test_123", data["payment_intent_id"])
	assert.Equal(t, "pi_test_123_secret_abc", data["client_secret"])
}

func TestSellCredits_MissingOrg(t *testing.T) {
	h, _ := setupTradingTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  "",
		})
		return c.Next()
	})
	app.Post("/sell-credits", h.SellCredits)

	body, _ := json.Marshal(map[string]interface{}{
		"project_id": uuid.New().String(), "amount": 10, "price": 5,
	})
	req := httptest.NewRequest("POST", "/sell-credits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestTransferCredits_MissingFields(t *testing.T) {
	h, _ := setupTradingTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  uuid.New().String(),
		})
		return c.Next()
	})
	app.Post("/transfer-credits", h.TransferCredits)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest("POST", "/transfer-credits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestRetireCredits_MissingFields(t *testing.T) {
	h, _ := setupTradingTest(t)
	app := fiber.New()
	app.Post("/retire-credits", h.RetireCredits)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest("POST", "/retire-credits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}
