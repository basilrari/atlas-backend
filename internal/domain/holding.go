package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Holding matches Express Holdings model (holdingsModel.js).
type Holding struct {
	HoldingID     uuid.UUID `gorm:"column:holding_id;type:uuid;primaryKey" json:"holding_id"`
	OrgID         uuid.UUID `gorm:"column:org_id;type:uuid;not null" json:"org_id"`
	ProjectID     uuid.UUID `gorm:"column:project_id;type:uuid;not null" json:"project_id"`
	VintageYear   *int      `gorm:"column:vintage_year" json:"vintage_year"`
	CreditBalance float64   `gorm:"column:credit_balance;type:decimal(18,2);not null;default:0" json:"credit_balance"`
	LockedForSale float64   `gorm:"column:locked_for_sale;type:decimal(18,2);not null;default:0" json:"locked_for_sale"`
	CreatedAt     time.Time `gorm:"column:createdAt" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updatedAt" json:"updatedAt"`
}

func (Holding) TableName() string {
	return "Holdings"
}

// BeforeCreate: never insert zero UUID for primary key; generate random when not set.
func (h *Holding) BeforeCreate(tx *gorm.DB) error {
	if h.HoldingID == uuid.Nil {
		h.HoldingID = uuid.New()
	}
	return nil
}
