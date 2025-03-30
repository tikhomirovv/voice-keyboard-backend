package migrations

import (
	"sort"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var migrations []*gormigrate.Migration

// registerMigration adds a migration to the list
func registerMigration(m *gormigrate.Migration) {
	migrations = append(migrations, m)
}

// NewMigrator creates a new gormigrate instance with all migrations
func NewMigrator(db *gorm.DB) *gormigrate.Gormigrate {
	// Sort migrations by ID to ensure they run in the correct order
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].ID < migrations[j].ID
	})

	return gormigrate.New(db, gormigrate.DefaultOptions, migrations)
}
