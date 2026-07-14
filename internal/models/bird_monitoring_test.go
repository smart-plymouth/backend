package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMonitoringSiteToDict(t *testing.T) {
	siteID := uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	site := &MonitoringSite{
		SiteID:    siteID,
		Name:      "Test Garden",
		Latitude:  50.3756,
		Longitude: -4.1427,
		Type:      "BirdNET-Pi",
	}

	dict := site.ToDict()

	if dict["site_id"] != siteID.String() {
		t.Errorf("unexpected site_id: %v", dict["site_id"])
	}
	if dict["name"] != "Test Garden" {
		t.Errorf("unexpected name: %v", dict["name"])
	}
	if dict["latitude"] != 50.3756 {
		t.Errorf("unexpected latitude: %v", dict["latitude"])
	}
	if dict["longitude"] != -4.1427 {
		t.Errorf("unexpected longitude: %v", dict["longitude"])
	}
	if dict["type"] != "BirdNET-Pi" {
		t.Errorf("unexpected type: %v", dict["type"])
	}
}

func TestMonitoringSiteTableName(t *testing.T) {
	s := MonitoringSite{}
	if s.TableName() != "monitoring_sites" {
		t.Errorf("expected 'monitoring_sites', got %q", s.TableName())
	}
}

func TestSpeciesToDict(t *testing.T) {
	sciName := "Turdus merula"
	species := &Species{
		SpeciesID:      1,
		CommonName:     "Blackbird",
		ScientificName: &sciName,
	}

	dict := species.ToDict()

	if dict["species_id"] != 1 {
		t.Errorf("unexpected species_id: %v", dict["species_id"])
	}
	if dict["common_name"] != "Blackbird" {
		t.Errorf("unexpected common_name: %v", dict["common_name"])
	}
	if dict["scientific_name"] != &sciName {
		// Check value via pointer
		if *(dict["scientific_name"].(*string)) != "Turdus merula" {
			t.Errorf("unexpected scientific_name: %v", dict["scientific_name"])
		}
	}
}

func TestSpeciesToDictNilScientificName(t *testing.T) {
	species := &Species{
		SpeciesID:  2,
		CommonName: "Unknown Bird",
	}

	dict := species.ToDict()
	if dict["scientific_name"] != (*string)(nil) {
		t.Errorf("expected nil scientific_name, got %v", dict["scientific_name"])
	}
}

func TestSpeciesTableName(t *testing.T) {
	s := Species{}
	if s.TableName() != "species" {
		t.Errorf("expected 'species', got %q", s.TableName())
	}
}

func TestSpeciesSightingToDict(t *testing.T) {
	siteID := uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	datetime := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	sciName := "Erithacus rubecula"

	sighting := &SpeciesSighting{
		SightingID: 42,
		SiteID:     siteID,
		SpeciesID:  3,
		Confidence: 0.95,
		Datetime:   datetime,
		SpeciesRel: &Species{
			SpeciesID:      3,
			CommonName:     "Robin",
			ScientificName: &sciName,
		},
	}

	dict := sighting.ToDict()

	if dict["sighting_id"] != 42 {
		t.Errorf("unexpected sighting_id: %v", dict["sighting_id"])
	}
	if dict["site_id"] != siteID.String() {
		t.Errorf("unexpected site_id: %v", dict["site_id"])
	}
	if dict["confidence"] != 0.95 {
		t.Errorf("unexpected confidence: %v", dict["confidence"])
	}
	if dict["datetime"] != "2024-06-15T14:30:00Z" {
		t.Errorf("unexpected datetime: %v", dict["datetime"])
	}
	if dict["species"] == nil {
		t.Error("expected species to be non-nil")
	}
}

func TestSpeciesSightingToDictNoSpecies(t *testing.T) {
	siteID := uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	sighting := &SpeciesSighting{
		SightingID: 43,
		SiteID:     siteID,
		SpeciesID:  3,
		Confidence: 0.80,
		Datetime:   time.Now(),
	}

	dict := sighting.ToDict()
	if dict["species"] != nil {
		t.Errorf("expected nil species, got %v", dict["species"])
	}
}

func TestSpeciesSightingTableName(t *testing.T) {
	s := SpeciesSighting{}
	if s.TableName() != "species_sightings" {
		t.Errorf("expected 'species_sightings', got %q", s.TableName())
	}
}
