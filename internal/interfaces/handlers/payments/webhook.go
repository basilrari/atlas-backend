package payments

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"troo-backend/internal/domain"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	DB            *gorm.DB
	WebhookSecret string
}

type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type paymentIntentObject struct {
	ID             string            `json:"id"`
	AmountReceived int               `json:"amount_received"`
	Currency       string            `json:"currency"`
	Status         string            `json:"status"`
	Metadata       map[string]string `json:"metadata"`
}

// HandleWebhook POST /api/v1/stripe/webhook — raw body, signature verification, then process.
func (wh *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	rawBody := c.BodyRaw()
	// Stripe sends "Stripe-Signature"; Fiber's Get is case-insensitive
	sig := c.Get("Stripe-Signature")

	if len(rawBody) == 0 {
		log.Warn().Msg("Stripe webhook received empty body (ensure no global body parser consumes the webhook body)")
		return c.Status(400).SendString("Webhook Error: empty body")
	}

	if err := verifyStripeSignature(rawBody, sig, wh.WebhookSecret); err != nil {
		log.Warn().Err(err).Bool("has_sig", sig != "").Bool("has_secret", wh.WebhookSecret != "").Msg("Stripe webhook signature verification failed")
		return c.Status(400).SendString(fmt.Sprintf("Webhook Error: %s", err.Error()))
	}

	var event stripeEvent
	if err := json.Unmarshal(rawBody, &event); err != nil {
		log.Warn().Err(err).Msg("Stripe webhook JSON parse failed")
		return c.Status(400).SendString(fmt.Sprintf("Webhook Error: %s", err.Error()))
	}

	if event.Type == "payment_intent.succeeded" {
		var pi paymentIntentObject
		if err := json.Unmarshal(event.Data.Object, &pi); err != nil {
			return c.Status(200).SendString("ok")
		}

		if err := wh.handlePaymentIntentSucceeded(pi, event.ID, rawBody); err != nil {
			// Express always returns 200 for domain errors too (to avoid Stripe retries)
			return c.Status(200).SendString("ok")
		}
	}

	return c.Status(200).SendString("ok")
}

func (wh *WebhookHandler) handlePaymentIntentSucceeded(pi paymentIntentObject, eventID string, rawBody []byte) error {
	listingID := pi.Metadata["listing_id"]
	buyerOrgID := pi.Metadata["buyer_org_id"]
	creditsAmountStr := pi.Metadata["credits_amount"]

	if listingID == "" || buyerOrgID == "" || creditsAmountStr == "" {
		return nil // skip silently, like Express
	}

	amount, err := strconv.ParseFloat(creditsAmountStr, 64)
	if err != nil || amount <= 0 {
		return nil
	}

	return wh.DB.Transaction(func(tx *gorm.DB) error {
		// Idempotency: check existing payment
		var existing domain.Payment
		if err := tx.Where("stripe_payment_intent_id = ?", pi.ID).First(&existing).Error; err == nil {
			return nil // already processed
		}

		// Create Payment record
		listingUUID, _ := uuid.Parse(listingID)
		buyerUUID, _ := uuid.Parse(buyerOrgID)

		payment := domain.Payment{
			StripePaymentIntentID: pi.ID,
			StripeEventID:         eventID,
			BuyerOrgID:            buyerUUID,
			ListingID:             listingUUID,
			CreditsAmount:         amount,
			AmountPaidCents:       pi.AmountReceived,
			Currency:              pi.Currency,
			Status:                pi.Status,
			RawPaymentIntent:      rawBody,
		}
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}

		// Call buyCreditsService logic (same as Express tradingService.buyCreditsService)
		return buyCreditsInTransaction(tx, listingUUID, buyerUUID, amount)
	})
}

