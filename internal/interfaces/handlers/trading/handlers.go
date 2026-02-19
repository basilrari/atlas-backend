package trading

import (
	"math"
	"os"
	"strconv"

	tradesvc "troo-backend/internal/application/trading"
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/paymentintent"
)

type Handlers struct {
	Service       *tradesvc.Service
	StripeCreator StripePaymentIntentCreator
}

// StripePaymentIntentCreator abstracts Stripe PaymentIntent creation for testability.
type StripePaymentIntentCreator interface {
	Create(amountCents int64, currency string, metadata map[string]string) (*StripePaymentIntentResult, error)
}

type StripePaymentIntentResult struct {
	ID           string `json:"id"`
	ClientSecret string `json:"client_secret"`
}

// RealStripeCreator uses the Stripe Go SDK to create PaymentIntents.
type RealStripeCreator struct {
	SecretKey string
}

func (r *RealStripeCreator) Create(amountCents int64, currency string, metadata map[string]string) (*StripePaymentIntentResult, error) {
	if r.SecretKey == "" {
		return nil, fiber.NewError(501, "Stripe integration pending")
	}
	stripe.Key = r.SecretKey
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountCents),
		Currency: stripe.String(currency),
		Metadata: metadata,
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	pi, err := paymentintent.New(params)
	if err != nil {
		return nil, err
	}
	return &StripePaymentIntentResult{
		ID:           pi.ID,
		ClientSecret: pi.ClientSecret,
	}, nil
}

