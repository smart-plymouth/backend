package models

import (
	"time"

	"github.com/google/uuid"
)

type AirQualitySite struct {
	SiteID    uuid.UUID `gorm:"column:site_id;type:uuid;primaryKey" json:"site_id"`
	Name      string    `gorm:"column:name;type:varchar(255);not null" json:"name"`
	Latitude  float64   `gorm:"column:latitude;not null" json:"latitude"`
	Longitude float64   `gorm:"column:longitude;not null" json:"longitude"`
}

func (AirQualitySite) TableName() string {
	return "airquality_sites"
}

func (s *AirQualitySite) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"site_id":   s.SiteID.String(),
		"name":      s.Name,
		"latitude":  s.Latitude,
		"longitude": s.Longitude,
	}
}

type AirQualityReading struct {
	ReadingID int       `gorm:"column:reading_id;primaryKey;autoIncrement" json:"reading_id"`
	SiteID    uuid.UUID `gorm:"column:site_id;type:uuid;not null;index" json:"site_id"`
	Datetime  time.Time `gorm:"column:datetime;type:timestamptz;not null;index" json:"datetime"`

	Site    *AirQualitySite     `gorm:"foreignKey:SiteID;references:SiteID" json:"-"`
	Metrics []AirQualityMetric  `gorm:"foreignKey:ReadingID;references:ReadingID" json:"-"`
}

func (AirQualityReading) TableName() string {
	return "airquality_readings"
}

func (r *AirQualityReading) ToDict() map[string]interface{} {
	result := map[string]interface{}{
		"reading_id": r.ReadingID,
		"site_id":    r.SiteID.String(),
		"datetime":   r.Datetime.Format(time.RFC3339),
	}

	if r.Metrics != nil {
		metrics := make([]map[string]interface{}, 0, len(r.Metrics))
		for _, m := range r.Metrics {
			metrics = append(metrics, m.ToDict())
		}
		result["metrics"] = metrics
	}

	return result
}

type AirQualityMetric struct {
	MetricID  int    `gorm:"column:metric_id;primaryKey;autoIncrement" json:"metric_id"`
	ReadingID int    `gorm:"column:reading_id;not null;index" json:"reading_id"`
	Pollutant string `gorm:"column:pollutant;type:varchar(50);not null" json:"pollutant"`
	Value     float64 `gorm:"column:value;not null" json:"value"`
	Unit      string `gorm:"column:unit;type:varchar(50);not null" json:"unit"`
}

func (AirQualityMetric) TableName() string {
	return "airquality_metrics"
}

func (m *AirQualityMetric) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"metric_id": m.MetricID,
		"pollutant": m.Pollutant,
		"value":     m.Value,
		"unit":      m.Unit,
	}
}
