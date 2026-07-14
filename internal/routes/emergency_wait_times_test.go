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

func TestListLocations(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	rows := sqlmock.NewRows([]string{"id", "name", "type", "address", "longitude", "latitude", "opening_times", "telephone_number"}).
		AddRow("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Emergency Department", "ED", "Derriford", -4.113, 50.417, nil, nil).
		AddRow("bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "UTC Cumberland", "UTC", "Cumberland", -4.168, 50.370, nil, nil)

	mock.ExpectQuery(`SELECT .+ FROM "ed_wait_times_locations"`).WillReturnRows(rows)

	r := chi.NewRouter()
	RegisterEmergencyWaitTimes(r, db)

	req := httptest.NewRequest("GET", "/api/emergency-wait-times/v1.0/locations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if len(body) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(body))
	}
}

func TestListLocationsEmpty(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "ed_wait_times_locations"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "address", "longitude", "latitude", "opening_times", "telephone_number"}))

	r := chi.NewRouter()
	RegisterEmergencyWaitTimes(r, db)

	req := httptest.NewRequest("GET", "/api/emergency-wait-times/v1.0/locations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if len(body) != 0 {
		t.Errorf("expected 0, got %d", len(body))
	}
}

func TestGetLocation(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	rows := sqlmock.NewRows([]string{"id", "name", "type", "address", "longitude", "latitude", "opening_times", "telephone_number"}).
		AddRow("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Emergency Department", "ED", "Derriford", -4.113, 50.417, nil, nil)

	mock.ExpectQuery(`SELECT .+ FROM "ed_wait_times_locations" WHERE`).WillReturnRows(rows)

	r := chi.NewRouter()
	RegisterEmergencyWaitTimes(r, db)

	req := httptest.NewRequest("GET", "/api/emergency-wait-times/v1.0/locations/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["name"] != "Emergency Department" {
		t.Errorf("unexpected name: %v", body["name"])
	}
}

func TestGetLocationNotFound(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "ed_wait_times_locations" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "address", "longitude", "latitude", "opening_times", "telephone_number"}))

	r := chi.NewRouter()
	RegisterEmergencyWaitTimes(r, db)

	req := httptest.NewRequest("GET", "/api/emergency-wait-times/v1.0/locations/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetLocationInvalidUUID(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterEmergencyWaitTimes(r, db)

	req := httptest.NewRequest("GET", "/api/emergency-wait-times/v1.0/locations/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListWaitTimes(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	locID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	locRows := sqlmock.NewRows([]string{"id", "name", "type", "address", "longitude", "latitude", "opening_times", "telephone_number"}).
		AddRow(locID, "Emergency Department", "ED", "Derriford", -4.113, 50.417, nil, nil)
	mock.ExpectQuery(`SELECT .+ FROM "ed_wait_times_locations" WHERE`).WillReturnRows(locRows)

	now := time.Now().UTC()
	wtRows := sqlmock.NewRows([]string{"location_id", "timestamp", "longest_wait", "patients_waiting", "patients_in_department"}).
		AddRow(locID, now, 45, 12, 35).
		AddRow(locID, now.Add(-5*time.Minute), 40, 10, 30)
	mock.ExpectQuery(`SELECT .+ FROM "ed_wait_times_wait_times" WHERE`).WillReturnRows(wtRows)

	r := chi.NewRouter()
	RegisterEmergencyWaitTimes(r, db)

	req := httptest.NewRequest("GET", "/api/emergency-wait-times/v1.0/locations/"+locID+"/wait-times", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if len(body) != 2 {
		t.Fatalf("expected 2 wait times, got %d", len(body))
	}
}

func TestListWaitTimesLocationNotFound(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "ed_wait_times_locations" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "address", "longitude", "latitude", "opening_times", "telephone_number"}))

	r := chi.NewRouter()
	RegisterEmergencyWaitTimes(r, db)

	req := httptest.NewRequest("GET", "/api/emergency-wait-times/v1.0/locations/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/wait-times", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListWaitTimesInvalidStartDate(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	locID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	mock.ExpectQuery(`SELECT .+ FROM "ed_wait_times_locations" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "address", "longitude", "latitude", "opening_times", "telephone_number"}).
			AddRow(locID, "ED", "ED", "Addr", -4.0, 50.0, nil, nil))

	r := chi.NewRouter()
	RegisterEmergencyWaitTimes(r, db)

	req := httptest.NewRequest("GET", "/api/emergency-wait-times/v1.0/locations/"+locID+"/wait-times?start=bad-date", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListWaitTimesInvalidEndDate(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	locID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	mock.ExpectQuery(`SELECT .+ FROM "ed_wait_times_locations" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "address", "longitude", "latitude", "opening_times", "telephone_number"}).
			AddRow(locID, "ED", "ED", "Addr", -4.0, 50.0, nil, nil))

	r := chi.NewRouter()
	RegisterEmergencyWaitTimes(r, db)

	start := time.Now().Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/emergency-wait-times/v1.0/locations/"+locID+"/wait-times?start="+start+"&end=bad-date", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListWaitTimesDateRangeExceeds31Days(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	locID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	mock.ExpectQuery(`SELECT .+ FROM "ed_wait_times_locations" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "address", "longitude", "latitude", "opening_times", "telephone_number"}).
			AddRow(locID, "ED", "ED", "Addr", -4.0, 50.0, nil, nil))

	r := chi.NewRouter()
	RegisterEmergencyWaitTimes(r, db)

	start := time.Now().Add(-60 * 24 * time.Hour).Format(time.RFC3339)
	end := time.Now().Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/emergency-wait-times/v1.0/locations/"+locID+"/wait-times?start="+start+"&end="+end, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
