package models

import (
	"time"

	"github.com/google/uuid"
)

type Location struct {
	ID              uuid.UUID  `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	Name            string     `gorm:"column:name;type:varchar(255);not null" json:"name"`
	Type            string     `gorm:"column:type;type:varchar(50);not null" json:"type"`
	Address         string     `gorm:"column:address;type:text;not null" json:"address"`
	Longitude       float64    `gorm:"column:longitude;not null" json:"longitude"`
	Latitude        float64    `gorm:"column:latitude;not null" json:"latitude"`
	OpeningTimes    *string    `gorm:"column:opening_times;type:text" json:"opening_times"`
	TelephoneNumber *string    `gorm:"column:telephone_number;type:varchar(50)" json:"telephone_number"`
}

func (Location) TableName() string {
	return "ed_wait_times_locations"
}

type WaitTime struct {
	LocationID           uuid.UUID `gorm:"column:location_id;type:uuid;primaryKey" json:"location_id"`
	Timestamp            time.Time `gorm:"column:timestamp;type:timestamptz;primaryKey" json:"timestamp"`
	LongestWait          int       `gorm:"column:longest_wait;not null" json:"longest_wait"`
	PatientsWaiting      int       `gorm:"column:patients_waiting;not null" json:"patients_waiting"`
	PatientsInDepartment int       `gorm:"column:patients_in_department;not null" json:"patients_in_department"`
}

func (WaitTime) TableName() string {
	return "ed_wait_times_wait_times"
}
