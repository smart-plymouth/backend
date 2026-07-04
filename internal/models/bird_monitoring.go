package models

import (
	"time"

	"github.com/google/uuid"
)

type MonitoringSite struct {
	SiteID    uuid.UUID `gorm:"column:site_id;type:uuid;primaryKey" json:"site_id"`
	Name      string    `gorm:"column:name;type:varchar(255);not null" json:"name"`
	Latitude  float64   `gorm:"column:latitude;not null" json:"latitude"`
	Longitude float64   `gorm:"column:longitude;not null" json:"longitude"`
	Type      string    `gorm:"column:type;type:varchar(50);not null;default:BirdNET-Pi" json:"type"`
	SiteKey   *string   `gorm:"column:site_key;type:varchar(255);uniqueIndex" json:"-"`
}

func (MonitoringSite) TableName() string {
	return "monitoring_sites"
}

func (s *MonitoringSite) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"site_id":   s.SiteID.String(),
		"name":      s.Name,
		"latitude":  s.Latitude,
		"longitude": s.Longitude,
		"type":      s.Type,
	}
}

type Species struct {
	SpeciesID      int     `gorm:"column:species_id;primaryKey;autoIncrement" json:"species_id"`
	CommonName     string  `gorm:"column:common_name;type:varchar(255);not null;uniqueIndex" json:"common_name"`
	ScientificName *string `gorm:"column:scientific_name;type:varchar(255)" json:"scientific_name"`
}

func (Species) TableName() string {
	return "species"
}

func (s *Species) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"species_id":      s.SpeciesID,
		"common_name":     s.CommonName,
		"scientific_name": s.ScientificName,
	}
}

type SpeciesSighting struct {
	SightingID int       `gorm:"column:sighting_id;primaryKey;autoIncrement" json:"sighting_id"`
	SiteID     uuid.UUID `gorm:"column:site_id;type:uuid;not null;index" json:"site_id"`
	SpeciesID  int       `gorm:"column:species_id;not null;index" json:"species_id"`
	Confidence float64   `gorm:"column:confidence;not null" json:"confidence"`
	Datetime   time.Time `gorm:"column:datetime;type:timestamptz;not null;index;default:now()" json:"datetime"`

	Site       *MonitoringSite `gorm:"foreignKey:SiteID;references:SiteID" json:"-"`
	SpeciesRel *Species        `gorm:"foreignKey:SpeciesID;references:SpeciesID" json:"-"`
}

func (SpeciesSighting) TableName() string {
	return "species_sightings"
}

func (s *SpeciesSighting) ToDict() map[string]interface{} {
	result := map[string]interface{}{
		"sighting_id": s.SightingID,
		"site_id":     s.SiteID.String(),
		"confidence":  s.Confidence,
		"datetime":    s.Datetime.Format(time.RFC3339),
	}
	if s.SpeciesRel != nil {
		result["species"] = s.SpeciesRel.ToDict()
	} else {
		result["species"] = nil
	}
	return result
}
