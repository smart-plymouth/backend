package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "006",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				CREATE TABLE IF NOT EXISTS planning_objections (
					id SERIAL PRIMARY KEY,
					case_reference VARCHAR(50) NOT NULL REFERENCES planning_cases(reference),
					objection TEXT NOT NULL,
					ai_rationalisation TEXT NOT NULL,
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS ix_planning_objections_case_reference
					ON planning_objections(case_reference);
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`DROP TABLE IF EXISTS planning_objections;`).Error
		},
	})
}
