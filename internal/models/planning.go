package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// JSONStringArray is a custom type for JSON array columns.
type JSONStringArray []string

func (j JSONStringArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONStringArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		s := value.(string)
		bytes = []byte(s)
	}
	return json.Unmarshal(bytes, j)
}

type PlanningCase struct {
	Reference            string          `gorm:"column:reference;type:varchar(50);primaryKey" json:"reference"`
	Address              string          `gorm:"column:address;type:text;not null" json:"address"`
	Proposal             string          `gorm:"column:proposal;type:text;not null" json:"proposal"`
	Status               string          `gorm:"column:status;type:varchar(100);not null" json:"status"`
	ReceivedDate         *time.Time      `gorm:"column:received_date;type:date" json:"received_date"`
	ValidatedDate        *time.Time      `gorm:"column:validated_date;type:date" json:"validated_date"`
	AIAnalysis           bool            `gorm:"column:ai_analysis;not null;default:false" json:"ai_analysis"`
	PotentialImpactScore *int            `gorm:"column:potential_impact_score" json:"potential_impact_score"`
	Tags                 JSONStringArray `gorm:"column:tags;type:jsonb" json:"tags"`
	EstimatedSize        *int            `gorm:"column:estimated_size" json:"estimated_size"`
	AIRationalisation    *string         `gorm:"column:ai_rationalisation;type:text" json:"ai_rationalisation"`
	Pros                 JSONStringArray `gorm:"column:pros;type:jsonb" json:"pros"`
	Cons                 JSONStringArray `gorm:"column:cons;type:jsonb" json:"cons"`
	CreatedAt            time.Time       `gorm:"column:created_at;type:timestamptz;autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time       `gorm:"column:updated_at;type:timestamptz;autoUpdateTime" json:"updated_at"`
}

func (PlanningCase) TableName() string {
	return "planning_cases"
}

func (c *PlanningCase) ToDict() map[string]interface{} {
	result := map[string]interface{}{
		"reference":              c.Reference,
		"address":               c.Address,
		"proposal":              c.Proposal,
		"status":                c.Status,
		"ai_analysis":           c.AIAnalysis,
		"potential_impact_score": c.PotentialImpactScore,
		"tags":                  c.Tags,
		"estimated_size":        c.EstimatedSize,
		"ai_rationalisation":    c.AIRationalisation,
		"pros":                  c.Pros,
		"cons":                  c.Cons,
		"created_at":            c.CreatedAt.Format(time.RFC3339),
		"updated_at":            c.UpdatedAt.Format(time.RFC3339),
	}
	if c.ReceivedDate != nil {
		result["received_date"] = c.ReceivedDate.Format("2006-01-02")
	} else {
		result["received_date"] = nil
	}
	if c.ValidatedDate != nil {
		result["validated_date"] = c.ValidatedDate.Format("2006-01-02")
	} else {
		result["validated_date"] = nil
	}
	return result
}

type PlanningObjection struct {
	ID                int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CaseReference     string    `gorm:"column:case_reference;type:varchar(50);not null;index" json:"case_reference"`
	Objection         string    `gorm:"column:objection;type:text;not null" json:"objection"`
	AIRationalisation string    `gorm:"column:ai_rationalisation;type:text;not null" json:"ai_rationalisation"`
	CreatedAt         time.Time `gorm:"column:created_at;type:timestamptz;autoCreateTime" json:"created_at"`
}

func (PlanningObjection) TableName() string {
	return "planning_objections"
}

type PlanningSupport struct {
	ID                int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CaseReference     string    `gorm:"column:case_reference;type:varchar(50);not null;index" json:"case_reference"`
	SupportReason     string    `gorm:"column:support_reason;type:text;not null" json:"support_reason"`
	AIRationalisation string    `gorm:"column:ai_rationalisation;type:text;not null" json:"ai_rationalisation"`
	CreatedAt         time.Time `gorm:"column:created_at;type:timestamptz;autoCreateTime" json:"created_at"`
}

func (PlanningSupport) TableName() string {
	return "planning_supports"
}
