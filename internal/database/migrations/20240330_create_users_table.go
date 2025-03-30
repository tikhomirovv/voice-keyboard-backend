package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func init() {
	registerMigration(&gormigrate.Migration{
		ID: "20240330_create_users_table",
		Migrate: func(tx *gorm.DB) error {
			// Enable uuid-ossp extension
			if err := tx.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`).Error; err != nil {
				return err
			}

			// Create users table
			return tx.Exec(`
				CREATE TABLE users (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					created_at BIGINT,
					updated_at BIGINT,
					deleted_at TIMESTAMP NULL
				)
			`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Exec("DROP TABLE IF EXISTS users").Error
		},
	})
}
