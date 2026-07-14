package tasks

import (
	"encoding/json"
	"testing"
)

func TestParseAQDataPointsMixedArray(t *testing.T) {
	// Mixed array with nulls - uses pointStart + pointInterval
	series := highchartsSeriesAQ{
		Name:          "O3",
		PointStart:    1700000000000,
		PointInterval: 3600000,
		Data:          json.RawMessage(`[20.0, null, 25.0]`),
	}

	points, err := parseAQDataPoints(series)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// null should be skipped, so we expect 2 points
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if points[0].TimestampMS != 1700000000000 {
		t.Errorf("point 0 timestamp: got %d", points[0].TimestampMS)
	}
	if points[0].Value != 20.0 {
		t.Errorf("point 0 value: got %f", points[0].Value)
	}
	if points[1].TimestampMS != 1700007200000 {
		t.Errorf("point 1 timestamp: got %d, want %d", points[1].TimestampMS, int64(1700007200000))
	}
}

func TestParseAQDataPointsDefaultInterval(t *testing.T) {
	// If PointInterval is 0, should default to 3600000 (1 hour)
	series := highchartsSeriesAQ{
		Name:          "NO2",
		PointStart:    1700000000000,
		PointInterval: 0,
		Data:          json.RawMessage(`[10.0, 20.0]`),
	}

	points, err := parseAQDataPoints(series)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	// Second point should be 1 hour (3600000ms) after start
	expected := int64(1700000000000 + 3600000)
	if points[1].TimestampMS != expected {
		t.Errorf("point 1 timestamp: got %d, want %d", points[1].TimestampMS, expected)
	}
}

func TestParseAQDataPointsEmptyData(t *testing.T) {
	series := highchartsSeriesAQ{
		Name:          "PM10",
		PointStart:    1700000000000,
		PointInterval: 3600000,
		Data:          json.RawMessage(`[]`),
	}

	points, err := parseAQDataPoints(series)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 0 {
		t.Errorf("expected 0 points for empty data, got %d", len(points))
	}
}

func TestParseAQDataPointsAllNulls(t *testing.T) {
	series := highchartsSeriesAQ{
		Name:          "SO2",
		PointStart:    1700000000000,
		PointInterval: 3600000,
		Data:          json.RawMessage(`[null, null, null]`),
	}

	points, err := parseAQDataPoints(series)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 0 {
		t.Errorf("expected 0 points for all-null data, got %d", len(points))
	}
}

func TestExtractSeriesFallbackNoMatch(t *testing.T) {
	html := `<html><body><p>No chart data here</p></body></html>`
	series := extractSeriesFallback(html)
	if series != nil {
		t.Errorf("expected nil for no match, got %v", series)
	}
}

func TestExtractSeriesFallbackWithData(t *testing.T) {
	html := `<script>
Highcharts.chart('container', {
	"series" : [{"name":"NO2","pointStart":1700000000000,"pointInterval":3600000,"data":[10.0,20.0]}] , "legend": {}
});
</script>`

	series := extractSeriesFallback(html)
	if len(series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(series))
	}
	if series[0].Name != "NO2" {
		t.Errorf("expected name 'NO2', got %q", series[0].Name)
	}
}

func TestDefraSites(t *testing.T) {
	// Verify known DEFRA sites are configured
	if len(defraSites) < 2 {
		t.Errorf("expected at least 2 DEFRA sites, got %d", len(defraSites))
	}

	for _, site := range defraSites {
		if site.DefraID == "" {
			t.Error("site has empty DefraID")
		}
		if site.Name == "" {
			t.Error("site has empty Name")
		}
		if site.Latitude == 0 || site.Longitude == 0 {
			t.Errorf("site %s has zero coordinates", site.Name)
		}
	}
}
