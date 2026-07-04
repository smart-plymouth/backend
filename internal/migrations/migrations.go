package migrations

import (
	"fmt"
	"log"
	"sort"
	"time"

	"gorm.io/gorm"
)

// MigrationRecord tracks which migrations have been applied.
type MigrationRecord struct {
	Version   string    `gorm:"column:version;primaryKey;type:varchar(50)"`
	AppliedAt time.Time `gorm:"column:applied_at;type:timestamptz;autoCreateTime"`
}

func (MigrationRecord) TableName() string {
	return "schema_migrations"
}

// Migration represents a single versioned migration.
type Migration struct {
	Version string
	Up      func(tx *gorm.DB) error
	Down    func(tx *gorm.DB) error
}

var registry []Migration

func Register(m Migration) {
	registry = append(registry, m)
}

// Up applies all pending migrations in order.
func Up(db *gorm.DB) error {
	// Ensure the migrations tracking table exists
	if err := db.AutoMigrate(&MigrationRecord{}); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	sort.Slice(registry, func(i, j int) bool {
		return registry[i].Version < registry[j].Version
	})

	for _, m := range registry {
		var record MigrationRecord
		result := db.Where("version = ?", m.Version).First(&record)
		if result.Error == nil {
			// Already applied
			continue
		}

		log.Printf("Applying migration %s", m.Version)
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := m.Up(tx); err != nil {
				return err
			}
			return tx.Create(&MigrationRecord{Version: m.Version}).Error
		}); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.Version, err)
		}
	}

	return nil
}

// Down reverts the last applied migration.
func Down(db *gorm.DB) error {
	if err := db.AutoMigrate(&MigrationRecord{}); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	sort.Slice(registry, func(i, j int) bool {
		return registry[i].Version < registry[j].Version
	})

	// Find the last applied migration
	var records []MigrationRecord
	if err := db.Order("version DESC").Find(&records).Error; err != nil {
		return err
	}

	if len(records) == 0 {
		log.Println("No migrations to revert")
		return nil
	}

	lastVersion := records[0].Version
	var lastMigration *Migration
	for i := range registry {
		if registry[i].Version == lastVersion {
			lastMigration = &registry[i]
			break
		}
	}

	if lastMigration == nil {
		return fmt.Errorf("migration %s not found in registry", lastVersion)
	}

	log.Printf("Reverting migration %s", lastVersion)
	return db.Transaction(func(tx *gorm.DB) error {
		if err := lastMigration.Down(tx); err != nil {
			return err
		}
		return tx.Where("version = ?", lastVersion).Delete(&MigrationRecord{}).Error
	})
}
