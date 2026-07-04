package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "007",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE planning_cases ADD COLUMN IF NOT EXISTS pros JSONB;
				ALTER TABLE planning_cases ADD COLUMN IF NOT EXISTS cons JSONB;
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE planning_cases DROP COLUMN IF EXISTS cons;
				ALTER TABLE planning_cases DROP COLUMN IF EXISTS pros;
			`).Error
		},
	})
}
