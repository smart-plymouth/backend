package tasks

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// --- getPreviousWeekMonday tests ---

func TestGetPreviousWeekMonday(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "from a Wednesday",
			now:  time.Date(2024, 7, 10, 15, 30, 0, 0, time.UTC), // Wednesday
			want: time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),    // Previous Monday
		},
		{
			name: "from a Monday",
			now:  time.Date(2024, 7, 8, 9, 0, 0, 0, time.UTC),  // Monday
			want: time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),  // Previous Monday
		},
		{
			name: "from a Sunday",
			now:  time.Date(2024, 7, 7, 23, 59, 0, 0, time.UTC), // Sunday
			want: time.Date(2024, 6, 24, 0, 0, 0, 0, time.UTC),  // Previous Monday
		},
		{
			name: "from a Saturday",
			now:  time.Date(2024, 7, 6, 12, 0, 0, 0, time.UTC), // Saturday
			want: time.Date(2024, 6, 24, 0, 0, 0, 0, time.UTC), // Previous Monday
		},
		{
			name: "from a Friday",
			now:  time.Date(2024, 7, 5, 8, 0, 0, 0, time.UTC), // Friday
			want: time.Date(2024, 6, 24, 0, 0, 0, 0, time.UTC), // Previous Monday
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPreviousWeekMonday(tt.now)
			if !got.Equal(tt.want) {
				t.Errorf("getPreviousWeekMonday(%v) = %v, want %v", tt.now, got, tt.want)
			}
			// Result should always be a Monday
			if got.Weekday() != time.Monday {
				t.Errorf("expected Monday, got %v", got.Weekday())
			}
		})
	}
}

// --- parseDateUK tests ---

func TestParseDateUK(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		want    time.Time
	}{
		{
			name:  "standard format: 2 Jan 2006",
			input: "5 Mar 2024",
			want:  time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "zero-padded: 02 Jan 2006",
			input: "15 Jul 2024",
			want:  time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "full month: 2 January 2006",
			input: "1 December 2023",
			want:  time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "slash format: 02/01/2006",
			input: "25/12/2024",
			want:  time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "invalid format returns nil",
			input:   "not-a-date",
			wantNil: true,
		},
		{
			name:    "empty string returns nil",
			input:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDateUK(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Errorf("parseDateUK(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("parseDateUK(%q) = nil, want %v", tt.input, tt.want)
			}
			if !got.Equal(tt.want) {
				t.Errorf("parseDateUK(%q) = %v, want %v", tt.input, *got, tt.want)
			}
		})
	}
}

// --- sanitizeUTF8 tests ---

