package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Org matches the Express Orgs table (orgModel.js).
type Org struct {
	OrgID              uuid.UUID      `gorm:"column:org_id;type:uuid;primaryKey" json:"org_id"`
	OrgName            string         `gorm:"column:org_name;not null;uniqueIndex" json:"org_name"`
	OrgCode            string         `gorm:"column:org_code;type:varchar(10);not null;uniqueIndex" json:"org_code"`
	CountryCode        string         `gorm:"column:country_code;type:char(2);not null" json:"country_code"`
	RegistrationID     *string        `gorm:"column:registration_id" json:"registration_id"`
	LogoURL            *string        `gorm:"column:logo_url" json:"logo_url"`
	IncorporationDocURL *string       `gorm:"column:incorporation_doc_url" json:"incorporation_doc_url"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Org) TableName() string {
	return "Orgs"
}

// BeforeCreate ensures org_id is set for DBs without default uuid.
func (o *Org) BeforeCreate(tx *gorm.DB) error {
	if o.OrgID == uuid.Nil {
		o.OrgID = uuid.New()
	}
	return nil
}
