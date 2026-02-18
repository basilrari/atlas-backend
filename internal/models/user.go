package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User matches Express Users table (userModel.js).
type User struct {
	UserID      uuid.UUID      `gorm:"column:user_id;type:uuid;primaryKey" json:"user_id"`
	Fullname    string         `gorm:"column:fullname;not null" json:"fullname"`
	UserName    string         `gorm:"column:user_name;not null" json:"user_name"`
	Email       string         `gorm:"column:email;not null;uniqueIndex" json:"email"`
	PasswordHash string        `gorm:"column:password_hash;not null" json:"-"`
	OrgID       *uuid.UUID     `gorm:"column:org_id;type:uuid" json:"org_id"`
	Role        string         `gorm:"column:role;not null;default:viewer" json:"role"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides table name to Users (Express tableName).
func (User) TableName() string {
	return "Users"
}

// BeforeCreate sets UUID if not set (for DBs without gen_random_uuid).
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.UserID == uuid.Nil {
		u.UserID = uuid.New()
	}
	return nil
}
