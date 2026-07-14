package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJSONStringArrayValue(t *testing.T) {
	tests := []struct {
		name    string
		input   JSONStringArray
		wantNil bool
	}{
		{
			name:    "nil array returns nil",
			input:   nil,
			wantNil: true,
		},
		{
			name:    "empty array returns valid JSON",
			input:   JSONStringArray{},
			wantNil: false,
		},
		{
			name:    "populated array returns valid JSON",
			input:   JSONStringArray{"residential", "extension", "listed-building"},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.input.Value()
			if err != nil {
				t.Fatalf("Value() returned error: %v", err)
			}

			if tt.wantNil {
				if val != nil {
					t.Errorf("expected nil, got %v", val)
				}
				return
			}

			// Verify it's valid JSON
			bytes, ok := val.([]byte)
			if !ok {
				t.Fatalf("Value() returned non-byte type: %T", val)
			}

			var result []string
			if err := json.Unmarshal(bytes, &result); err != nil {
				t.Fatalf("Value() returned invalid JSON: %v", err)
			}

			if len(result) != len(tt.input) {
				t.Errorf("expected %d elements, got %d", len(tt.input), len(result))
			}
		})
	}
}

func TestJSONStringArrayScan(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    JSONStringArray
		wantNil bool
	}{
		{
			name:    "nil value",
			input:   nil,
			wantNil: true,
		},
		{
			name:  "byte slice",
			input: []byte(`["tag1","tag2","tag3"]`),
			want:  JSONStringArray{"tag1", "tag2", "tag3"},
		},
		{
			name:  "string value",
			input: `["hello","world"]`,
			want:  JSONStringArray{"hello", "world"},
		},
		{
			name:  "empty array",
			input: []byte(`[]`),
			want:  JSONStringArray{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSONStringArray
			err := j.Scan(tt.input)
			if err != nil {
				t.Fatalf("Scan() returned error: %v", err)
			}

			if tt.wantNil {
				if j != nil {
					t.Errorf("expected nil, got %v", j)
				}
				return
			}

			if len(j) != len(tt.want) {
				t.Errorf("expected %d elements, got %d", len(tt.want), len(j))
				return
			}

			for i := range j {
				if j[i] != tt.want[i] {
					t.Errorf("element %d: got %q, want %q", i, j[i], tt.want[i])
				}
			}
		})
	}
}

func TestPlanningCaseToDict(t *testing.T) {
	received := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	validated := time.Date(2024, 3, 20, 0, 0, 0, 0, time.UTC)
	impact := 7
	size := 5
	rationalisation := "This is a significant development"

	c := &PlanningCase{
		Reference:            "24/00123/FUL",
		Address:              "1 Test Street, Plymouth",
		Proposal:             "Build a house",
		Status:               "Pending",
		ReceivedDate:         &received,
		ValidatedDate:        &validated,
		AIAnalysis:           true,
		PotentialImpactScore: &impact,
		EstimatedSize:        &size,
		Tags:                 JSONStringArray{"residential", "new-build"},
		AIRationalisation:    &rationalisation,
		Pros:                 JSONStringArray{"more housing"},
		Cons:                 JSONStringArray{"traffic increase"},
		CreatedAt:            time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2024, 3, 20, 14, 0, 0, 0, time.UTC),
	}

	dict := c.ToDict()

	if dict["reference"] != "24/00123/FUL" {
		t.Errorf("unexpected reference: %v", dict["reference"])
	}
	if dict["address"] != "1 Test Street, Plymouth" {
		t.Errorf("unexpected address: %v", dict["address"])
	}
	if dict["ai_analysis"] != true {
		t.Errorf("unexpected ai_analysis: %v", dict["ai_analysis"])
	}
	if dict["received_date"] != "2024-03-15" {
		t.Errorf("unexpected received_date: %v", dict["received_date"])
	}
	if dict["validated_date"] != "2024-03-20" {
		t.Errorf("unexpected validated_date: %v", dict["validated_date"])
	}
}

func TestPlanningCaseToDictNilDates(t *testing.T) {
	c := &PlanningCase{
		Reference: "24/00456/FUL",
		Address:   "2 Test Street",
		Proposal:  "Extension",
		Status:    "Pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	dict := c.ToDict()

	if dict["received_date"] != nil {
		t.Errorf("expected nil received_date, got %v", dict["received_date"])
	}
	if dict["validated_date"] != nil {
		t.Errorf("expected nil validated_date, got %v", dict["validated_date"])
	}
}

func TestPlanningCaseTableName(t *testing.T) {
	c := PlanningCase{}
	if c.TableName() != "planning_cases" {
		t.Errorf("expected table name 'planning_cases', got %q", c.TableName())
	}
}

func TestPlanningObjectionTableName(t *testing.T) {
	o := PlanningObjection{}
	if o.TableName() != "planning_objections" {
		t.Errorf("expected table name 'planning_objections', got %q", o.TableName())
	}
}

func TestPlanningSupportTableName(t *testing.T) {
	s := PlanningSupport{}
	if s.TableName() != "planning_supports" {
		t.Errorf("expected table name 'planning_supports', got %q", s.TableName())
	}
}

func TestPlanningPhasetenEmailTableName(t *testing.T) {
	e := PlanningPhasetenEmail{}
	if e.TableName() != "planning_phaseten_emails" {
		t.Errorf("expected table name 'planning_phaseten_emails', got %q", e.TableName())
	}
}
