package tasks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/smartplymouth/backend/internal/config"
	"github.com/smartplymouth/backend/internal/models"
)

const (
	weeklyListURL        = "https://planning.plymouth.gov.uk/online-applications/search.do"
	weeklyListResultsURL = "https://planning.plymouth.gov.uk/online-applications/weeklyListResults.do"
	pagedResultsURL      = "https://planning.plymouth.gov.uk/online-applications/pagedSearchResults.do"
)

type fetchWeeklyPayload struct {
	WeekStartISO string `json:"week_start_iso,omitempty"`
}

func NewFetchWeeklyPlanningHandler(db *gorm.DB, cfg *config.Config) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload fetchWeeklyPayload
		if t.Payload() != nil {
			json.Unmarshal(t.Payload(), &payload)
		}

		var weekStart time.Time
		if payload.WeekStartISO != "" {
			parsed, err := time.Parse("2006-01-02", payload.WeekStartISO)
			if err != nil {
				return fmt.Errorf("invalid week_start_iso: %w", err)
			}
			weekStart = parsed
		} else {
			weekStart = getPreviousWeekMonday(time.Now())
		}

		log.Printf("Fetching planning applications for week beginning %s", weekStart.Format("2006-01-02"))

		client := &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		htmlPages, err := fetchAllPages(weekStart, client)
		if err != nil {
			return fmt.Errorf("failed to fetch weekly planning list: %w", err)
		}

		cases := parseResults(htmlPages)
		if len(cases) == 0 {
			log.Println("No planning applications parsed - page structure may have changed")
			return nil
		}

		casesAdded := 0
		casesUpdated := 0
		var newRefs []string
		var unanalysedRefs []string

		for _, caseData := range cases {
			var existing models.PlanningCase
			result := db.Where("reference = ?", caseData.Reference).First(&existing)

			if result.Error == nil {
				// Update existing
				if caseData.Address != "" {
					existing.Address = caseData.Address
				}
				if caseData.Proposal != "" {
					existing.Proposal = caseData.Proposal
				}
				if caseData.Status != "" {
					existing.Status = caseData.Status
				}
				if caseData.ReceivedDate != nil {
					existing.ReceivedDate = caseData.ReceivedDate
				}
				if caseData.ValidatedDate != nil {
					existing.ValidatedDate = caseData.ValidatedDate
				}
				db.Save(&existing)
				casesUpdated++
				if !existing.AIAnalysis {
					unanalysedRefs = append(unanalysedRefs, caseData.Reference)
				}
			} else {
				addr := caseData.Address
				if addr == "" {
					addr = "Unknown"
				}
				proposal := caseData.Proposal
				if proposal == "" {
					proposal = "No description available"
				}
				status := caseData.Status
				if status == "" {
					status = "Pending"
				}
				newCase := models.PlanningCase{
					Reference:     caseData.Reference,
					Address:       addr,
					Proposal:      proposal,
					Status:        status,
					ReceivedDate:  caseData.ReceivedDate,
					ValidatedDate: caseData.ValidatedDate,
				}
				db.Create(&newCase)
				newRefs = append(newRefs, caseData.Reference)
				casesAdded++
			}
		}

		// Queue AI analysis for new and unanalysed cases
		analysisRefs := append(newRefs, unanalysedRefs...)
		if len(analysisRefs) > 0 {
			enqueueClient := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr()})
			defer enqueueClient.Close()

			for _, ref := range analysisRefs {
				payload, _ := json.Marshal(map[string]string{"reference": ref})
				task := asynq.NewTask(TypeAnalysePlanningApplication, payload)
				if _, err := enqueueClient.Enqueue(task); err != nil {
					log.Printf("Failed to enqueue analysis for %s: %v", ref, err)
				}
			}

			log.Printf("Queued AI analysis for %d new and %d unanalysed existing cases",
				len(newRefs), len(unanalysedRefs))
		}

		log.Printf("Planning applications: %d added, %d updated for week beginning %s",
			casesAdded, casesUpdated, weekStart.Format("2006-01-02"))

		return nil
	}
}

func getPreviousWeekMonday(now time.Time) time.Time {
	daysSinceMonday := int(now.Weekday()) - 1
	if daysSinceMonday < 0 {
		daysSinceMonday = 6
	}
	thisMonday := now.AddDate(0, 0, -daysSinceMonday)
	previousMonday := thisMonday.AddDate(0, 0, -7)
	return time.Date(previousMonday.Year(), previousMonday.Month(), previousMonday.Day(), 0, 0, 0, 0, time.UTC)
}

func fetchAllPages(weekStart time.Time, client *http.Client) ([]string, error) {
	var allHTML []string

	// Fetch by validated date
	html, err := fetchWeeklyListPage(weekStart, client, "DC_Validated")
	if err != nil {
		return nil, err
	}
	allHTML = append(allHTML, html)
	allHTML = append(allHTML, fetchRemainingPages(html, client)...)

	// Also fetch by received date to catch applications not yet validated
	htmlReceived, err := fetchWeeklyListPage(weekStart, client, "DC_Received")
	if err != nil {
		log.Printf("Warning: failed to fetch by received date: %v", err)
	} else {
		allHTML = append(allHTML, htmlReceived)
		allHTML = append(allHTML, fetchRemainingPages(htmlReceived, client)...)
	}

	return allHTML, nil
}

