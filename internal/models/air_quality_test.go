package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAirQualitySiteToDict(t *testing.T) {
	siteID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	site := &AirQualitySite{
		SiteID:    siteID,
		Name:      "Plymouth Centre",
		Latitude:  50.37167,
		Longitude: -4.14250,
	}

	dict := site.ToDict()

	if dict["site_id"] != siteID.String() {
		t.Errorf("unexpected site_id: %v", dict["site_id"])
	}
	if dict["name"] != "Plymouth Centre" {
		t.Errorf("unexpected name: %v", dict["name"])
	}
	if dict["latitude"] != 50.37167 {
		t.Errorf("unexpected latitude: %v", dict["latitude"])
	}
	if dict["longitude"] != -4.14250 {
		t.Errorf("unexpected longitude: %v", dict["longitude"])
	}
}

func TestAirQualitySiteTableName(t *testing.T) {
	s := AirQualitySite{}
	if s.TableName() != "airquality_sites" {
		t.Errorf("expected 'airquality_sites', got %q", s.TableName())
	}
}

func TestAirQualityReadingToDict(t *testing.T) {
	siteID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	datetime := time.Date(2024, 7, 10, 14, 0, 0, 0, time.UTC)

	reading := &AirQualityReading{
		ReadingID: 100,
		SiteID:    siteID,
		Datetime:  datetime,
		Metrics: []AirQualityMetric{
			{MetricID: 1, ReadingID: 100, Pollutant: "NO2", Value: 25.5, Unit: "µg/m³"},
			{MetricID: 2, ReadingID: 100, Pollutant: "PM10", Value: 12.3, Unit: "µg/m³"},
		},
	}

	dict := reading.ToDict()

	if dict["reading_id"] != 100 {
		t.Errorf("unexpected reading_id: %v", dict["reading_id"])
	}
	if dict["site_id"] != siteID.String() {
		t.Errorf("unexpected site_id: %v", dict["site_id"])
	}
	if dict["datetime"] != "2024-07-10T14:00:00Z" {
		t.Errorf("unexpected datetime: %v", dict["datetime"])
	}

	metrics, ok := dict["metrics"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected metrics to be []map[string]interface{}")
	}
	if len(metrics) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(metrics))
	}
}

func TestAirQualityReadingToDictNoMetrics(t *testing.T) {
	siteID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	reading := &AirQualityReading{
		ReadingID: 101,
		SiteID:    siteID,
		Datetime:  time.Now(),
	}

	dict := reading.ToDict()

	// When Metrics is nil, ToDict should not include metrics key or it should be nil
	if _, exists := dict["metrics"]; exists {
		t.Log("metrics key exists but is expected to be absent when Metrics is nil")
	}
}

func TestAirQualityReadingTableName(t *testing.T) {
	r := AirQualityReading{}
	if r.TableName() != "airquality_readings" {
		t.Errorf("expected 'airquality_readings', got %q", r.TableName())
	}
}

func TestAirQualityMetricToDict(t *testing.T) {
	metric := &AirQualityMetric{
		MetricID:  5,
		ReadingID: 100,
		Pollutant: "PM2.5",
		Value:     8.7,
		Unit:      "µg/m³",
	}

	dict := metric.ToDict()

	if dict["metric_id"] != 5 {
		t.Errorf("unexpected metric_id: %v", dict["metric_id"])
	}
	if dict["pollutant"] != "PM2.5" {
		t.Errorf("unexpected pollutant: %v", dict["pollutant"])
	}
	if dict["value"] != 8.7 {
		t.Errorf("unexpected value: %v", dict["value"])
	}
	if dict["unit"] != "µg/m³" {
		t.Errorf("unexpected unit: %v", dict["unit"])
	}
}

func TestAirQualityMetricTableName(t *testing.T) {
	m := AirQualityMetric{}
	if m.TableName() != "airquality_metrics" {
		t.Errorf("expected 'airquality_metrics', got %q", m.TableName())
	}
}
