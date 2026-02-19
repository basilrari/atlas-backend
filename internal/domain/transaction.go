package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Transaction struct {
	TxID             uuid.UUID      `gorm:"column:tx_id;type:uuid;primaryKey" json:"tx_id"`
	Type             string         `gorm:"column:type;type:varchar(20);not null" json:"type"`
	ProjectID        uuid.UUID      `gorm:"column:project_id;type:uuid;not null" json:"project_id"`
	FromOrgID        *uuid.UUID     `gorm:"column:from_org_id;type:uuid" json:"from_org_id"`
	ToOrgID          *uuid.UUID     `gorm:"column:to_org_id;type:uuid" json:"to_org_id"`
	Amount           float64        `gorm:"column:amount;type:decimal(18,2);not null" json:"amount"`
	RelatedListingID *uuid.UUID `gorm:"column:related_listing_id;type:uuid" json:"related_listing_id"`
	CreatedAt        time.Time  `gorm:"column:createdAt" json:"createdAt"`
	UpdatedAt        time.Time  `gorm:"column:updatedAt" json:"updatedAt"`
}

func (Transaction) TableName() string {
	return "Transactions"
}

func (t *Transaction) BeforeCreate(tx *gorm.DB) error {
	if t.TxID == uuid.Nil {
		t.TxID = uuid.New()
	}
	return nil
}
