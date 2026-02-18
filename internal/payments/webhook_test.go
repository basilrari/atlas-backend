package payments

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"troo-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const testSecret = "whsec_test_secret_123"

func setupWebhookTest(t *testing.T) (*WebhookHandler, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Listing{}, &models.Holding{}, &models.Payment{},
		&models.Transaction{}, &models.Org{},
	))
	wh := &WebhookHandler{DB: db, WebhookSecret: testSecret}
	return wh, db
}

func signPayload(t *testing.T, payload []byte, secret string) string {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(payload)))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%s,v1=%s", ts, sig)
}

func TestWebhook_MissingSignature(t *testing.T) {
	wh, _ := setupWebhookTest(t)
	app := fiber.New()
	app.Post("/webhook", wh.HandleWebhook)

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestWebhook_InvalidSignature(t *testing.T) {
	wh, _ := setupWebhookTest(t)
	app := fiber.New()
	app.Post("/webhook", wh.HandleWebhook)

	body := []byte(`{"type":"payment_intent.succeeded"}`)
	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("stripe-signature", "t=123,v1=invalid")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestWebhook_ValidSignature_Returns200(t *testing.T) {
	wh, _ := setupWebhookTest(t)
	app := fiber.New()
	app.Post("/webhook", wh.HandleWebhook)

	event := map[string]interface{}{
		"id":   "evt_test_123",
		"type": "charge.succeeded",
		"data": map[string]interface{}{
			"object": map[string]interface{}{},
		},
	}
	body, _ := json.Marshal(event)
	sig := signPayload(t, body, testSecret)

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("stripe-signature", sig)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestWebhook_PaymentIntentSucceeded_BuysCredits(t *testing.T) {
	wh, db := setupWebhookTest(t)

	sellerOrgID := uuid.New()
	buyerOrgID := uuid.New()
	projectID := uuid.New()

	require.NoError(t, db.Create(&models.Listing{
		ListingID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ProjectID: projectID, SellerID: &sellerOrgID,
		CreditsAvailable: 100, PricePerCredit: 5, Status: "open",
		ProjectName: "Test", Registry: "R", Category: "C",
		LocationCity: "X", LocationState: "Y", LocationCountry: "Z",
		ThumbnailURL: "u", Methodology: "M",
	}).Error)

	require.NoError(t, db.Create(&models.Holding{
		HoldingID: uuid.New(), OrgID: sellerOrgID, ProjectID: projectID,
		CreditBalance: 100, LockedForSale: 100,
	}).Error)

	piObj := map[string]interface{}{
		"id":              "pi_test_buy_001",
		"amount_received": 5000,
		"currency":        "sgd",
		"status":          "succeeded",
		"metadata": map[string]string{
			"listing_id":     "11111111-1111-1111-1111-111111111111",
			"buyer_org_id":   buyerOrgID.String(),
			"credits_amount": "10",
		},
	}
	event := map[string]interface{}{
		"id":   "evt_test_buy_001",
		"type": "payment_intent.succeeded",
		"data": map[string]interface{}{
			"object": piObj,
		},
	}
	body, _ := json.Marshal(event)
	sig := signPayload(t, body, testSecret)

	app := fiber.New()
	app.Post("/webhook", wh.HandleWebhook)

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("stripe-signature", sig)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Verify payment record was created
	var payment models.Payment
	require.NoError(t, db.Where("stripe_payment_intent_id = ?", "pi_test_buy_001").First(&payment).Error)
	assert.Equal(t, 10.0, payment.CreditsAmount)

	// Verify listing was updated
	var listing models.Listing
	require.NoError(t, db.Where("listing_id = ?", "11111111-1111-1111-1111-111111111111").First(&listing).Error)
	assert.Equal(t, 90.0, listing.CreditsAvailable)

	// Verify buyer holding was created
	var buyerHolding models.Holding
	require.NoError(t, db.Where("org_id = ? AND project_id = ?", buyerOrgID, projectID).First(&buyerHolding).Error)
	assert.Equal(t, 10.0, buyerHolding.CreditBalance)

	// Verify seller holding was decremented
	var sellerHolding models.Holding
	require.NoError(t, db.Where("org_id = ? AND project_id = ?", sellerOrgID, projectID).First(&sellerHolding).Error)
	assert.Equal(t, 90.0, sellerHolding.CreditBalance)
	assert.Equal(t, 90.0, sellerHolding.LockedForSale)
}