// buyCreditsInTransaction mirrors Express buyCreditsService({ transaction }).
func buyCreditsInTransaction(tx *gorm.DB, listingID, buyerOrgID uuid.UUID, amount float64) error {
	var listing domain.Listing
	if err := tx.Where("listing_id = ?", listingID).First(&listing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("Listing not found")
		}
		return err
	}
	if listing.Status != "open" {
		return errors.New("Listing is not open for purchase")
	}
	if listing.CreditsAvailable < amount {
		return errors.New("Insufficient credits available in the listing")
	}

	listing.CreditsAvailable = math.Round((listing.CreditsAvailable-amount)*100) / 100
	if listing.CreditsAvailable == 0 {
		listing.Status = "closed"
	}
	if err := tx.Save(&listing).Error; err != nil {
		return err
	}

	// Listing events (PARTIALLY_FILLED or FILLED, then CLOSED if fully filled) — match Express buyCreditsService
	var buyerOrg domain.Org
	if err := tx.Where("org_id = ?", buyerOrgID).Select("org_code").First(&buyerOrg).Error; err != nil {
		return errors.New("Buyer org not found")
	}
	remainingCredits := listing.CreditsAvailable
	eventType := "FILLED"
	if remainingCredits > 0 {
		eventType = "PARTIALLY_FILLED"
	}
	fillEventData, _ := json.Marshal(map[string]interface{}{
		"bought_quantity":    amount,
		"remaining_quantity": remainingCredits,
		"price_per_credit":   listing.PricePerCredit,
	})
	if err := tx.Create(&domain.ListingEvent{
		ListingID:    listing.ListingID,
		EventType:    eventType,
		ActorOrgCode: &buyerOrg.OrgCode,
		EventData:    datatypes.JSON(fillEventData),
	}).Error; err != nil {
		return err
	}
	if remainingCredits == 0 {
		closedEventData, _ := json.Marshal(map[string]interface{}{"reason": "fully_filled"})
		if err := tx.Create(&domain.ListingEvent{
			ListingID:    listing.ListingID,
			EventType:    "CLOSED",
			ActorOrgCode: nil,
			EventData:    datatypes.JSON(closedEventData),
		}).Error; err != nil {
			return err
		}
	}

	// Reduce seller holdings (if org-owned listing)
	if listing.SellerID != nil {
		var sellerHolding domain.Holding
		if err := tx.Where("org_id = ? AND project_id = ?", listing.SellerID, listing.ProjectID).First(&sellerHolding).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.New("Seller holdings not found")
			}
			return err
		}
		if sellerHolding.LockedForSale < amount {
			return errors.New("Seller does not have enough locked credits")
		}
		sellerHolding.LockedForSale = math.Round((sellerHolding.LockedForSale-amount)*100) / 100
		sellerHolding.CreditBalance = math.Round((sellerHolding.CreditBalance-amount)*100) / 100
		if err := tx.Save(&sellerHolding).Error; err != nil {
			return err
		}
	}

	// Add credits to buyer holdings
	var buyerHolding domain.Holding
	err := tx.Where("org_id = ? AND project_id = ?", buyerOrgID, listing.ProjectID).First(&buyerHolding).Error
	if err == gorm.ErrRecordNotFound {
		buyerHolding = domain.Holding{
			OrgID:         buyerOrgID,
			ProjectID:     listing.ProjectID,
			CreditBalance: amount,
		}
		if err := tx.Create(&buyerHolding).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		buyerHolding.CreditBalance = math.Round((buyerHolding.CreditBalance+amount)*100) / 100
		if err := tx.Save(&buyerHolding).Error; err != nil {
			return err
		}
	}

	// Transaction record
	sellerID := listing.SellerID
	txRecord := domain.Transaction{
		Type:             "buy",
		FromOrgID:        sellerID,
		ToOrgID:          &buyerOrgID,
		ProjectID:        listing.ProjectID,
		Amount:           amount,
		RelatedListingID: &listing.ListingID,
	}
	return tx.Create(&txRecord).Error
}

// verifyStripeSignature verifies the Stripe-Signature header using the webhook secret.
func verifyStripeSignature(payload []byte, sigHeader, secret string) error {
	if sigHeader == "" || secret == "" {
		return errors.New("missing signature or secret")
	}

	var timestamp string
	var signatures []string

	parts := strings.Split(sigHeader, ",")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}

	if timestamp == "" || len(signatures) == 0 {
		return errors.New("invalid signature format")
	}

	signedPayload := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expectedSig)) {
			// Check tolerance (5 minutes)
			ts, err := strconv.ParseInt(timestamp, 10, 64)
			if err != nil {
				return errors.New("invalid timestamp")
			}
			diff := time.Now().Unix() - ts
			if diff < 0 {
				diff = -diff
			}
			if diff > 300 {
				return errors.New("timestamp too old")
			}
			return nil
		}
	}

	return errors.New("signature mismatch")
}
