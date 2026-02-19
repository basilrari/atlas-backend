package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Payment struct {
	ID                     uuid.UUID      `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	StripePaymentIntentID  string         `gorm:"column:stripe_payment_intent_id;uniqueIndex;not null" json:"stripe_payment_intent_id"`
	StripeEventID          string         `gorm:"column:stripe_event_id;uniqueIndex;not null" json:"stripe_event_id"`
	BuyerOrgID             uuid.UUID      `gorm:"column:buyer_org_id;type:uuid;not null" json:"buyer_org_id"`
	ListingID              uuid.UUID      `gorm:"column:listing_id;type:uuid;not null" json:"listing_id"`
	CreditsAmount          float64        `gorm:"column:credits_amount;type:decimal;not null" json:"credits_amount"`
	AmountPaidCents        int            `gorm:"column:amount_paid_cents;not null" json:"amount_paid_cents"`
	Currency               string         `gorm:"column:currency;not null" json:"currency"`
	Status                 string         `gorm:"column:status;not null" json:"status"`
	RawPaymentIntent       datatypes.JSON `gorm:"column:raw_payment_intent;type:jsonb;not null" json:"raw_payment_intent"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

func (Payment) TableName() string {
	return "Payments"
}

func (p *Payment) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
