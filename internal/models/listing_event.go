package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ListingEvent struct {
	EventID      uuid.UUID      `gorm:"column:event_id;type:uuid;primaryKey" json:"event_id"`
	ListingID    uuid.UUID      `gorm:"column:listing_id;type:uuid;not null" json:"listing_id"`
	EventType    string         `gorm:"column:event_type;type:varchar(30);not null" json:"event_type"`
	EventData    datatypes.JSON `gorm:"column:event_data;type:jsonb;not null" json:"event_data"`
	ActorOrgCode *string        `gorm:"column:actor_org_code" json:"actor_org_code"`
	CreatedAt    time.Time      `json:"createdAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ListingEvent) TableName() string {
	return "ListingEvents"
}

func (le *ListingEvent) BeforeCreate(tx *gorm.DB) error {
	if le.EventID == uuid.Nil {
		le.EventID = uuid.New()
	}
	return nil
}
