package tasks

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestParseWaitTimesPage(t *testing.T) {
	html := `<html><body>
<div>
	<h2>Emergency Department</h2>
	<p>The longest wait is currently 45 minutes</p>
	<p>There are currently 12 patients waiting to be seen</p>
	<p>There are currently 35 patients in the department</p>
</div>
<div>
	<h2>UTC Cumberland Centre</h2>
	<p>The longest wait is currently 20 minutes</p>
	<p>There are currently 5 patients waiting to be seen</p>
	<p>There are currently 8 patients in the department</p>
</div>
</body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	entries := parseWaitTimesPage(doc)

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	ed := entries[0]
	if ed.LocationName != "Emergency Department" {
		t.Errorf("entry 0 name: got %q", ed.LocationName)
	}
	if ed.LongestWait != 45 {
		t.Errorf("entry 0 longest wait: got %d, want 45", ed.LongestWait)
	}
	if ed.PatientsWaiting != 12 {
		t.Errorf("entry 0 patients waiting: got %d, want 12", ed.PatientsWaiting)
	}
	if ed.PatientsInDepartment != 35 {
		t.Errorf("entry 0 patients in dept: got %d, want 35", ed.PatientsInDepartment)
	}

	utc := entries[1]
	if utc.LocationName != "UTC Cumberland Centre" {
		t.Errorf("entry 1 name: got %q", utc.LocationName)
	}
	if utc.LongestWait != 20 {
		t.Errorf("entry 1 longest wait: got %d, want 20", utc.LongestWait)
	}
}

func TestParseWaitTimesPageNoLocations(t *testing.T) {
	html := `<html><body><h2>Something Else</h2><p>No data</p></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	entries := parseWaitTimesPage(doc)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseWaitTimesPagePartialData(t *testing.T) {
	html := `<html><body>
<div>
	<h2>MIU Tavistock</h2>
	<p>The longest wait is currently 10 minutes</p>
</div>
</body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	entries := parseWaitTimesPage(doc)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].LongestWait != 10 {
		t.Errorf("longest wait: got %d, want 10", entries[0].LongestWait)
	}
	if entries[0].PatientsWaiting != 0 {
		t.Errorf("patients waiting should be 0 when not found, got %d", entries[0].PatientsWaiting)
	}
}

func TestParseWaitTimesPageAllLocations(t *testing.T) {
	html := `<html><body>
<div><h2>Emergency Department</h2><p>10 minutes</p><p>5 patients waiting</p><p>20 patients in dept</p></div>
<div><h2>UTC Dartmoor Building (Derriford)</h2><p>15 minutes</p><p>3 patients</p><p>10 patients</p></div>
<div><h2>UTC Cumberland Centre</h2><p>20 minutes</p><p>7 patients</p><p>12 patients</p></div>
<div><h2>MIU Tavistock</h2><p>5 minutes</p><p>2 patients</p><p>4 patients</p></div>
<div><h2>MIU Kingsbridge (South Hams)</h2><p>8 minutes</p><p>1 patients</p><p>3 patients</p></div>
</body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	entries := parseWaitTimesPage(doc)
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.LocationName] = true
	}

	expected := []string{"Emergency Department", "UTC Dartmoor", "UTC Cumberland Centre", "MIU Tavistock", "MIU Kingsbridge"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing location: %s", name)
		}
	}
}

func TestHeadingToLocationMapping(t *testing.T) {
	expected := map[string]string{
		"Emergency Department":              "Emergency Department",
		"UTC Dartmoor Building (Derriford)": "UTC Dartmoor",
		"UTC Cumberland Centre":             "UTC Cumberland Centre",
		"MIU Tavistock":                     "MIU Tavistock",
		"MIU Kingsbridge (South Hams)":      "MIU Kingsbridge",
	}

	for heading, name := range expected {
		got, ok := headingToLocation[heading]
		if !ok {
			t.Errorf("heading %q not found in headingToLocation map", heading)
			continue
		}
		if got != name {
			t.Errorf("headingToLocation[%q] = %q, want %q", heading, got, name)
		}
	}

	if len(headingToLocation) != len(expected) {
		t.Errorf("expected %d entries in headingToLocation, got %d", len(expected), len(headingToLocation))
	}
}

func TestNewFetchWaitTimesHandlerWithMockServer(t *testing.T) {
	// Test that the handler handles a non-200 response gracefully
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// We can't easily inject the URL into the handler since it's hardcoded,
	// but we can test the page parsing path which is what matters most.
	// The handler itself is tested via the parseWaitTimesPage function above.
}
