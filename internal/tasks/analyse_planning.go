package tasks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/hibiken/asynq"
	"github.com/ledongthuc/pdf"
	"gorm.io/gorm"

	"github.com/smartplymouth/backend/internal/config"
	"github.com/smartplymouth/backend/internal/models"
)

const planningAppBaseURL = "https://planning.plymouth.gov.uk/online-applications/"

type analysePayload struct {
	Reference string `json:"reference"`
}

func NewAnalysePlanningHandler(db *gorm.DB, cfg *config.Config) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload analysePayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		reference := payload.Reference

		var planCase models.PlanningCase
		if err := db.Where("reference = ?", reference).First(&planCase).Error; err != nil {
			return fmt.Errorf("case %s not found: %w", reference, err)
		}

		tmpDir, err := os.MkdirTemp("", "smartplymouth_planning_")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		log.Printf("Analysing planning application %s (working dir: %s)", reference, tmpDir)

		httpClient := &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		// Step 1: Find the application's internal key
		keyVal, err := getApplicationKeyVal(reference, httpClient)
		if err != nil || keyVal == "" {
			return fmt.Errorf("could not find keyVal for application %s: %v", reference, err)
		}

		// Step 2: Collect metadata
		log.Printf("Collecting metadata for %s", reference)
		metadata := collectApplicationMetadata(keyVal, httpClient)

		// Step 3: Download documents
		log.Printf("Downloading documents for %s", reference)
		downloadDir := filepath.Join(tmpDir, "documents")
		os.MkdirAll(downloadDir, 0755)
		documents := downloadDocuments(keyVal, httpClient, downloadDir)
		log.Printf("Downloaded %d documents for %s", len(documents), reference)

		// Step 4: Extract text from PDFs
		var documentTexts []string
		for _, doc := range documents {
			if strings.HasSuffix(strings.ToLower(doc.Path), ".pdf") {
				text := extractTextFromPDF(doc.Path)
				if text != "" {
					documentTexts = append(documentTexts, fmt.Sprintf("[%s]\n%s", doc.Filename, text))
				}
			}
		}

		if planCase.Proposal != "" {
			documentTexts = append([]string{fmt.Sprintf("[Application Proposal]\n%s", planCase.Proposal)}, documentTexts...)
		}

		// Step 5: Run AI analysis
		log.Printf("Running AI analysis for %s", reference)
		analysis := runAIAnalysis(metadata, documentTexts, reference, cfg)
		if analysis == nil {
			return fmt.Errorf("AI analysis failed to produce valid results for %s", reference)
		}

		// Step 6: Update database
		planCase.AIAnalysis = true
		planCase.PotentialImpactScore = &analysis.PotentialImpactScore
		planCase.EstimatedSize = &analysis.EstimatedSize
		planCase.Tags = analysis.Tags
		planCase.AIRationalisation = &analysis.AIRationalisation
		planCase.Pros = analysis.Pros
		planCase.Cons = analysis.Cons
		db.Save(&planCase)

		// Step 7: Generate objections
		log.Printf("Generating potential objections for %s", reference)
		db.Where("case_reference = ?", reference).Delete(&models.PlanningObjection{})

		objections := generateObjections(metadata, documentTexts, reference, analysis, cfg)
		for _, obj := range objections {
			db.Create(&models.PlanningObjection{
				CaseReference:     reference,
				Objection:         obj.Objection,
				AIRationalisation: obj.AIRationalisation,
			})
		}
		log.Printf("Generated %d potential objections for %s", len(objections), reference)

		// Step 8: Generate supports
		log.Printf("Generating potential reasons for support for %s", reference)
		db.Where("case_reference = ?", reference).Delete(&models.PlanningSupport{})

		supports := generateSupports(metadata, documentTexts, reference, analysis, cfg)
		for _, sup := range supports {
			db.Create(&models.PlanningSupport{
				CaseReference:     reference,
				SupportReason:     sup.SupportReason,
				AIRationalisation: sup.AIRationalisation,
			})
		}
		log.Printf("Generated %d potential reasons for support for %s", len(supports), reference)

		log.Printf("Analysis complete for %s: impact=%d, size=%d, tags=%v",
			reference, analysis.PotentialImpactScore, analysis.EstimatedSize, analysis.Tags)

		return nil
	}
}

