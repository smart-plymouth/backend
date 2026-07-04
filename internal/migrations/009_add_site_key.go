package migrations

import "gorm.io/gorm"

func init() {
	Register(Migration{
		Version: "009",
		Up: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE monitoring_sites ADD COLUMN IF NOT EXISTS site_key VARCHAR(255);
				CREATE UNIQUE INDEX IF NOT EXISTS ix_monitoring_sites_site_key ON monitoring_sites(site_key);
			`).Error
		},
		Down: func(tx *gorm.DB) error {
			return tx.Exec(`
				DROP INDEX IF EXISTS ix_monitoring_sites_site_key;
				ALTER TABLE monitoring_sites DROP COLUMN IF EXISTS site_key;
			`).Error
		},
	})
}
