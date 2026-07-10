package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/smartplymouth/backend/internal/models"
)

// DEFRA UK-AIR data-plot URL pattern.
const defraDataPlotURL = "https://uk-air.defra.gov.uk/data-plot?site_id=%s&days=%d"

// defraSite represents a DEFRA monitoring station to scrape.
type defraSite struct {
	DefraID   string
	Name      string
	Latitude  float64
	Longitude float64
}

var defraSites = []defraSite{
	{DefraID: "PLYM", Name: "Plymouth Centre", Latitude: 50.37167, Longitude: -4.14250},
	{DefraID: "PLYR", Name: "Plymouth Tavistock Road", Latitude: 50.38600, Longitude: -4.14100},
}

// highchartsSeriesAQ matches the relevant fields from the Highcharts config on DEFRA pages.
type highchartsSeriesAQ struct {
	Name          string          `json:"name"`
	PointStart    float64         `json:"pointStart"`
	PointInterval float64         `json:"pointInterval"`
	Data          json.RawMessage `json:"data"`
}

type chartConfigAQ struct {
	Series []highchartsSeriesAQ `json:"series"`
}

type aqDataPoint struct {
	TimestampMS int64
	Value       float64
}

// aqHourlyReading holds parsed data for a single hour at a single site.
type aqHourlyReading struct {
	Time   time.Time
	Values map[string]float64 // pollutant → concentration (µg/m³)
}

func NewFetchAirQualityHandler(db *gorm.DB) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		log.Println("Starting air quality data fetch")

		for _, site := range defraSites {
			if err := fetchAndStoreAirQuality(db, site); err != nil {
				log.Printf("Error fetching air quality for %s: %v", site.Name, err)
				continue
			}
		}

		log.Println("Air quality fetch complete")
		return nil
	}
}

func fetchAndStoreAirQuality(db *gorm.DB, site defraSite) error {
	// Ensure the site exists in the database
	dbSite, err := ensureAirQualitySite(db, site)
	if err != nil {
		return fmt.Errorf("ensuring site %s: %w", site.Name, err)
	}

	// Fetch last 2 days of data (covers the latest hour even with delays)
	readings, err := fetchDefraHourlyData(site, 2)
	if err != nil {
		return fmt.Errorf("fetching data for %s: %w", site.Name, err)
	}

	if len(readings) == 0 {
		log.Printf("No readings returned for %s", site.Name)
		return nil
	}

	// Insert readings that don't already exist
	inserted := 0
	for _, reading := range readings {
		if len(reading.Values) == 0 {
			continue
		}

		// Check if a reading already exists for this site+datetime
		var count int64
		db.Model(&models.AirQualityReading{}).
			Where("site_id = ? AND datetime = ?", dbSite.SiteID, reading.Time).
			Count(&count)
		if count > 0 {
			continue
		}

		// Create the reading
		dbReading := models.AirQualityReading{
			SiteID:   dbSite.SiteID,
			Datetime: reading.Time,
		}
		if err := db.Create(&dbReading).Error; err != nil {
			log.Printf("Failed to insert reading for %s at %s: %v", site.Name, reading.Time, err)
			continue
		}

		// Create metrics for this reading
		for pollutant, value := range reading.Values {
			metric := models.AirQualityMetric{
				ReadingID: dbReading.ReadingID,
				Pollutant: pollutant,
				Value:     value,
				Unit:      "µg/m³",
			}
			if err := db.Create(&metric).Error; err != nil {
				log.Printf("Failed to insert metric %s for reading %d: %v", pollutant, dbReading.ReadingID, err)
			}
		}

		inserted++
	}

	log.Printf("Inserted %d new readings for %s (%s)", inserted, site.Name, site.DefraID)
	return nil
}

func ensureAirQualitySite(db *gorm.DB, site defraSite) (*models.AirQualitySite, error) {
	var dbSite models.AirQualitySite
	err := db.Where("name = ?", site.Name).First(&dbSite).Error
	if err == nil {
		return &dbSite, nil
	}

	// Create the site
	dbSite = models.AirQualitySite{
		SiteID:    uuid.New(),
		Name:      site.Name,
		Latitude:  site.Latitude,
		Longitude: site.Longitude,
	}
	if err := db.Create(&dbSite).Error; err != nil {
		return nil, err
	}

	log.Printf("Created air quality site: %s (%s)", site.Name, dbSite.SiteID)
	return &dbSite, nil
}

