package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "001",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				DO $$
				BEGIN
					IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'location_type') THEN
						CREATE TYPE location_type AS ENUM (
							'emergency_department',
							'urgent_treatment_centre',
							'minor_injuries_unit'
						);
					END IF;
				END$$;

				CREATE TABLE IF NOT EXISTS ed_wait_times_locations (
					id UUID PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					type VARCHAR(50) NOT NULL,
					address TEXT NOT NULL,
					longitude DOUBLE PRECISION NOT NULL,
					latitude DOUBLE PRECISION NOT NULL,
					opening_times TEXT,
					telephone_number VARCHAR(50)
				);

				CREATE TABLE IF NOT EXISTS ed_wait_times_wait_times (
					location_id UUID NOT NULL REFERENCES ed_wait_times_locations(id),
					timestamp TIMESTAMPTZ NOT NULL,
					longest_wait INTEGER NOT NULL,
					patients_waiting INTEGER NOT NULL,
					patients_in_department INTEGER NOT NULL,
					PRIMARY KEY (location_id, timestamp)
				);
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`
				DROP TABLE IF EXISTS ed_wait_times_wait_times;
				DROP TABLE IF EXISTS ed_wait_times_locations;
				DROP TYPE IF EXISTS location_type;
			`).Error
		},
	})
}
