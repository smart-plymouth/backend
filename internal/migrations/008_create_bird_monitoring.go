package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "008",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				CREATE TABLE IF NOT EXISTS monitoring_sites (
					site_id UUID PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					latitude DOUBLE PRECISION NOT NULL,
					longitude DOUBLE PRECISION NOT NULL,
					type VARCHAR(50) NOT NULL DEFAULT 'BirdNET-Pi'
				);

				CREATE TABLE IF NOT EXISTS species (
					species_id SERIAL PRIMARY KEY,
					common_name VARCHAR(255) NOT NULL UNIQUE
				);

				CREATE TABLE IF NOT EXISTS species_sightings (
					sighting_id SERIAL PRIMARY KEY,
					site_id UUID NOT NULL REFERENCES monitoring_sites(site_id),
					species_id INTEGER NOT NULL REFERENCES species(species_id),
					confidence DOUBLE PRECISION NOT NULL,
					datetime TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS ix_species_sightings_site_id ON species_sightings(site_id);
				CREATE INDEX IF NOT EXISTS ix_species_sightings_species_id ON species_sightings(species_id);
				CREATE INDEX IF NOT EXISTS ix_species_sightings_datetime ON species_sightings(datetime);
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`
				DROP INDEX IF EXISTS ix_species_sightings_datetime;
				DROP INDEX IF EXISTS ix_species_sightings_species_id;
				DROP INDEX IF EXISTS ix_species_sightings_site_id;
				DROP TABLE IF EXISTS species_sightings;
				DROP TABLE IF EXISTS species;
				DROP TABLE IF EXISTS monitoring_sites;
			`).Error
		},
	})
}
