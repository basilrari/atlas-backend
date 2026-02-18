package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RetirementCertificate struct {
	CertificateID     uuid.UUID      `gorm:"column:certificate_id;type:uuid;primaryKey" json:"certificate_id"`
	OrgID             uuid.UUID      `gorm:"column:org_id;type:uuid;not null" json:"org_id"`
	ProjectID         uuid.UUID      `gorm:"column:project_id;type:uuid;not null" json:"project_id"`
	Amount            float64        `gorm:"column:amount;type:decimal(18,2);not null" json:"amount"`
	RetiredAt         time.Time      `gorm:"column:retired_at;not null" json:"retired_at"`
	Purpose           *string        `gorm:"column:purpose" json:"purpose"`
	Beneficiary       *string        `gorm:"column:beneficiary" json:"beneficiary"`
	TransactionID     uuid.UUID      `gorm:"column:transaction_id;type:uuid;not null" json:"transaction_id"`
	CertificateNumber string         `gorm:"column:certificate_number;uniqueIndex;not null" json:"certificate_number"`
	Status            string         `gorm:"column:status;type:varchar(20);not null;default:'issued'" json:"status"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

func (RetirementCertificate) TableName() string {
	return "RetirementCertificates"
}

func (r *RetirementCertificate) BeforeCreate(tx *gorm.DB) error {
	if r.CertificateID == uuid.Nil {
		r.CertificateID = uuid.New()
	}
	return nil
}
