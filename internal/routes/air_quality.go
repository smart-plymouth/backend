package routes

import (
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/smartplymouth/backend/internal/models"
)

func RegisterAirQuality(r *chi.Mux, db *gorm.DB) {
	r.Route("/api/air-quality/v1.0", func(r chi.Router) {
		r.Get("/sites", listAirQualitySites(db))
		r.Get("/sites/{site_id}", getAirQualitySite(db))
		r.Get("/readings", listAirQualityReadings(db))
		r.Get("/readings/{reading_id}", getAirQualityReading(db))
	})
}

func listAirQualitySites(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var sites []models.AirQualitySite
		db.Order("name").Find(&sites)

		siteDicts := make([]map[string]interface{}, 0, len(sites))
		for _, site := range sites {
			siteDicts = append(siteDicts, site.ToDict())
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"sites": siteDicts,
		})
	}
}

func getAirQualitySite(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteIDStr := chi.URLParam(r, "site_id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid site_id format."})
			return
		}

		var site models.AirQualitySite
		if err := db.Where("site_id = ?", siteID).First(&site).Error; err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Site not found."})
			return
		}

		writeJSON(w, http.StatusOK, site.ToDict())
	}
}

func listAirQualityReadings(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteID := r.URL.Query().Get("site_id")
		fromDateStr := r.URL.Query().Get("from_date")
		toDateStr := r.URL.Query().Get("to_date")
		pollutant := r.URL.Query().Get("pollutant")
		page := queryInt(r, "page", 1)
		perPage := queryInt(r, "per_page", 25)
		if perPage > 100 {
			perPage = 100
		}

		query := db.Model(&models.AirQualityReading{}).Preload("Metrics")

		if siteID != "" {
			if _, err := uuid.Parse(siteID); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid site_id format."})
				return
			}
			query = query.Where("site_id = ?", siteID)
		}

		var fromDate, toDate *time.Time

		if fromDateStr != "" {
			parsed, err := time.Parse("2006-01-02", fromDateStr)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid from_date format. Use YYYY-MM-DD."})
				return
			}
			fromDate = &parsed
			query = query.Where("datetime >= ?", parsed)
		}

		if toDateStr != "" {
			parsed, err := time.Parse("2006-01-02", toDateStr)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid to_date format. Use YYYY-MM-DD."})
				return
			}
			toDate = &parsed
			toDatetime := parsed.Add(24 * time.Hour)
			query = query.Where("datetime < ?", toDatetime)
		}

		if fromDate != nil && toDate != nil {
			if toDate.Before(*fromDate) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "to_date must be on or after from_date."})
				return
			}
			if toDate.Sub(*fromDate).Hours()/24 > 31 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Date range must not exceed 31 days."})
				return
			}
		}

		if pollutant != "" {
			query = query.Where("reading_id IN (?)",
				db.Model(&models.AirQualityMetric{}).Select("reading_id").Where("pollutant = ?", pollutant),
			)
		}

		var total int64
		query.Count(&total)

		var readings []models.AirQualityReading
		offset := (page - 1) * perPage
		query.Order("datetime DESC").Offset(offset).Limit(perPage).Find(&readings)

		pages := int(math.Ceil(float64(total) / float64(perPage)))

		readingDicts := make([]map[string]interface{}, 0, len(readings))
		for _, reading := range readings {
			readingDicts = append(readingDicts, reading.ToDict())
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"readings": readingDicts,
			"total":    total,
			"page":     page,
			"pages":    pages,
			"per_page": perPage,
		})
	}
}

func getAirQualityReading(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		readingIDStr := chi.URLParam(r, "reading_id")
		readingID := 0
		for _, c := range readingIDStr {
			if c < '0' || c > '9' {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid reading_id format."})
				return
			}
			readingID = readingID*10 + int(c-'0')
		}

		var reading models.AirQualityReading
		if err := db.Preload("Metrics").Where("reading_id = ?", readingID).First(&reading).Error; err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Reading not found."})
			return
		}

		writeJSON(w, http.StatusOK, reading.ToDict())
	}
}