// BuyCredits POST /api/v1/trading/buy-credits — ONLY creates Stripe PaymentIntent.
func (h *Handlers) BuyCredits(c *fiber.Ctx) error {
	var body struct {
		ListingID string  `json:"listing_id"`
		Amount    float64 `json:"amount"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.Error(c, "Missing required fields", 400, nil)
	}
	if body.ListingID == "" || body.Amount == 0 {
		return response.Error(c, "Missing required fields", 400, nil)
	}
	if _, err := uuid.Parse(body.ListingID); err != nil {
		return response.Error(c, "Invalid UUID format for listing_id", 400, nil)
	}

	actor := getActorTrading(c)
	if actor == nil || actor.OrgID == "" {
		return response.Error(c, "Invalid UUID format for buyer_org_id", 400, nil)
	}
	if _, err := uuid.Parse(actor.OrgID); err != nil {
		return response.Error(c, "Invalid UUID format for buyer_org_id", 400, nil)
	}

	if body.Amount <= 0 {
		return response.Error(c, "Amount must be a positive number", 400, nil)
	}

	amountCents := int64(math.Round(body.Amount * 100))

	if h.StripeCreator == nil {
		return response.Error(c, "Stripe not configured", 500, nil)
	}

	pi, err := h.StripeCreator.Create(amountCents, "sgd", map[string]string{
		"listing_id":     body.ListingID,
		"buyer_org_id":   actor.OrgID,
		"credits_amount": strconv.FormatFloat(body.Amount, 'f', 2, 64),
	})
	if err != nil {
		code := 500
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}
		return response.Error(c, err.Error(), code, nil)
	}

	return response.Success(c, "Payment intent created", fiber.Map{
		"payment_intent_id": pi.ID,
		"client_secret":     pi.ClientSecret,
	}, nil)
}

// SellCredits POST /api/v1/trading/sell-credits
func (h *Handlers) SellCredits(c *fiber.Ctx) error {
	var body struct {
		ProjectID string  `json:"project_id"`
		Amount    float64 `json:"amount"`
		Price     float64 `json:"price"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.Error(c, "Missing required fields", 400, nil)
	}

	actor := getActorTrading(c)
	if actor == nil || actor.OrgID == "" {
		return response.Error(c, "User not associated with organization", 403, nil)
	}
	if body.ProjectID == "" || body.Amount == 0 || body.Price == 0 {
		return response.Error(c, "Missing required fields", 400, nil)
	}
	orgID, _ := uuid.Parse(actor.OrgID)
	projectID, err := uuid.Parse(body.ProjectID)
	if err != nil {
		return response.Error(c, "Invalid project_id", 400, nil)
	}
	if body.Amount <= 0 {
		return response.Error(c, "Invalid amount", 400, nil)
	}
	if body.Price <= 0 {
		return response.Error(c, "Invalid price", 400, nil)
	}

	result, err := h.Service.SellCredits(c.Context(), orgID, projectID, body.Amount, body.Price)
	if err != nil {
		statusMap := map[string]int{
			"Org not found":                      404,
			"No holdings found for this project":  404,
			"Insufficient credits to sell":         400,
			"Project not found":                    404,
		}
		if code, ok := statusMap[err.Error()]; ok {
			return response.Error(c, err.Error(), code, nil)
		}
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	return response.Success(c, "Listing created/updated successfully", result, nil)
}

// TransferCredits POST /api/v1/trading/transfer-credits
func (h *Handlers) TransferCredits(c *fiber.Ctx) error {
	var body struct {
		ToOrgCode string  `json:"to_org_code"`
		ProjectID string  `json:"project_id"`
		Amount    float64 `json:"amount"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.Error(c, "Missing required fields", 400, nil)
	}

	actor := getActorTrading(c)
	if actor == nil || actor.OrgID == "" {
		return response.Error(c, "User not associated with an organization", 403, nil)
	}
	if body.ToOrgCode == "" || body.ProjectID == "" || body.Amount == 0 {
		return response.Error(c, "Missing required fields", 400, nil)
	}
	fromOrgID, _ := uuid.Parse(actor.OrgID)
	projectID, err := uuid.Parse(body.ProjectID)
	if err != nil {
		return response.Error(c, "Invalid UUID format for project_id", 400, nil)
	}
	if body.Amount <= 0 {
		return response.Error(c, "Amount must be a positive number", 400, nil)
	}

	result, err := h.Service.TransferCredits(c.Context(), fromOrgID, projectID, body.ToOrgCode, body.Amount)
	if err != nil {
		statusMap := map[string]int{
			"Cannot transfer to the same organization":    400,
			"No Holdings found for this project":           404,
			"Insufficient available credits to transfer":   400,
			"Target organization not found":                404,
		}
		if code, ok := statusMap[err.Error()]; ok {
			return response.Error(c, err.Error(), code, nil)
		}
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	return response.Success(c, "Transfer successful", result, nil)
}

// RetireCredits POST /api/v1/trading/retire-credits — org_id from session (user can only retire their org's credits).
func (h *Handlers) RetireCredits(c *fiber.Ctx) error {
	actor := getActorTrading(c)
	if actor == nil || actor.OrgID == "" {
		return response.Error(c, "User is not associated with any organization", 403, nil)
	}
	orgID, err := uuid.Parse(actor.OrgID)
	if err != nil {
		return response.Error(c, "User is not associated with any organization", 403, nil)
	}

	var body struct {
		ProjectID   string  `json:"project_id"`
		Amount      float64 `json:"amount"`
		Purpose     *string `json:"purpose"`
		Beneficiary *string `json:"beneficiary"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.Error(c, "project_id and amount are required", 400, nil)
	}
	if body.ProjectID == "" || body.Amount == 0 {
		return response.Error(c, "project_id and amount are required", 400, nil)
	}
	if body.Amount <= 0 {
		return response.Error(c, "Amount must be a positive number", 400, nil)
	}

	projectID, err := uuid.Parse(body.ProjectID)
	if err != nil {
		return response.Error(c, "Invalid project_id", 400, nil)
	}

	result, err := h.Service.RetireCredits(c.Context(), orgID, projectID, body.Amount, body.Purpose, body.Beneficiary)
	if err != nil {
		return response.Error(c, err.Error(), 400, nil)
	}
	return response.Success(c, "Credits retired successfully", result, nil)
}

type tradingActor struct {
	UserID string
	OrgID  string
}

func getActorTrading(c *fiber.Ctx) *tradingActor {
	u := middleware.GetUser(c)
	if u == nil {
		return nil
	}
	m, ok := u.(map[string]interface{})
	if !ok {
		return nil
	}
	userID, _ := m["user_id"].(string)
	orgID := ""
	if o, ok := m["org_id"]; ok && o != nil {
		if s, ok := o.(string); ok {
			orgID = s
		}
	}
	return &tradingActor{UserID: userID, OrgID: orgID}
}

func init() {
	_ = os.Getenv("STRIPE_SECRET_KEY")
}
