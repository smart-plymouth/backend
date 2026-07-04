package tasks

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/smartplymouth/backend/internal/models"
)

const waitTimesURL = "https://www.plymouthhospitals.nhs.uk/urgent-waiting-times/"

var headingToLocation = map[string]string{
	"Emergency Department":              "Emergency Department",
	"UTC Dartmoor Building (Derriford)": "UTC Dartmoor",
	"UTC Cumberland Centre":             "UTC Cumberland Centre",
	"MIU Tavistock":                     "MIU Tavistock",
	"MIU Kingsbridge (South Hams)":      "MIU Kingsbridge",
}

type waitTimeEntry struct {
	LocationName         string
	LongestWait          int
	PatientsWaiting      int
	PatientsInDepartment int
}

func NewFetchWaitTimesHandler(db *gorm.DB) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		resp, err := http.Get(waitTimesURL)
		if err != nil {
			return fmt.Errorf("failed to fetch wait times page: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("wait times page returned status %d", resp.StatusCode)
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to parse wait times page: %w", err)
		}

		entries := parseWaitTimesPage(doc)
		if len(entries) == 0 {
			log.Println("No locations parsed from wait times page")
			return nil
		}

		now := time.Now().UTC()

		// Load all locations
		var locations []models.Location
		db.Find(&locations)
		locMap := make(map[string]models.Location)
		for _, loc := range locations {
			locMap[loc.Name] = loc
		}

		recordsAdded := 0
		for _, entry := range entries {
			loc, ok := locMap[entry.LocationName]
			if !ok {
				log.Printf("Location not found in DB: %s", entry.LocationName)
				continue
			}

			wt := models.WaitTime{
				LocationID:           loc.ID,
				Timestamp:            now,
				LongestWait:          entry.LongestWait,
				PatientsWaiting:      entry.PatientsWaiting,
				PatientsInDepartment: entry.PatientsInDepartment,
			}
			if err := db.Create(&wt).Error; err != nil {
				log.Printf("Failed to insert wait time for %s: %v", entry.LocationName, err)
				continue
			}
			recordsAdded++
		}

		log.Printf("Added %d wait time records", recordsAdded)
		return nil
	}
}

func parseWaitTimesPage(doc *goquery.Document) []waitTimeEntry {
	var entries []waitTimeEntry
	minutesRe := regexp.MustCompile(`(\d+)\s*minutes`)
	patientsRe := regexp.MustCompile(`(\d+)\s*patients`)

	// Find all h2 elements that match our known headings
	doc.Find("h2").Each(func(i int, s *goquery.Selection) {
		heading := strings.TrimSpace(s.Text())
		locationName, ok := headingToLocation[heading]
		if !ok {
			return
		}

		// Get all text content between this h2 and the next h2
		var sectionText strings.Builder
		s.NextUntil("h2").Each(func(_ int, el *goquery.Selection) {
			sectionText.WriteString(el.Text())
			sectionText.WriteString(" ")
		})
		text := sectionText.String()

		longestWait := 0
		if matches := minutesRe.FindStringSubmatch(text); len(matches) > 1 {
			longestWait, _ = strconv.Atoi(matches[1])
		}

		patientMatches := patientsRe.FindAllStringSubmatch(text, -1)
		patientsWaiting := 0
		patientsInDept := 0
		if len(patientMatches) > 0 {
			patientsWaiting, _ = strconv.Atoi(patientMatches[0][1])
		}
		if len(patientMatches) > 1 {
			patientsInDept, _ = strconv.Atoi(patientMatches[1][1])
		}

		entries = append(entries, waitTimeEntry{
			LocationName:         locationName,
			LongestWait:          longestWait,
			PatientsWaiting:      patientsWaiting,
			PatientsInDepartment: patientsInDept,
		})

		log.Printf("Parsed %s: wait=%dmin, waiting=%d, in_dept=%d",
			locationName, longestWait, patientsWaiting, patientsInDept)
	})

	return entries
}