func fetchDefraHourlyData(site defraSite, days int) ([]aqHourlyReading, error) {
	url := fmt.Sprintf(defraDataPlotURL, site.DefraID, days)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	html := string(body)

	// Extract Highcharts JSON: Highcharts.chart('...', {...});
	re := regexp.MustCompile(`Highcharts\.chart\('[^']+',\s*(\{.*?\})\);`)
	match := re.FindStringSubmatch(html)

	var config chartConfigAQ
	if match != nil {
		if err := json.Unmarshal([]byte(match[1]), &config); err != nil {
			// Fallback: extract series array directly
			config.Series = extractSeriesFallback(html)
		}
	} else {
		config.Series = extractSeriesFallback(html)
	}

	if len(config.Series) == 0 {
		return nil, fmt.Errorf("no series data found")
	}

	// Parse series into readings indexed by timestamp
	london, _ := time.LoadLocation("Europe/London")
	timeMap := make(map[int64]map[string]float64)

	for _, s := range config.Series {
		if s.Name == "" {
			continue
		}

		points, err := parseAQDataPoints(s)
		if err != nil {
			continue
		}

		for _, pt := range points {
			if math.IsNaN(pt.Value) {
				continue
			}
			if _, ok := timeMap[pt.TimestampMS]; !ok {
				timeMap[pt.TimestampMS] = make(map[string]float64)
			}
			timeMap[pt.TimestampMS][s.Name] = pt.Value
		}
	}

	// Convert to readings
	var readings []aqHourlyReading
	for tsMS, values := range timeMap {
		if len(values) == 0 {
			continue
		}
		t := time.UnixMilli(tsMS).In(london)
		readings = append(readings, aqHourlyReading{
			Time:   t,
			Values: values,
		})
	}

	sort.Slice(readings, func(i, j int) bool {
		return readings[i].Time.Before(readings[j].Time)
	})

	return readings, nil
}

func extractSeriesFallback(html string) []highchartsSeriesAQ {
	seriesRe := regexp.MustCompile(`"series"\s*:\s*(\[.*?\])\s*,\s*"legend"`)
	seriesMatch := seriesRe.FindStringSubmatch(html)
	if seriesMatch == nil {
		return nil
	}
	var series []highchartsSeriesAQ
	if err := json.Unmarshal([]byte(seriesMatch[1]), &series); err != nil {
		return nil
	}
	return series
}

func parseAQDataPoints(s highchartsSeriesAQ) ([]aqDataPoint, error) {
	var points []aqDataPoint

	// Try array of pairs first: [[timestamp_ms, value], ...]
	var pairs [][]json.Number
	if err := json.Unmarshal(s.Data, &pairs); err == nil && len(pairs) > 0 {
		if len(pairs[0]) == 2 {
			for _, pair := range pairs {
				ts, err1 := pair[0].Int64()
				val, err2 := pair[1].Float64()
				if err1 != nil || err2 != nil {
					continue
				}
				points = append(points, aqDataPoint{TimestampMS: ts, Value: val})
			}
			return points, nil
		}
	}

	// Try plain array using pointStart + pointInterval
	var rawValues []*float64
	if err := json.Unmarshal(s.Data, &rawValues); err == nil {
		interval := int64(s.PointInterval)
		if interval == 0 {
			interval = 3600000 // default 1 hour
		}
		start := int64(s.PointStart)
		for i, v := range rawValues {
			if v == nil {
				continue
			}
			ts := start + int64(i)*interval
			points = append(points, aqDataPoint{TimestampMS: ts, Value: *v})
		}
		return points, nil
	}

	// Try mixed array where each element is [ts, val] or null
	var raw []json.RawMessage
	if err := json.Unmarshal(s.Data, &raw); err != nil {
		return nil, fmt.Errorf("cannot parse data array")
	}

	interval := int64(s.PointInterval)
	if interval == 0 {
		interval = 3600000
	}
	start := int64(s.PointStart)

	for i, elem := range raw {
		elemStr := strings.TrimSpace(string(elem))
		if elemStr == "null" {
			continue
		}
		// Try as [ts, val]
		var pair [2]json.Number
		if err := json.Unmarshal(elem, &pair); err == nil {
			ts, e1 := pair[0].Int64()
			val, e2 := pair[1].Float64()
			if e1 == nil && e2 == nil {
				points = append(points, aqDataPoint{TimestampMS: ts, Value: val})
				continue
			}
		}
		// Try as plain number
		val, err := strconv.ParseFloat(elemStr, 64)
		if err == nil {
			ts := start + int64(i)*interval
			points = append(points, aqDataPoint{TimestampMS: ts, Value: val})
		}
	}

	return points, nil
}
