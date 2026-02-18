package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Holding matches Express Holdings model (holdingsModel.js).
type Holding struct {
	HoldingID     uuid.UUID      `gorm:"column:holding_id;type:uuid;primaryKey" json:"holding_id"`
	OrgID         uuid.UUID      `gorm:"column:org_id;type:uuid;not null" json:"org_id"`
	ProjectID     uuid.UUID      `gorm:"column:project_id;type:uuid;not null" json:"project_id"`
	VintageYear   *int           `gorm:"column:vintage_year" json:"vintage_year"`
	CreditBalance float64        `gorm:"column:credit_balance;type:decimal(18,2);not null;default:0" json:"credit_balance"`
	LockedForSale float64        `gorm:"column:locked_for_sale;type:decimal(18,2);not null;default:0" json:"locked_for_sale"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Holding) TableName() string {
	return "Holdings"
}

