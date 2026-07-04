package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "010",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE species ADD COLUMN IF NOT EXISTS scientific_name VARCHAR(255);
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE species DROP COLUMN IF EXISTS scientific_name;
			`).Error
		},
	})
}