func TestSanitizeUTF8(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "valid UTF-8 unchanged",
			input: "Hello, World!",
			want:  "Hello, World!",
		},
		{
			name:  "non-breaking space replaced",
			input: "hello" + string([]byte{0xa0}) + "world",
			want:  "hello world",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "unicode characters preserved",
			input: "café résumé",
			want:  "café résumé",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeUTF8(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeUTF8(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- parseResults tests ---

func TestParseResults(t *testing.T) {
	html := `<html><body>
<ul>
<li class="searchresult">
	<a class="summaryLink"><div>Erect two storey extension</div></a>
	<p class="address">123 High Street, Plymouth</p>
	<p class="metaInfo">Ref. No: 24/00001/FUL | Received: Mon 15 Jan 2024 | Validated: Mon 22 Jan 2024 | Status: Pending</p>
</li>
<li class="searchresult">
	<a class="summaryLink"><div>Change of use to HMO</div></a>
	<p class="address">456 Low Road, Plymouth</p>
	<p class="metaInfo">Ref. No: 24/00002/FUL | Received: Mon 1 Feb 2024 | Validated: Mon 8 Feb 2024 | Status: Approved</p>
</li>
</ul>
</body></html>`

	cases := parseResults([]string{html})

	if len(cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(cases))
	}

	// First case
	if cases[0].Reference != "24/00001/FUL" {
		t.Errorf("case 0 reference: got %q", cases[0].Reference)
	}
	if cases[0].Address != "123 High Street, Plymouth" {
		t.Errorf("case 0 address: got %q", cases[0].Address)
	}
	if cases[0].Proposal != "Erect two storey extension" {
		t.Errorf("case 0 proposal: got %q", cases[0].Proposal)
	}
	if cases[0].Status != "Pending" {
		t.Errorf("case 0 status: got %q", cases[0].Status)
	}

	// Second case
	if cases[1].Reference != "24/00002/FUL" {
		t.Errorf("case 1 reference: got %q", cases[1].Reference)
	}
	if cases[1].Status != "Approved" {
		t.Errorf("case 1 status: got %q", cases[1].Status)
	}
}

func TestParseResultsNoResults(t *testing.T) {
	html := `<html><body><p>No results found</p></body></html>`
	cases := parseResults([]string{html})
	if len(cases) != 0 {
		t.Errorf("expected 0 cases, got %d", len(cases))
	}
}

func TestParseResultsMultiplePages(t *testing.T) {
	page1 := `<html><body>
<li class="searchresult">
	<a class="summaryLink"><div>Extension</div></a>
	<p class="address">1 Street</p>
	<p class="metaInfo">Ref. No: 24/00010/FUL | Status: Pending</p>
</li>
</body></html>`

	page2 := `<html><body>
<li class="searchresult">
	<a class="summaryLink"><div>New Build</div></a>
	<p class="address">2 Avenue</p>
	<p class="metaInfo">Ref. No: 24/00011/FUL | Status: Approved</p>
</li>
</body></html>`

	cases := parseResults([]string{page1, page2})
	if len(cases) != 2 {
		t.Fatalf("expected 2 cases across pages, got %d", len(cases))
	}
}

func TestParseResultsSkipsMissingReference(t *testing.T) {
	html := `<html><body>
<li class="searchresult">
	<a class="summaryLink"><div>Some proposal</div></a>
	<p class="address">Some address</p>
	<p class="metaInfo">No reference here</p>
</li>
</body></html>`

	cases := parseResults([]string{html})
	if len(cases) != 0 {
		t.Errorf("expected 0 cases when no reference, got %d", len(cases))
	}
}

// --- sanitizeFilename tests ---

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal filename",
			input: "document.pdf",
			want:  "document.pdf",
		},
		{
			name:  "filename with special characters",
			input: "file<>name|here.pdf",
			want:  "filenamehere.pdf",
		},
		{
			name:  "filename with dashes and underscores",
			input: "my-file_v2.pdf",
			want:  "my-file_v2.pdf",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only special characters",
			input: "<>|!@#$%^&*()",
			want:  "",
		},
		{
			name:  "spaces and alphanumeric",
			input: "Design and Access Statement.pdf",
			want:  "Design and Access Statement.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- AI analysis helper tests ---

func TestClamp(t *testing.T) {
	tests := []struct {
		name     string
		val      int
		min, max int
		want     int
	}{
		{"within range", 5, 1, 10, 5},
		{"below minimum", -1, 1, 10, 1},
		{"above maximum", 15, 1, 10, 10},
		{"at minimum", 1, 1, 10, 1},
		{"at maximum", 10, 1, 10, 10},
		{"zero range", 5, 5, 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clamp(tt.val, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.val, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestLimitSlice(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		max   int
		want  int
	}{
		{"nil slice", nil, 5, 0},
		{"empty slice", []string{}, 5, 0},
		{"under limit", []string{"a", "b"}, 5, 2},
		{"at limit", []string{"a", "b", "c"}, 3, 3},
		{"over limit", []string{"a", "b", "c", "d", "e"}, 3, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := limitSlice(tt.input, tt.max)
			if len(got) != tt.want {
				t.Errorf("limitSlice(len=%d, max=%d) returned len=%d, want %d",
					len(tt.input), tt.max, len(got), tt.want)
			}
		})
	}
}

func TestFormatMetadata(t *testing.T) {
	metadata := map[string]string{
		"Reference": "24/00001/FUL",
		"Address":   "123 Street",
	}

	result := formatMetadata(metadata)

	if !strings.Contains(result, "Reference: 24/00001/FUL") {
		t.Error("expected result to contain Reference")
	}
	if !strings.Contains(result, "Address: 123 Street") {
		t.Error("expected result to contain Address")
	}
	if !strings.HasPrefix(result, "- ") {
		t.Error("expected result lines to start with '- '")
	}
}

func TestFormatMetadataEmpty(t *testing.T) {
	result := formatMetadata(map[string]string{})
	if result != "" {
		t.Errorf("expected empty string for empty metadata, got %q", result)
	}
}

func TestCleanJSONResponse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already clean JSON object",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON with markdown fences",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON with think tags",
			input: "<think>reasoning here</think>{\"key\": \"value\"}",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON array without braces",
			input: `[{"a": 1}]`,
			want:  `{"a": 1}`,
		},
		{
			name:  "pure JSON array",
			input: `["a", "b", "c"]`,
			want:  `["a", "b", "c"]`,
		},
		{
			name:  "text before JSON object",
			input: "Here is the analysis:\n{\"score\": 5}",
			want:  `{"score": 5}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanJSONResponse(tt.input)
			if got != tt.want {
				t.Errorf("cleanJSONResponse() = %q, want %q", got, tt.want)
			}
			// Verify the result is valid JSON
			if !json.Valid([]byte(got)) {
				t.Errorf("cleanJSONResponse() result is not valid JSON: %q", got)
			}
		})
	}
}

// --- parseAQDataPoints tests ---

func TestParseAQDataPointsPairs(t *testing.T) {
	series := highchartsSeriesAQ{
		Name: "NO2",
		Data: json.RawMessage(`[[1700000000000, 25.5], [1700003600000, 30.2]]`),
	}

	points, err := parseAQDataPoints(series)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if points[0].TimestampMS != 1700000000000 {
		t.Errorf("point 0 timestamp: got %d", points[0].TimestampMS)
	}
	if points[0].Value != 25.5 {
		t.Errorf("point 0 value: got %f", points[0].Value)
	}
	if points[1].Value != 30.2 {
		t.Errorf("point 1 value: got %f", points[1].Value)
	}
}

func TestParseAQDataPointsPlainArray(t *testing.T) {
	val1 := 10.0
	val2 := 15.0

	series := highchartsSeriesAQ{
		Name:          "PM10",
		PointStart:    1700000000000,
		PointInterval: 3600000,
		Data:          json.RawMessage(`[10.0, 15.0, null]`),
	}

	points, err := parseAQDataPoints(series)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 2 { // null should be skipped
		t.Fatalf("expected 2 points (null skipped), got %d", len(points))
	}
	if points[0].Value != val1 {
		t.Errorf("point 0 value: got %f, want %f", points[0].Value, val1)
	}
	if points[1].Value != val2 {
		t.Errorf("point 1 value: got %f, want %f", points[1].Value, val2)
	}
	if points[1].TimestampMS != 1700003600000 {
		t.Errorf("point 1 timestamp: got %d, want %d", points[1].TimestampMS, int64(1700003600000))
	}
}

// --- Task type constants tests ---

func TestTaskTypeConstants(t *testing.T) {
	if TypeFetchWaitTimes != "fetch_wait_times" {
		t.Errorf("unexpected TypeFetchWaitTimes: %s", TypeFetchWaitTimes)
	}
	if TypeFetchWeeklyPlanning != "fetch_weekly_planning" {
		t.Errorf("unexpected TypeFetchWeeklyPlanning: %s", TypeFetchWeeklyPlanning)
	}
	if TypeRefreshPlanningApplications != "refresh_planning_applications" {
		t.Errorf("unexpected TypeRefreshPlanningApplications: %s", TypeRefreshPlanningApplications)
	}
	if TypeAnalysePlanningApplication != "analyse_planning_application" {
		t.Errorf("unexpected TypeAnalysePlanningApplication: %s", TypeAnalysePlanningApplication)
	}
	if TypeFetchAirQuality != "fetch_air_quality" {
		t.Errorf("unexpected TypeFetchAirQuality: %s", TypeFetchAirQuality)
	}
	if TypeExampleTask != "example_task" {
		t.Errorf("unexpected TypeExampleTask: %s", TypeExampleTask)
	}
}