func getApplicationKeyVal(reference string, client *http.Client) (string, error) {
	formURL := planningAppBaseURL + "search.do?action=simple"
	req, _ := http.NewRequest("GET", formURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	csrfToken, _ := doc.Find("input[name=_csrf]").Attr("value")

	formData := url.Values{
		"_csrf":                                  {csrfToken},
		"searchCriteria.reference":               {reference},
		"searchCriteria.planningPortalReference": {""},
		"searchCriteria.alternativeReference":    {""},
		"searchType":                             {"Application"},
	}

	postReq, _ := http.NewRequest("POST",
		planningAppBaseURL+"simpleSearchResults.do?action=firstPage",
		strings.NewReader(formData.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	postReq.Header.Set("Referer", formURL)
	postReq.Header.Set("Origin", "https://planning.plymouth.gov.uk")
	for _, cookie := range resp.Cookies() {
		postReq.AddCookie(cookie)
	}

	postResp, err := client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	keyValRe := regexp.MustCompile(`keyVal=([A-Z0-9_]+)`)

	if matches := keyValRe.FindStringSubmatch(postResp.Request.URL.String()); len(matches) > 1 {
		return matches[1], nil
	}

	resultDoc, err := goquery.NewDocumentFromReader(postResp.Body)
	if err != nil {
		return "", err
	}

	var keyVal string
	resultDoc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || keyVal != "" {
			return
		}
		if matches := keyValRe.FindStringSubmatch(href); len(matches) > 1 {
			keyVal = matches[1]
		}
	})

	return keyVal, nil
}

func collectApplicationMetadata(keyVal string, client *http.Client) map[string]string {
	summaryURL := fmt.Sprintf("%sapplicationDetails.do?activeTab=summary&keyVal=%s", planningAppBaseURL, keyVal)
	req, _ := http.NewRequest("GET", summaryURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil
	}

	metadata := make(map[string]string)
	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		header := strings.TrimSpace(s.Find("th").Text())
		value := strings.TrimSpace(s.Find("td").Text())
		header = strings.TrimRight(header, ":")
		if header != "" && value != "" {
			metadata[header] = value
		}
	})

	return metadata
}

type downloadedDoc struct {
	Filename string
	Path     string
	URL      string
}

func downloadDocuments(keyVal string, client *http.Client, downloadDir string) []downloadedDoc {
	docsURL := fmt.Sprintf("%sapplicationDetails.do?activeTab=documents&keyVal=%s", planningAppBaseURL, keyVal)
	req, _ := http.NewRequest("GET", docsURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil
	}

	var downloaded []downloadedDoc
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		if !strings.Contains(href, "/files/") && !strings.Contains(href, "/document/") {
			return
		}

		docURL := href
		if !strings.HasPrefix(href, "http") {
			docURL = "https://planning.plymouth.gov.uk" + href
		}

		filename := strings.TrimSpace(s.Text())
		if filename == "" {
			filename = filepath.Base(href)
		}
		sanitized := sanitizeFilename(filename)
		if sanitized == "" {
			sanitized = fmt.Sprintf("document_%d", len(downloaded))
		}

		docReq, _ := http.NewRequest("GET", docURL, nil)
		docReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		docResp, err := client.Do(docReq)
		if err != nil {
			log.Printf("Failed to download document %s: %v", docURL, err)
			return
		}
		defer docResp.Body.Close()

		if !strings.Contains(sanitized, ".") {
			ct := docResp.Header.Get("Content-Type")
			switch {
			case strings.Contains(ct, "pdf"):
				sanitized += ".pdf"
			case strings.Contains(ct, "image"):
				sanitized += ".png"
			default:
				sanitized += ".bin"
			}
		}

		fpath := filepath.Join(downloadDir, sanitized)
		f, err := os.Create(fpath)
		if err != nil {
			return
		}
		io.Copy(f, docResp.Body)
		f.Close()

		downloaded = append(downloaded, downloadedDoc{
			Filename: sanitized,
			Path:     fpath,
			URL:      docURL,
		})
	})

	return downloaded
}

func sanitizeFilename(name string) string {
	var result strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == ' ' || c == '.' || c == '_' || c == '-' {
			result.WriteRune(c)
		}
	}
	return strings.TrimSpace(result.String())
}

func extractTextFromPDF(fpath string) string {
	f, err := os.Open(fpath)
	if err != nil {
		return ""
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return ""
	}

	reader, err := pdf.NewReader(f, stat.Size())
	if err != nil {
		log.Printf("Failed to read PDF %s: %v", fpath, err)
		return ""
	}

	var text strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		content, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		text.WriteString(content)
		text.WriteString("\n")
	}
	return text.String()
}
