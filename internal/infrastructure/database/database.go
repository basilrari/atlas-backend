package database

import (
	"troo-backend/internal/domain"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open opens a GORM DB from DSN (Supabase/Postgres pooler URL).
// PreferSimpleProtocol disables prepared statement caching to avoid 42P05
// ("prepared statement already exists") when using connection poolers (e.g. PgBouncer, Supabase, Render).
func Open(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
}

// AutoMigrate runs migrations for core models (User for auth).
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&domain.User{})
}
