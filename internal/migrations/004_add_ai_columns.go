package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "004",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE planning_cases ADD COLUMN IF NOT EXISTS ai_analysis BOOLEAN NOT NULL DEFAULT FALSE;
				ALTER TABLE planning_cases ADD COLUMN IF NOT EXISTS potential_impact_score INTEGER;
				ALTER TABLE planning_cases ADD COLUMN IF NOT EXISTS tags JSONB;
				ALTER TABLE planning_cases ADD COLUMN IF NOT EXISTS estimated_size INTEGER;
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE planning_cases DROP COLUMN IF EXISTS estimated_size;
				ALTER TABLE planning_cases DROP COLUMN IF EXISTS tags;
				ALTER TABLE planning_cases DROP COLUMN IF EXISTS potential_impact_score;
				ALTER TABLE planning_cases DROP COLUMN IF EXISTS ai_analysis;
			`).Error
		},
	})
}
