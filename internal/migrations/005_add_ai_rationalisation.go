package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "005",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE planning_cases ADD COLUMN IF NOT EXISTS ai_rationalisation TEXT;
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE planning_cases DROP COLUMN IF EXISTS ai_rationalisation;
			`).Error
		},
	})
}
