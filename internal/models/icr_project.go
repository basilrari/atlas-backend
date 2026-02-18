package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// IcrProject matches Express icrProjects model (icrProjects.js).
type IcrProject struct {
	ID                      uuid.UUID     `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	Num                     *int          `gorm:"column:num" json:"num"`
	FullName                *string       `gorm:"column:fullName" json:"fullName"`
	ShortDescription        *string       `gorm:"column:shortDescription" json:"shortDescription"`
	Description             *string       `gorm:"column:description" json:"description"`
	Status                  string        `gorm:"column:status;type:varchar(32);not null" json:"status"`
	Registry                string        `gorm:"column:registry;not null;default:'Carbon registry'" json:"registry"`
	City                    *string       `gorm:"column:city" json:"city"`
	State                   *string       `gorm:"column:state" json:"state"`
	CountryCode             *string       `gorm:"column:countryCode;type:varchar(2)" json:"countryCode"`
	StartDate               *time.Time    `gorm:"column:startDate" json:"startDate"`
	CreditingPeriodStartDate *time.Time   `gorm:"column:creditingPeriodStartDate" json:"creditingPeriodStartDate"`
	Thumbnail               *string       `gorm:"column:thumbnail" json:"thumbnail"`
	PublicURL               *string       `gorm:"column:publicUrl" json:"publicUrl"`
	Sector                  datatypes.JSON `gorm:"column:sector;type:jsonb" json:"sector"`
	Additionalities         datatypes.JSON `gorm:"column:additionalities;type:jsonb" json:"additionalities"`
	OtherBenefits           datatypes.JSON `gorm:"column:otherBenefits;type:jsonb" json:"otherBenefits"`
	Methodology             datatypes.JSON `gorm:"column:methodology;type:jsonb" json:"methodology"`
	Type                    datatypes.JSON `gorm:"column:type;type:jsonb" json:"type"`
	EstimatedAnnualMitigations datatypes.JSON `gorm:"column:estimatedAnnualMitigations;type:jsonb" json:"estimatedAnnualMitigations"`
	Location                datatypes.JSON `gorm:"column:location;type:jsonb" json:"location"`
	KmlFile                 datatypes.JSON `gorm:"column:kmlFile;type:jsonb" json:"kmlFile"`
	Proponents              datatypes.JSON `gorm:"column:proponents;type:jsonb" json:"proponents"`
	Validators              datatypes.JSON `gorm:"column:validators;type:jsonb" json:"validators"`
	Documentation           datatypes.JSON `gorm:"column:documentation;type:jsonb" json:"documentation"`
	SyncedAt                time.Time     `gorm:"column:syncedAt;not null" json:"syncedAt"`
	CreatedAt               time.Time     `json:"createdAt"`
	UpdatedAt               time.Time     `json:"updatedAt"`
	DeletedAt               gorm.DeletedAt `gorm:"index" json:"-"`
}

func (IcrProject) TableName() string {
	return "icrProjects"
}

