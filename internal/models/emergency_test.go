package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestLocationTableName(t *testing.T) {
	l := Location{}
	if l.TableName() != "ed_wait_times_locations" {
		t.Errorf("expected 'ed_wait_times_locations', got %q", l.TableName())
	}
}

func TestWaitTimeTableName(t *testing.T) {
	w := WaitTime{}
	if w.TableName() != "ed_wait_times_wait_times" {
		t.Errorf("expected 'ed_wait_times_wait_times', got %q", w.TableName())
	}
}

func TestLocationFields(t *testing.T) {
	phone := "01234 567890"
	hours := "24 hours"

	loc := Location{
		ID:              uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"),
		Name:            "Emergency Department",
		Type:            "ED",
		Address:         "Derriford Hospital, Plymouth",
		Longitude:       -4.1137,
		Latitude:        50.4167,
		OpeningTimes:    &hours,
		TelephoneNumber: &phone,
	}

	if loc.Name != "Emergency Department" {
		t.Errorf("unexpected Name: %s", loc.Name)
	}
	if loc.Type != "ED" {
		t.Errorf("unexpected Type: %s", loc.Type)
	}
	if *loc.TelephoneNumber != "01234 567890" {
		t.Errorf("unexpected TelephoneNumber: %s", *loc.TelephoneNumber)
	}
	if *loc.OpeningTimes != "24 hours" {
		t.Errorf("unexpected OpeningTimes: %s", *loc.OpeningTimes)
	}
}
