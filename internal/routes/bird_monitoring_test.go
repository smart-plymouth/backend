package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"

	"github.com/smartplymouth/backend/internal/testutil"
)

func TestListBirdSites(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	rows := sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude", "type", "site_key"}).
		AddRow("11111111-1111-1111-1111-111111111111", "Garden A", 50.375, -4.142, "BirdNET-Pi", nil).
		AddRow("22222222-2222-2222-2222-222222222222", "Garden B", 50.380, -4.150, "BirdNET-Pi", nil)

	mock.ExpectQuery(`SELECT .+ FROM "monitoring_sites"`).WillReturnRows(rows)

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("GET", "/api/bird-monitoring/v1.0/sites", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	sites := body["sites"].([]interface{})
	if len(sites) != 2 {
		t.Errorf("expected 2 sites, got %d", len(sites))
	}
}

func TestListBirdSitesEmpty(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "monitoring_sites"`).
		WillReturnRows(sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude", "type", "site_key"}))

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("GET", "/api/bird-monitoring/v1.0/sites", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReceiveWebhookUnauthorized(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "monitoring_sites" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude", "type", "site_key"}))

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	payload := `{"message":"{\"common_name\":\"Robin\",\"confidence\":0.9}"}`
	req := httptest.NewRequest("POST", "/api/bird-monitoring/v1.0/webhook/bad-key", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestReceiveWebhookInvalidJSON(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	siteKey := "test-key"
	mock.ExpectQuery(`SELECT .+ FROM "monitoring_sites" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude", "type", "site_key"}).
			AddRow("11111111-1111-1111-1111-111111111111", "Test", 50.0, -4.0, "BirdNET-Pi", siteKey))

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("POST", "/api/bird-monitoring/v1.0/webhook/test-key", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReceiveWebhookMissingMessage(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "monitoring_sites" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude", "type", "site_key"}).
			AddRow("11111111-1111-1111-1111-111111111111", "Test", 50.0, -4.0, "BirdNET-Pi", "test-key"))

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("POST", "/api/bird-monitoring/v1.0/webhook/test-key", strings.NewReader(`{"title":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReceiveWebhookInvalidMessageJSON(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "monitoring_sites" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude", "type", "site_key"}).
			AddRow("11111111-1111-1111-1111-111111111111", "Test", 50.0, -4.0, "BirdNET-Pi", "test-key"))

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("POST", "/api/bird-monitoring/v1.0/webhook/test-key", strings.NewReader(`{"message":"not-json"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReceiveWebhookMissingCommonName(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "monitoring_sites" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude", "type", "site_key"}).
			AddRow("11111111-1111-1111-1111-111111111111", "Test", 50.0, -4.0, "BirdNET-Pi", "test-key"))

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("POST", "/api/bird-monitoring/v1.0/webhook/test-key",
		strings.NewReader(`{"message":"{\"confidence\":0.9}"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReceiveWebhookSuccess(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	siteID := "11111111-1111-1111-1111-111111111111"
	mock.ExpectQuery(`SELECT .+ FROM "monitoring_sites" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude", "type", "site_key"}).
			AddRow(siteID, "Test Site", 50.0, -4.0, "BirdNET-Pi", "test-key"))

	// Species lookup (not found, so it creates)
	mock.ExpectQuery(`SELECT .+ FROM "species" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"species_id", "common_name", "scientific_name"}))

	// Create species
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "species"`).
		WillReturnRows(sqlmock.NewRows([]string{"species_id"}).AddRow(1))
	mock.ExpectCommit()

	// Create sighting
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "species_sightings"`).
		WillReturnRows(sqlmock.NewRows([]string{"sighting_id"}).AddRow(1))
	mock.ExpectCommit()

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	payload := `{"version":"1.0","title":"BirdNET","message":"{\"common_name\":\"Robin\",\"scientific_name\":\"Erithacus rubecula\",\"confidence\":0.92}","type":"detection"}`
	req := httptest.NewRequest("POST", "/api/bird-monitoring/v1.0/webhook/test-key", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "recorded" {
		t.Errorf("unexpected status: %v", body["status"])
	}
}

func TestListSightingsInvalidSiteID(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("GET", "/api/bird-monitoring/v1.0/sightings?site_id=not-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListSightingsInvalidFromDate(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("GET", "/api/bird-monitoring/v1.0/sightings?from_date=bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListSightingsInvalidToDate(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("GET", "/api/bird-monitoring/v1.0/sightings?to_date=bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListSightingsToBeforeFrom(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	// Count query — GORM executes it before the date validation happens at app level
	// Actually the handler validates dates before counting, so no DB call expected
	_ = mock

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("GET", "/api/bird-monitoring/v1.0/sightings?from_date=2024-07-10&to_date=2024-07-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestListSightingsDateRangeExceeds31Days(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("GET", "/api/bird-monitoring/v1.0/sightings?from_date=2024-01-01&to_date=2024-03-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListSightingsSuccess(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	now := time.Now().UTC()

	// Count
	mock.ExpectQuery(`SELECT count\(\*\) FROM "species_sightings"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Sightings
	mock.ExpectQuery(`SELECT .+ FROM "species_sightings"`).
		WillReturnRows(sqlmock.NewRows([]string{"sighting_id", "site_id", "species_id", "confidence", "datetime"}).
			AddRow(1, "11111111-1111-1111-1111-111111111111", 1, 0.95, now))

	// Species preload
	mock.ExpectQuery(`SELECT .+ FROM "species" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"species_id", "common_name", "scientific_name"}).
			AddRow(1, "Robin", "Erithacus rubecula"))

	r := chi.NewRouter()
	RegisterBirdMonitoring(r, db)

	req := httptest.NewRequest("GET", "/api/bird-monitoring/v1.0/sightings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["total"].(float64) != 1 {
		t.Errorf("expected total=1, got %v", body["total"])
	}
}
