package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "011",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				CREATE TABLE IF NOT EXISTS planning_supports (
					id SERIAL PRIMARY KEY,
					case_reference VARCHAR(50) NOT NULL REFERENCES planning_cases(reference),
					support_reason TEXT NOT NULL,
					ai_rationalisation TEXT NOT NULL,
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS ix_planning_supports_case_reference
					ON planning_supports(case_reference);
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`DROP TABLE IF EXISTS planning_supports;`).Error
		},
	})
}