func fetchRemainingPages(html string, client *http.Client) []string {
	var pages []string

	page := 2
	for {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			break
		}

		nextLink := doc.Find("a.next")
		if nextLink.Length() == 0 {
			break
		}

		pageURL := fmt.Sprintf("%s?action=page&searchCriteria.page=%d", pagedResultsURL, page)
		req, _ := http.NewRequest("GET", pageURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Referer", weeklyListResultsURL+"?action=firstPage")

		resp, err := client.Do(req)
		if err != nil {
			break
		}
		defer resp.Body.Close()

		doc2, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			break
		}
		html2, _ := doc2.Html()
		html = html2
		pages = append(pages, html2)
		page++

		if page > 50 {
			log.Println("Reached page limit of 50, stopping pagination")
			break
		}
	}

	return pages
}


func fetchWeeklyListPage(weekStart time.Time, client *http.Client, dateType string) (string, error) {
	// Step 1: GET the form page
	formURL := weeklyListURL + "?action=weeklyList"
	req, _ := http.NewRequest("GET", formURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to load form page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Step 2: Extract CSRF token
	csrfToken, _ := doc.Find("input[name=_csrf]").Attr("value")

	// Step 3: POST the search form
	formData := url.Values{
		"_csrf":                {csrfToken},
		"searchCriteria.ward": {""},
		"week":                {weekStart.Format("02 Jan 2006")},
		"dateType":            {dateType},
		"searchType":          {"Application"},
	}

	postReq, _ := http.NewRequest("POST", weeklyListResultsURL+"?action=firstPage",
		strings.NewReader(formData.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	postReq.Header.Set("Referer", formURL)
	postReq.Header.Set("Origin", "https://planning.plymouth.gov.uk")

	// Copy cookies from the GET response
	for _, cookie := range resp.Cookies() {
		postReq.AddCookie(cookie)
	}

	postResp, err := client.Do(postReq)
	if err != nil {
		return "", fmt.Errorf("failed to submit search form: %w", err)
	}
	defer postResp.Body.Close()

	resultDoc, err := goquery.NewDocumentFromReader(postResp.Body)
	if err != nil {
		return "", err
	}

	html, _ := resultDoc.Html()
	return html, nil
}

type parsedCase struct {
	Reference     string
	Address       string
	Proposal      string
	Status        string
	ReceivedDate  *time.Time
	ValidatedDate *time.Time
}

func parseResults(htmlPages []string) []parsedCase {
	var cases []parsedCase
	refRe := regexp.MustCompile(`Ref\.\s*No:\s*([\w/]+)`)
	recvRe := regexp.MustCompile(`Received:\s*\w+\s+(\d{1,2}\s+\w+\s+\d{4})`)
	valRe := regexp.MustCompile(`Validated:\s*\w+\s+(\d{1,2}\s+\w+\s+\d{4})`)
	statusRe := regexp.MustCompile(`Status:\s*(.+?)(?:\s*\||\s*$)`)

	for _, html := range htmlPages {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			continue
		}

		doc.Find("li.searchresult").Each(func(i int, s *goquery.Selection) {
			// Proposal
			proposal := ""
			summaryLink := s.Find("a.summaryLink")
			if summaryLink.Length() > 0 {
				div := summaryLink.Find("div")
				if div.Length() > 0 {
					proposal = strings.TrimSpace(div.Text())
				} else {
					proposal = strings.TrimSpace(summaryLink.Text())
				}
			}

			// Address
			address := strings.TrimSpace(s.Find("p.address").Text())

			// Meta info
			metaText := strings.TrimSpace(s.Find("p.metaInfo").Text())

			reference := ""
			if matches := refRe.FindStringSubmatch(metaText); len(matches) > 1 {
				reference = strings.TrimSpace(matches[1])
			}
			if reference == "" {
				return
			}

			var receivedDate *time.Time
			if matches := recvRe.FindStringSubmatch(metaText); len(matches) > 1 {
				receivedDate = parseDateUK(matches[1])
			}

			var validatedDate *time.Time
			if matches := valRe.FindStringSubmatch(metaText); len(matches) > 1 {
				validatedDate = parseDateUK(matches[1])
			}

			status := "Pending"
			if matches := statusRe.FindStringSubmatch(metaText); len(matches) > 1 {
				status = strings.TrimSpace(matches[1])
			}

			cases = append(cases, parsedCase{
				Reference:     reference,
				Address:       sanitizeUTF8(address),
				Proposal:      sanitizeUTF8(proposal),
				Status:        sanitizeUTF8(status),
				ReceivedDate:  receivedDate,
				ValidatedDate: validatedDate,
			})
		})
	}

	return cases
}

func parseDateUK(dateStr string) *time.Time {
	formats := []string{"2 Jan 2006", "02 Jan 2006", "2 January 2006", "02/01/2006"}
	for _, f := range formats {
		if t, err := time.Parse(f, dateStr); err == nil {
			return &t
		}
	}
	return nil
}

// sanitizeUTF8 replaces invalid UTF-8 bytes with their nearest valid equivalent.
// Specifically handles Latin-1/Windows-1252 bytes (like 0xa0 non-breaking space)
// that appear in HTML scraped from the planning portal.
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	// Replace byte-by-byte: valid UTF-8 sequences pass through,
	// invalid single bytes are treated as Latin-1 and converted to their
	// Unicode equivalents (e.g. 0xa0 -> U+00A0 non-breaking space -> regular space).
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			// Invalid byte — treat as Latin-1 code point
			cp := rune(s[i])
			if cp == 0xa0 {
				// Non-breaking space -> regular space
				b.WriteByte(' ')
			} else {
				// Write the Latin-1 character as its Unicode equivalent
				b.WriteRune(cp)
			}
			i++
		} else {
			b.WriteRune(r)
			i += size
		}
	}
	return b.String()
}
