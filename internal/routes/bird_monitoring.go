package routes

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/smartplymouth/backend/internal/models"
)

func RegisterBirdMonitoring(r *chi.Mux, db *gorm.DB) {
	r.Route("/api/bird-monitoring/v1.0", func(r chi.Router) {
		r.Post("/webhook/{site_key}", receiveWebhook(db))
		r.Get("/sites", listSites(db))
		r.Get("/sightings", listSightings(db))
	})
}

func receiveWebhook(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteKey := chi.URLParam(r, "site_key")

		var site models.MonitoringSite
		if err := db.Where("site_key = ?", siteKey).First(&site).Error; err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}

		var reqBody struct {
			Version     string `json:"version"`
			Title       string `json:"title"`
			Message     string `json:"message"`
			Attachments []any  `json:"attachments"`
			Type        string `json:"type"`
		}

		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Request body must be JSON"})
			return
		}

		if reqBody.Message == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message field is required"})
			return
		}

		var message struct {
			CommonName     string  `json:"common_name"`
			ScientificName string  `json:"scientific_name"`
			Confidence     float64 `json:"confidence"`
		}

		if err := json.Unmarshal([]byte(reqBody.Message), &message); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message field must be a valid JSON string"})
			return
		}

		if message.CommonName == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "common_name and confidence are required"})
			return
		}

		// Find or create species
		var species models.Species
		result := db.Where("common_name = ?", message.CommonName).First(&species)
		if result.Error != nil {
			species = models.Species{
				CommonName: message.CommonName,
			}
			if message.ScientificName != "" {
				species.ScientificName = &message.ScientificName
			}
			db.Create(&species)
		} else if species.ScientificName == nil && message.ScientificName != "" {
			species.ScientificName = &message.ScientificName
			db.Save(&species)
		}

		// Create sighting
		sighting := models.SpeciesSighting{
			SiteID:     site.SiteID,
			SpeciesID:  species.SpeciesID,
			Confidence: message.Confidence,
			Datetime:   time.Now().UTC(),
		}
		db.Create(&sighting)

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"status":      "recorded",
			"sighting_id": sighting.SightingID,
			"species":     species.ToDict(),
		})
	}
}

func listSites(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var sites []models.MonitoringSite
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

func listSightings(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteID := r.URL.Query().Get("site_id")
		fromDateStr := r.URL.Query().Get("from_date")
		toDateStr := r.URL.Query().Get("to_date")
		page := queryInt(r, "page", 1)
		perPage := queryInt(r, "per_page", 25)
		if perPage > 100 {
			perPage = 100
		}

		query := db.Model(&models.SpeciesSighting{}).Preload("SpeciesRel")

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

		var total int64
		query.Count(&total)

		var sightings []models.SpeciesSighting
		offset := (page - 1) * perPage
		query.Order("datetime DESC").Offset(offset).Limit(perPage).Find(&sightings)

		pages := int(math.Ceil(float64(total) / float64(perPage)))

		sightingDicts := make([]map[string]interface{}, 0, len(sightings))
		for _, s := range sightings {
			sightingDicts = append(sightingDicts, s.ToDict())
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"sightings": sightingDicts,
			"total":     total,
			"page":      page,
			"pages":     pages,
			"per_page":  perPage,
		})
	}
}

// Silence unused import warning for fmt
var _ = fmt.Sprintf
