package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "013",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				CREATE TABLE IF NOT EXISTS airquality_sites (
					site_id UUID PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					latitude DOUBLE PRECISION NOT NULL,
					longitude DOUBLE PRECISION NOT NULL
				);

				CREATE TABLE IF NOT EXISTS airquality_readings (
					reading_id SERIAL PRIMARY KEY,
					site_id UUID NOT NULL REFERENCES airquality_sites(site_id),
					datetime TIMESTAMPTZ NOT NULL
				);

				CREATE INDEX IF NOT EXISTS ix_airquality_readings_site_id ON airquality_readings(site_id);
				CREATE INDEX IF NOT EXISTS ix_airquality_readings_datetime ON airquality_readings(datetime);

				CREATE TABLE IF NOT EXISTS airquality_metrics (
					metric_id SERIAL PRIMARY KEY,
					reading_id INTEGER NOT NULL REFERENCES airquality_readings(reading_id),
					pollutant VARCHAR(50) NOT NULL,
					value DOUBLE PRECISION NOT NULL,
					unit VARCHAR(50) NOT NULL
				);

				CREATE INDEX IF NOT EXISTS ix_airquality_metrics_reading_id ON airquality_metrics(reading_id);
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`
				DROP INDEX IF EXISTS ix_airquality_metrics_reading_id;
				DROP TABLE IF EXISTS airquality_metrics;
				DROP INDEX IF EXISTS ix_airquality_readings_datetime;
				DROP INDEX IF EXISTS ix_airquality_readings_site_id;
				DROP TABLE IF EXISTS airquality_readings;
				DROP TABLE IF EXISTS airquality_sites;
			`).Error
		},
	})
}
