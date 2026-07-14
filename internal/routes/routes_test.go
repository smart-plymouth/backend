package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/smartplymouth/backend/internal/models"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]interface{}{
		"message": "hello",
		"count":   42,
	}

	writeJSON(w, http.StatusOK, data)

	result := w.Result()
	if result.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}

	contentType := result.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", contentType)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(result.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["message"] != "hello" {
		t.Errorf("unexpected message: %v", body["message"])
	}
	if body["count"] != float64(42) {
		t.Errorf("unexpected count: %v", body["count"])
	}
}

func TestWriteJSONStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"OK", http.StatusOK},
		{"Created", http.StatusCreated},
		{"Bad Request", http.StatusBadRequest},
		{"Not Found", http.StatusNotFound},
		{"Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeJSON(w, tt.status, map[string]string{"status": "test"})

			if w.Code != tt.status {
				t.Errorf("expected status %d, got %d", tt.status, w.Code)
			}
		})
	}
}

func TestQueryInt(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		key        string
		defaultVal int
		want       int
	}{
		{
			name:       "value present",
			query:      "page=3",
			key:        "page",
			defaultVal: 1,
			want:       3,
		},
		{
			name:       "value missing returns default",
			query:      "",
			key:        "page",
			defaultVal: 1,
			want:       1,
		},
		{
			name:       "invalid value returns default",
			query:      "page=abc",
			key:        "page",
			defaultVal: 1,
			want:       1,
		},
		{
			name:       "zero value returns default",
			query:      "page=0",
			key:        "page",
			defaultVal: 1,
			want:       1,
		},
		{
			name:       "negative value returns default",
			query:      "page=-5",
			key:        "page",
			defaultVal: 1,
			want:       1,
		},
		{
			name:       "large value",
			query:      "per_page=100",
			key:        "per_page",
			defaultVal: 25,
			want:       100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			got := queryInt(req, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("queryInt(key=%q, default=%d) = %d, want %d",
					tt.key, tt.defaultVal, got, tt.want)
			}
		})
	}
}

func TestRegisterTestAPI(t *testing.T) {
	r := chi.NewRouter()
	RegisterTestAPI(r, nil) // db not used in test-api

	req := httptest.NewRequest("GET", "/api/test-api/v1.0/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["message"] != "Hello, World!" {
		t.Errorf("unexpected message: %v", body["message"])
	}
	if body["service"] != "test-api" {
		t.Errorf("unexpected service: %v", body["service"])
	}
	if body["version"] != "1.0" {
		t.Errorf("unexpected version: %v", body["version"])
	}
}

func TestLocationToDict(t *testing.T) {
	phone := "01234 567890"
	hours := "Mon-Fri 9-5"

	loc := &models.Location{
		ID:              uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"),
		Name:            "Test Hospital",
		Type:            "ED",
		Address:         "123 Hospital Road",
		Longitude:       -4.1137,
		Latitude:        50.4167,
		OpeningTimes:    &hours,
		TelephoneNumber: &phone,
	}

	dict := locationToDict(loc)

	if dict["id"] != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Errorf("unexpected id: %v", dict["id"])
	}
	if dict["name"] != "Test Hospital" {
		t.Errorf("unexpected name: %v", dict["name"])
	}
	if dict["type"] != "ED" {
		t.Errorf("unexpected type: %v", dict["type"])
	}
	if dict["longitude"] != -4.1137 {
		t.Errorf("unexpected longitude: %v", dict["longitude"])
	}
	if dict["latitude"] != 50.4167 {
		t.Errorf("unexpected latitude: %v", dict["latitude"])
	}
	if *(dict["opening_times"].(*string)) != "Mon-Fri 9-5" {
		t.Errorf("unexpected opening_times: %v", dict["opening_times"])
	}
	if *(dict["telephone_number"].(*string)) != "01234 567890" {
		t.Errorf("unexpected telephone_number: %v", dict["telephone_number"])
	}
}
