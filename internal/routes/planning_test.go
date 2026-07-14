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

func TestListCases(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "planning_cases"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"reference", "address", "proposal", "status", "received_date", "validated_date",
		"ai_analysis", "potential_impact_score", "tags", "estimated_size",
		"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
	}).
		AddRow("24/00001/FUL", "1 High Street", "Extension", "Pending", nil, nil, false, nil, nil, nil, nil, nil, nil, now, now).
		AddRow("24/00002/FUL", "2 Low Road", "New Build", "Approved", nil, nil, false, nil, nil, nil, nil, nil, nil, now, now)

	mock.ExpectQuery(`SELECT .+ FROM "planning_cases"`).WillReturnRows(rows)

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["total"].(float64) != 2 {
		t.Errorf("expected total=2, got %v", body["total"])
	}
}

func TestListCasesEmpty(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "planning_cases"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT .+ FROM "planning_cases"`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["total"].(float64) != 0 {
		t.Errorf("expected total=0, got %v", body["total"])
	}
}

func TestListCasesInvalidValidatedDate(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases?validated_date=bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListCasesInvalidValidatedFrom(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases?validated_from=bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListCasesInvalidValidatedTo(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases?validated_to=bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListCasesPerPageCap(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "planning_cases"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ FROM "planning_cases"`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases?per_page=200", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["per_page"].(float64) != 100 {
		t.Errorf("expected per_page capped to 100, got %v", body["per_page"])
	}
}

func TestGetCase(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	now := time.Now()
	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}).AddRow("24/00001/FUL", "1 High Street", "Extension", "Pending", nil, nil, false, nil, nil, nil, nil, nil, nil, now, now))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases/24/00001/FUL", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["reference"] != "24/00001/FUL" {
		t.Errorf("unexpected reference: %v", body["reference"])
	}
}

func TestGetCaseNotFound(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases/99/99999/FUL", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListObjections(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	now := time.Now()
	// Verify case exists
	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}).AddRow("24/00001/FUL", "1 S", "X", "P", nil, nil, false, nil, nil, nil, nil, nil, nil, now, now))

	// Objections
	mock.ExpectQuery(`SELECT .+ FROM "planning_objections" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "case_reference", "objection", "ai_rationalisation", "created_at"}).
			AddRow(1, "24/00001/FUL", "Traffic", "Reason1", now).
			AddRow(2, "24/00001/FUL", "Noise", "Reason2", now))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases/24/00001/FUL/objections", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	objs := body["objections"].([]interface{})
	if len(objs) != 2 {
		t.Errorf("expected 2 objections, got %d", len(objs))
	}
}

func TestListObjectionsCaseNotFound(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases/99/99999/FUL/objections", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListSupports(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	now := time.Now()
	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}).AddRow("24/00001/FUL", "1 S", "X", "P", nil, nil, false, nil, nil, nil, nil, nil, nil, now, now))

	mock.ExpectQuery(`SELECT .+ FROM "planning_supports" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "case_reference", "support_reason", "ai_rationalisation", "created_at"}).
			AddRow(1, "24/00001/FUL", "Housing need", "Good", now))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases/24/00001/FUL/supports", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	sups := body["supports"].([]interface{})
	if len(sups) != 1 {
		t.Errorf("expected 1 support, got %d", len(sups))
	}
}

func TestListSupportsCaseNotFound(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("GET", "/api/planning/v1.0/cases/99/99999/FUL/supports", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSubmitPhasetenEmailInvalidBody(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("POST", "/api/planning/v1.0/phaseten_email",
		strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitPhasetenEmailMissing(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("POST", "/api/planning/v1.0/phaseten_email",
		strings.NewReader(`{"email":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitPhasetenEmailInvalidFormat(t *testing.T) {
	db, _ := testutil.NewMockDB(t)

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("POST", "/api/planning/v1.0/phaseten_email",
		strings.NewReader(`{"email":"not-an-email"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitPhasetenEmailSuccess(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "planning_phaseten_emails"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("POST", "/api/planning/v1.0/phaseten_email",
		strings.NewReader(`{"email":"test@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "registered" {
		t.Errorf("unexpected status: %v", body["status"])
	}
}

func TestTriggerAnalysisCaseNotFound(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("POST", "/api/planning/v1.0/cases/99/99999/FUL/analyse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestGenerateLetterCaseNotFound(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	body := `{"first_name":"John","last_name":"Doe","letter_type":"objection"}`
	req := httptest.NewRequest("POST", "/api/planning/v1.0/cases/99/99999/FUL/generate-letter",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestGenerateLetterInvalidBody(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	now := time.Now()
	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}).AddRow("24/00001/FUL", "1 S", "X", "P", nil, nil, false, nil, nil, nil, nil, nil, nil, now, now))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	req := httptest.NewRequest("POST", "/api/planning/v1.0/cases/24/00001/FUL/generate-letter",
		strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestGenerateLetterMissingNames(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	now := time.Now()
	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}).AddRow("24/00001/FUL", "1 S", "X", "P", nil, nil, false, nil, nil, nil, nil, nil, nil, now, now))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	body := `{"first_name":"","last_name":"","letter_type":"objection"}`
	req := httptest.NewRequest("POST", "/api/planning/v1.0/cases/24/00001/FUL/generate-letter",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGenerateLetterInvalidLetterType(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	now := time.Now()
	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}).AddRow("24/00001/FUL", "1 S", "X", "P", nil, nil, false, nil, nil, nil, nil, nil, nil, now, now))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	body := `{"first_name":"John","last_name":"Doe","letter_type":"complaint"}`
	req := httptest.NewRequest("POST", "/api/planning/v1.0/cases/24/00001/FUL/generate-letter",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGenerateLetterNoObjections(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	now := time.Now()
	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}).AddRow("24/00001/FUL", "1 S", "X", "P", nil, nil, false, nil, nil, nil, nil, nil, nil, now, now))

	// No objections found
	mock.ExpectQuery(`SELECT .+ FROM "planning_objections" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "case_reference", "objection", "ai_rationalisation", "created_at"}))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	body := `{"first_name":"John","last_name":"Doe","letter_type":"objection"}`
	req := httptest.NewRequest("POST", "/api/planning/v1.0/cases/24/00001/FUL/generate-letter",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestGenerateLetterNoSupports(t *testing.T) {
	db, mock := testutil.NewMockDB(t)

	now := time.Now()
	mock.ExpectQuery(`SELECT .+ FROM "planning_cases" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"reference", "address", "proposal", "status", "received_date", "validated_date",
			"ai_analysis", "potential_impact_score", "tags", "estimated_size",
			"ai_rationalisation", "pros", "cons", "created_at", "updated_at",
		}).AddRow("24/00001/FUL", "1 S", "X", "P", nil, nil, false, nil, nil, nil, nil, nil, nil, now, now))

	mock.ExpectQuery(`SELECT .+ FROM "planning_supports" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "case_reference", "support_reason", "ai_rationalisation", "created_at"}))

	r := chi.NewRouter()
	RegisterPlanning(r, db, nil, nil)

	body := `{"first_name":"John","last_name":"Doe","letter_type":"support"}`
	req := httptest.NewRequest("POST", "/api/planning/v1.0/cases/24/00001/FUL/generate-letter",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}
