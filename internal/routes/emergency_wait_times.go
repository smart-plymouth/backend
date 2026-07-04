package routes

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/smartplymouth/backend/internal/models"
)

func RegisterEmergencyWaitTimes(r *chi.Mux, db *gorm.DB) {
	r.Route("/api/emergency-wait-times/v1.0", func(r chi.Router) {
		r.Get("/locations", listLocations(db))
		r.Get("/locations/{location_id}", getLocation(db))
		r.Get("/locations/{location_id}/wait-times", listWaitTimes(db))
	})
}

func listLocations(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var locations []models.Location
		db.Find(&locations)

		result := make([]map[string]interface{}, 0, len(locations))
		for _, loc := range locations {
			result = append(result, locationToDict(&loc))
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func getLocation(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "location_id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Location not found"})
			return
		}

		var location models.Location
		if err := db.Where("id = ?", id).First(&location).Error; err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Location not found"})
			return
		}

		writeJSON(w, http.StatusOK, locationToDict(&location))
	}
}

func listWaitTimes(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "location_id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Location not found"})
			return
		}

		// Verify location exists
		var location models.Location
		if err := db.Where("id = ?", id).First(&location).Error; err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Location not found"})
			return
		}

		query := db.Where("location_id = ?", id)

		startStr := r.URL.Query().Get("start")
		endStr := r.URL.Query().Get("end")

		var startDT, endDT time.Time
		var hasStart, hasEnd bool

		if startStr != "" {
			parsed, err := time.Parse(time.RFC3339, startStr)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid start date format. Use ISO 8601."})
				return
			}
			startDT = parsed
			hasStart = true
			query = query.Where("timestamp >= ?", startDT)
		}

		if endStr != "" {
			parsed, err := time.Parse(time.RFC3339, endStr)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid end date format. Use ISO 8601."})
				return
			}
			endDT = parsed
			hasEnd = true
			query = query.Where("timestamp <= ?", endDT)
		}

		if hasStart && hasEnd {
			if endDT.Sub(startDT) > 31*24*time.Hour {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Date range must not exceed 31 days."})
				return
			}
		}

		var waitTimes []models.WaitTime
		query.Order("timestamp DESC").Find(&waitTimes)

		result := make([]map[string]interface{}, 0, len(waitTimes))
		for _, wt := range waitTimes {
			result = append(result, map[string]interface{}{
				"location_id":           wt.LocationID.String(),
				"timestamp":             wt.Timestamp.Format(time.RFC3339),
				"longest_wait":          wt.LongestWait,
				"patients_waiting":      wt.PatientsWaiting,
				"patients_in_department": wt.PatientsInDepartment,
			})
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func locationToDict(loc *models.Location) map[string]interface{} {
	return map[string]interface{}{
		"id":               loc.ID.String(),
		"name":             loc.Name,
		"type":             loc.Type,
		"address":          loc.Address,
		"longitude":        loc.Longitude,
		"latitude":         loc.Latitude,
		"opening_times":    loc.OpeningTimes,
		"telephone_number": loc.TelephoneNumber,
	}
}
