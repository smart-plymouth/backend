package routes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

func RegisterTestAPI(r *chi.Mux, db *gorm.DB) {
	r.Route("/api/test-api/v1.0", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"message": "Hello, World!",
				"service": "test-api",
				"version": "1.0",
			})
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
