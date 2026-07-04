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

	// Collect all location h2 elements and their positions in the document
	type locationH2 struct {
		selection    *goquery.Selection
		locationName string
	}

	var locationH2s []locationH2
	doc.Find("h2").Each(func(i int, s *goquery.Selection) {
		heading := strings.TrimSpace(s.Text())
		if name, ok := headingToLocation[heading]; ok {
			locationH2s = append(locationH2s, locationH2{
				selection:    s,
				locationName: name,
			})
		}
	})

	// For each location h2, find the closest ancestor that wraps just this
	// location's section. The page uses a container per location — find the
	// ancestor whose text contains "minutes" or "patients" but is as narrow
	// as possible (i.e. doesn't contain another location's h2).
	for _, loc := range locationH2s {
		sectionText := extractSectionText(loc.selection, doc)

		longestWait := 0
		if matches := minutesRe.FindStringSubmatch(sectionText); len(matches) > 1 {
			longestWait, _ = strconv.Atoi(matches[1])
		}

		patientMatches := patientsRe.FindAllStringSubmatch(sectionText, -1)
		patientsWaiting := 0
		patientsInDept := 0
		if len(patientMatches) > 0 {
			patientsWaiting, _ = strconv.Atoi(patientMatches[0][1])
		}
		if len(patientMatches) > 1 {
			patientsInDept, _ = strconv.Atoi(patientMatches[1][1])
		}

		entries = append(entries, waitTimeEntry{
			LocationName:         loc.locationName,
			LongestWait:          longestWait,
			PatientsWaiting:      patientsWaiting,
			PatientsInDepartment: patientsInDept,
		})

		log.Printf("Parsed %s: wait=%dmin, waiting=%d, in_dept=%d",
			loc.locationName, longestWait, patientsWaiting, patientsInDept)
	}

	return entries
}

// extractSectionText finds the narrowest ancestor of the h2 that contains
// the stats data (minutes/patients text) without containing another h2 from
// our known set. This replicates the Python approach of walking forward through
// document nodes until hitting the next location heading.
func extractSectionText(h2 *goquery.Selection, doc *goquery.Document) string {
	// Strategy: walk up the ancestor chain from the h2. At each level, check
	// if the ancestor's text contains "patients" (our data marker) and count
	// how many of our known-location h2s are inside it. We want the smallest
	// ancestor that has our data and contains exactly one of our location h2s.
	current := h2.Parent()
	for current.Length() > 0 {
		text := current.Text()
		if strings.Contains(text, "patients") {
			// Count how many known-location h2s are inside this element
			h2Count := 0
			current.Find("h2").Each(func(_ int, s *goquery.Selection) {
				heading := strings.TrimSpace(s.Text())
				if _, ok := headingToLocation[heading]; ok {
					h2Count++
				}
			})
			if h2Count == 1 {
				return text
			}
		}
		current = current.Parent()
	}

	// Fallback: use the h2's parent text (may include some noise but should
	// still match our regexes)
	return h2.Parent().Text()
}
