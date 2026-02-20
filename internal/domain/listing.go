package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SDGNumbers stores the DB json value as string but marshals to JSON as an array so the API matches Express (frontend expects an array for .map()).
type SDGNumbers string

// MarshalJSON implements json.Marshaler so API responses send sdg_numbers as [7, 13] not "[7,13]".
func (s SDGNumbers) MarshalJSON() ([]byte, error) {
	if s == "" {
		return []byte("[]"), nil
	}
	var arr []interface{}
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return []byte("[]"), nil
	}
	return json.Marshal(arr)
}

// UnmarshalJSON implements json.Unmarshaler for reading from request body.
func (s *SDGNumbers) UnmarshalJSON(data []byte) error {
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	bs, err := json.Marshal(arr)
	if err != nil {
		return err
	}
	*s = SDGNumbers(bs)
	return nil
}

// Scan implements sql.Scanner for reading from DB (json column).
func (s *SDGNumbers) Scan(value interface{}) error {
	if value == nil {
		*s = ""
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*s = SDGNumbers(v)
		return nil
	case string:
		*s = SDGNumbers(v)
		return nil
	default:
		return errors.New("unsupported type for SDGNumbers")
	}
}

// Value implements driver.Valuer for writing to DB.
func (s SDGNumbers) Value() (driver.Value, error) {
	if s == "" {
		return "[]", nil
	}
	return string(s), nil
}

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
	SdgNumbers       SDGNumbers     `gorm:"column:sdg_numbers;type:json" json:"sdg_numbers"`
	Methodology      string         `gorm:"column:methodology;not null" json:"methodology"`
	VintageYear      int            `gorm:"column:vintage_year;not null" json:"vintage_year"`
	CreatedAt time.Time `gorm:"column:createdAt" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updatedAt" json:"updatedAt"`
}

func (Listing) TableName() string {
	return "Listings"
}

// BeforeCreate sets listing_id if not already set (DBs without default uuid).
func (l *Listing) BeforeCreate(tx *gorm.DB) error {
	if l.ListingID == uuid.Nil {
		l.ListingID = uuid.New()
	}
	return nil
}
