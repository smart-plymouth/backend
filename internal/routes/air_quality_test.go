package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"

	"github.com/smartplymouth/backend/internal/testutil"
)

func TestListAirQualitySites(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	rows := sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude"}).
		AddRow("11111111-1111-1111-1111-111111111111", "Plymouth Centre", 50.371, -4.142).
		AddRow("22222222-2222-2222-2222-222222222222", "Plymouth Tavistock Road", 50.386, -4.141)

	mock.ExpectQuery(`SELECT .+ FROM "airquality_sites"`).
		WillReturnRows(rows)

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/sites", nil)
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

func TestListAirQualitySitesEmpty(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "airquality_sites"`).
		WillReturnRows(sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude"}))

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/sites", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	sites := body["sites"].([]interface{})
	if len(sites) != 0 {
		t.Errorf("expected 0, got %d", len(sites))
	}
}

func TestGetAirQualitySite(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	siteID := "11111111-1111-1111-1111-111111111111"
	rows := sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude"}).
		AddRow(siteID, "Plymouth Centre", 50.371, -4.142)

	mock.ExpectQuery(`SELECT .+ FROM "airquality_sites" WHERE site_id`).WillReturnRows(rows)

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/sites/"+siteID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["name"] != "Plymouth Centre" {
		t.Errorf("unexpected name: %v", body["name"])
	}
}

func TestGetAirQualitySiteNotFound(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	siteID := "11111111-1111-1111-1111-111111111111"
	mock.ExpectQuery(`SELECT .+ FROM "airquality_sites" WHERE site_id`).
		WithArgs(siteID).
		WillReturnRows(sqlmock.NewRows([]string{"site_id", "name", "latitude", "longitude"}))

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/sites/"+siteID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetAirQualitySiteInvalidUUID(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/sites/invalid-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListAirQualityReadingsInvalidSiteID(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/readings?site_id=not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListAirQualityReadingsInvalidFromDate(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/readings?from_date=bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListAirQualityReadingsInvalidToDate(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/readings?to_date=bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListAirQualityReadingsToBeforeFrom(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	// The handler parses dates, then checks the range — it needs to get past the
	// from_date and to_date parsing, but the validation happens before the DB query
	from, _ := time.Parse("2006-01-02", "2024-07-10")
	to, _ := time.Parse("2006-01-02", "2024-07-01")
	_ = from
	_ = to

	// No DB calls expected because validation fails first
	_ = mock

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/readings?from_date=2024-07-10&to_date=2024-07-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestListAirQualityReadingsDateRangeExceeds31Days(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/readings?from_date=2024-01-01&to_date=2024-03-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetAirQualityReadingInvalidID(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/readings/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetAirQualityReadingNotFound(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "airquality_readings" WHERE reading_id`).
		WithArgs(999).
		WillReturnRows(sqlmock.NewRows([]string{"reading_id", "site_id", "datetime"}))

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/readings/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestGetAirQualityReadingSuccess(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	siteID := "11111111-1111-1111-1111-111111111111"
	now := time.Now().UTC()

	mock.ExpectQuery(`SELECT .+ FROM "airquality_readings" WHERE reading_id`).
		WillReturnRows(sqlmock.NewRows([]string{"reading_id", "site_id", "datetime"}).
			AddRow(1, siteID, now))

	mock.ExpectQuery(`SELECT .+ FROM "airquality_metrics" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"metric_id", "reading_id", "pollutant", "value", "unit"}).
			AddRow(1, 1, "NO2", 25.5, "µg/m³"))

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/readings/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestListAirQualityReadingsSuccess(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	siteID := "11111111-1111-1111-1111-111111111111"
	now := time.Now().UTC()

	// Count query
	mock.ExpectQuery(`SELECT count.+ FROM "airquality_readings"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Readings query
	mock.ExpectQuery(`SELECT .+ FROM "airquality_readings"`).
		WillReturnRows(sqlmock.NewRows([]string{"reading_id", "site_id", "datetime"}).
			AddRow(1, siteID, now))

	// Metrics preload
	mock.ExpectQuery(`SELECT .+ FROM "airquality_metrics" WHERE`).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"metric_id", "reading_id", "pollutant", "value", "unit"}).
			AddRow(1, 1, "NO2", 25.5, "µg/m³"))

	r := chi.NewRouter()
	RegisterAirQuality(r, db)

	req := httptest.NewRequest("GET", "/api/air-quality/v1.0/readings", nil)
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
