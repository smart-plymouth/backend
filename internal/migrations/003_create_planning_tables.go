package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "003",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				CREATE TABLE IF NOT EXISTS planning_cases (
					reference VARCHAR(50) PRIMARY KEY,
					address TEXT NOT NULL,
					proposal TEXT NOT NULL,
					status VARCHAR(100) NOT NULL,
					received_date DATE,
					validated_date DATE,
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS ix_planning_cases_validated_date
					ON planning_cases(validated_date);
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`
				DROP INDEX IF EXISTS ix_planning_cases_validated_date;
				DROP TABLE IF EXISTS planning_cases;
			`).Error
		},
	})
}
