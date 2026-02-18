package database

import (
	"troo-backend/internal/domain"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open opens a GORM DB from DSN (Supabase/Postgres pooler URL).
func Open(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

// AutoMigrate runs migrations for core models (User for auth).
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&domain.User{})
}
