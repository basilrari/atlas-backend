package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Invitation matches Express Invitations model (invitationModel.js).
type Invitation struct {
	InviteID    uuid.UUID      `gorm:"column:invite_id;type:uuid;primaryKey" json:"invite_id"`
	OrgID       uuid.UUID      `gorm:"column:org_id;type:uuid;not null" json:"org_id"`
	Email       string         `gorm:"column:email;not null" json:"email"`
	Role        string         `gorm:"column:role;not null" json:"role"`
	InviteToken string         `gorm:"column:invite_token;not null" json:"invite_token"`
	Status      string         `gorm:"column:status;not null;default:'pending'" json:"status"`
	CreatedBy   string         `gorm:"column:created_by;not null" json:"created_by"`
	ExpiresAt   time.Time      `gorm:"column:expires_at;not null" json:"expires_at"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Invitation) TableName() string {
	return "Invitations"
}

func (i *Invitation) BeforeCreate(tx *gorm.DB) error {
	if i.InviteID == uuid.Nil {
		i.InviteID = uuid.New()
	}
	return nil
}
