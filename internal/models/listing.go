package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Listing matches Express Listings model (listingModel.js).
type Listing struct {
	ListingID        uuid.UUID      `gorm:"column:listing_id;type:uuid;primaryKey" json:"listing_id"`
	ProjectID        uuid.UUID      `gorm:"column:project_id;type:uuid;not null" json:"project_id"`
	SellerID         *uuid.UUID     `gorm:"column:seller_id;type:uuid" json:"seller_id"`
	CreditsAvailable float64        `gorm:"column:credits_available;type:decimal(18,2);not null" json:"credits_available"`
	PricePerCredit   float64        `gorm:"column:price_per_credit;type:decimal(18,2);not null" json:"price_per_credit"`
	ExternalTradeID  *string        `gorm:"column:external_trade_id" json:"external_trade_id"`
	ProjectName      string         `gorm:"column:project_name;not null" json:"project_name"`
	ProjectStartYear int            `gorm:"column:project_start_year;not null" json:"project_start_year"`
	Registry         string         `gorm:"column:registry;not null" json:"registry"`
	Category         string         `gorm:"column:category;not null" json:"category"`
	LocationCity     string         `gorm:"column:location_city;not null" json:"location_city"`
	LocationState    string         `gorm:"column:location_state;not null" json:"location_state"`
	LocationCountry  string         `gorm:"column:location_country;not null" json:"location_country"`
	ThumbnailURL     string         `gorm:"column:thumbnail_url;not null" json:"thumbnail_url"`
	Status           string         `gorm:"column:status;type:varchar(20);default:'open'" json:"status"`
	SdgNumbers       string         `gorm:"column:sdg_numbers;type:json" json:"sdg_numbers"`
	Methodology      string         `gorm:"column:methodology;not null" json:"methodology"`
	VintageYear      int            `gorm:"column:vintage_year;not null" json:"vintage_year"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Listing) TableName() string {
	return "Listings"
}

