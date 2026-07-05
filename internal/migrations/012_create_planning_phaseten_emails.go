package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "012",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				CREATE TABLE IF NOT EXISTS planning_phaseten_emails (
					id SERIAL PRIMARY KEY,
					email VARCHAR(255) NOT NULL UNIQUE,
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`DROP TABLE IF EXISTS planning_phaseten_emails;`).Error
		},
	})
}
