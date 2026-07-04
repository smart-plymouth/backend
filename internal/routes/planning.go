package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	openai "github.com/sashabaranov/go-openai"
	"gorm.io/gorm"

	"github.com/smartplymouth/backend/internal/config"
	"github.com/smartplymouth/backend/internal/models"
	"github.com/smartplymouth/backend/internal/taskqueue"
	"github.com/smartplymouth/backend/internal/tasks"

	"github.com/hibiken/asynq"
)

func RegisterPlanning(r *chi.Mux, db *gorm.DB, client *taskqueue.Client, cfg *config.Config) {
	r.Route("/api/planning/v1.0", func(r chi.Router) {
		r.Get("/cases", listCases(db))
		r.Get("/cases/*", func(w http.ResponseWriter, r *http.Request) {
			// Handle sub-routes with reference containing slashes
			path := chi.URLParam(r, "*")
			parts := strings.Split(path, "/")

			// Determine if this is /cases/{ref} or /cases/{ref}/action
			if len(parts) >= 3 {
				// /cases/XX/XXXXX/XXX/action
				lastPart := parts[len(parts)-1]
				switch lastPart {
				case "analyse":
					reference := strings.Join(parts[:len(parts)-1], "/")
					triggerAnalysis(db, client, reference, w, r)
					return
				case "objections":
					reference := strings.Join(parts[:len(parts)-1], "/")
					listObjections(db, reference, w)
					return
				case "supports":
					reference := strings.Join(parts[:len(parts)-1], "/")
					listSupports(db, reference, w)
					return
				case "generate-letter":
					reference := strings.Join(parts[:len(parts)-1], "/")
					generateLetter(db, cfg, reference, w, r)
					return
				}
			}

			// Otherwise it's just /cases/{reference}
			reference := path
			getCase(db, reference, w)
		})
		r.Post("/refresh", triggerRefresh(client))
		r.Post("/cases/*", func(w http.ResponseWriter, r *http.Request) {
			path := chi.URLParam(r, "*")
			parts := strings.Split(path, "/")

			if len(parts) >= 3 {
				lastPart := parts[len(parts)-1]
				reference := strings.Join(parts[:len(parts)-1], "/")
				switch lastPart {
				case "analyse":
					triggerAnalysis(db, client, reference, w, r)
					return
				case "generate-letter":
					generateLetter(db, cfg, reference, w, r)
					return
				}
			}

			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		})
	})
}

func listCases(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")
		status := r.URL.Query().Get("status")
		validatedDate := r.URL.Query().Get("validated_date")
		validatedFrom := r.URL.Query().Get("validated_from")
		validatedTo := r.URL.Query().Get("validated_to")
		page := queryInt(r, "page", 1)
		perPage := queryInt(r, "per_page", 25)
		if perPage > 100 {
			perPage = 100
		}

		query := db.Model(&models.PlanningCase{})

		if search != "" {
			like := "%" + search + "%"
			query = query.Where(
				"reference ILIKE ? OR proposal ILIKE ? OR address ILIKE ? OR CAST(received_date AS TEXT) ILIKE ? OR CAST(validated_date AS TEXT) ILIKE ? OR CAST(tags AS TEXT) ILIKE ?",
				like, like, like, like, like, like,
			)
		}

		if status != "" {
			query = query.Where("status ILIKE ?", "%"+status+"%")
		}

		if validatedDate != "" {
			parsed, err := time.Parse("2006-01-02", validatedDate)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid validated_date format. Use YYYY-MM-DD."})
				return
			}
			query = query.Where("validated_date = ?", parsed)
		} else {
			if validatedFrom != "" {
				parsed, err := time.Parse("2006-01-02", validatedFrom)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid validated_from format. Use YYYY-MM-DD."})
					return
				}
				query = query.Where("validated_date >= ?", parsed)
			}
			if validatedTo != "" {
				parsed, err := time.Parse("2006-01-02", validatedTo)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid validated_to format. Use YYYY-MM-DD."})
					return
				}
				query = query.Where("validated_date <= ?", parsed)
			}
		}

		var total int64
		query.Count(&total)

		var cases []models.PlanningCase
		offset := (page - 1) * perPage
		query.Order("validated_date DESC").Offset(offset).Limit(perPage).Find(&cases)

		pages := int(math.Ceil(float64(total) / float64(perPage)))

		caseDicts := make([]map[string]interface{}, 0, len(cases))
		for _, c := range cases {
			caseDicts = append(caseDicts, c.ToDict())
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"cases":    caseDicts,
			"total":    total,
			"page":     page,
			"pages":    pages,
			"per_page": perPage,
		})
	}
}

func getCase(db *gorm.DB, reference string, w http.ResponseWriter) {
	var planCase models.PlanningCase
	if err := db.Where("reference = ?", reference).First(&planCase).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Case not found"})
		return
	}
	writeJSON(w, http.StatusOK, planCase.ToDict())
}

func triggerAnalysis(db *gorm.DB, client *taskqueue.Client, reference string, w http.ResponseWriter, r *http.Request) {
	var planCase models.PlanningCase
	if err := db.Where("reference = ?", reference).First(&planCase).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Case not found"})
		return
	}

	payload, _ := json.Marshal(map[string]string{"reference": reference})
	task := asynq.NewTask(tasks.TypeAnalysePlanningApplication, payload)
	info, err := client.Enqueue(task)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to enqueue task"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":    "accepted",
		"task_id":   info.ID,
		"reference": reference,
	})
}

func listObjections(db *gorm.DB, reference string, w http.ResponseWriter) {
	var planCase models.PlanningCase
	if err := db.Where("reference = ?", reference).First(&planCase).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Case not found"})
		return
	}

	var objections []models.PlanningObjection
	db.Where("case_reference = ?", reference).Order("created_at DESC").Find(&objections)

	objDicts := make([]map[string]interface{}, 0, len(objections))
	for _, obj := range objections {
		objDicts = append(objDicts, map[string]interface{}{
			"id":                 obj.ID,
			"case_reference":     obj.CaseReference,
			"objection":          obj.Objection,
			"ai_rationalisation": obj.AIRationalisation,
			"created_at":         obj.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reference":  reference,
		"objections": objDicts,
	})
}

func listSupports(db *gorm.DB, reference string, w http.ResponseWriter) {
	var planCase models.PlanningCase
	if err := db.Where("reference = ?", reference).First(&planCase).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Case not found"})
		return
	}

	var supports []models.PlanningSupport
	db.Where("case_reference = ?", reference).Order("created_at DESC").Find(&supports)

	supDicts := make([]map[string]interface{}, 0, len(supports))
	for _, s := range supports {
		supDicts = append(supDicts, map[string]interface{}{
			"id":                 s.ID,
			"case_reference":     s.CaseReference,
			"support_reason":     s.SupportReason,
			"ai_rationalisation": s.AIRationalisation,
			"created_at":         s.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reference": reference,
		"supports":  supDicts,
	})
}

func triggerRefresh(client *taskqueue.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task := asynq.NewTask(tasks.TypeRefreshPlanningApplications, nil)
		info, err := client.Enqueue(task)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to enqueue task"})
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"status":  "accepted",
			"task_id": info.ID,
		})
	}
}

func generateLetter(db *gorm.DB, cfg *config.Config, reference string, w http.ResponseWriter, r *http.Request) {
	var planCase models.PlanningCase
	if err := db.Where("reference = ?", reference).First(&planCase).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Case not found"})
		return
	}

	var reqBody struct {
		FirstName  string `json:"first_name"`
		LastName   string `json:"last_name"`
		LetterType string `json:"letter_type"`
	}

	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &reqBody); err != nil || len(body) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Request body must be JSON"})
		return
	}

	if reqBody.FirstName == "" || reqBody.LastName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "first_name and last_name are required"})
		return
	}

	if reqBody.LetterType != "objection" && reqBody.LetterType != "support" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "letter_type must be 'objection' or 'support'"})
		return
	}

	var reasonsText string
	if reqBody.LetterType == "objection" {
		var objections []models.PlanningObjection
		db.Where("case_reference = ?", reference).Order("created_at DESC").Find(&objections)
		if len(objections) == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "No objection reasons available for this case. Run AI analysis first."})
			return
		}
		var parts []string
		for _, obj := range objections {
			parts = append(parts, fmt.Sprintf("**%s**\n%s", obj.Objection, obj.AIRationalisation))
		}
		reasonsText = strings.Join(parts, "\n\n")
	} else {
		var supports []models.PlanningSupport
		db.Where("case_reference = ?", reference).Order("created_at DESC").Find(&supports)
		if len(supports) == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "No support reasons available for this case. Run AI analysis first."})
			return
		}
		var parts []string
		for _, s := range supports {
			parts = append(parts, fmt.Sprintf("**%s**\n%s", s.SupportReason, s.AIRationalisation))
		}
		reasonsText = strings.Join(parts, "\n\n")
	}

	letterInstruction := ""
	if reqBody.LetterType == "objection" {
		letterInstruction = "Write a formal letter of objection to the planning authority regarding the planning application described below. The letter should clearly state the grounds for objection, referencing relevant planning policy where appropriate. The tone should be firm but polite and professional."
	} else {
		letterInstruction = "Write a formal letter of support to the planning authority regarding the planning application described below. The letter should clearly state the reasons for support, referencing relevant planning policy where appropriate. The tone should be positive, constructive and professional."
	}

	systemPrompt := `You are an expert UK planning consultant who drafts formal letters to planning authorities on behalf of members of the public.

Rules:
- Write in formal letter format with appropriate structure
- Format the entire letter in Markdown
- Use **bold** for the recipient name and application reference
- Use headings (##) for major sections where appropriate
- Address it to:
  Planning Department
  Floor 2, Ballard House
  West Hoe Road
  Plymouth
  PL1 3BJ
- Use this EXACT address — do NOT make up or alter it
- Do NOT include a placeholder or space for the sender's address
- Use today's actual date (provided below) — do NOT use a placeholder
- Reference the planning application number clearly
- Use the reasons provided as the basis for the letter
- Cite specific planning policies where mentioned in the reasons
- Keep the language accessible but professional
- Sign off with the author's name
- Do NOT invent additional reasons beyond those provided
- ONLY make claims about things explicitly mentioned in the application details and reasons provided.`

	todayStr := time.Now().Format("2 January 2006")
	userMessage := fmt.Sprintf(`%s

## Application Details
- Reference: %s
- Address: %s
- Proposal: %s

## Today's Date
%s

## Reasons
%s

## Author
- Name: %s %s

Generate the letter now.`, letterInstruction, reference, planCase.Address, planCase.Proposal,
		todayStr, reasonsText, reqBody.FirstName, reqBody.LastName)

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Streaming not supported"})
		return
	}

	clientCfg := openai.DefaultConfig(cfg.NscaleToken)
	clientCfg.BaseURL = cfg.NscaleBaseURL
	aiClient := openai.NewClientWithConfig(clientCfg)

	stream, err := aiClient.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{
		Model:       cfg.LLMModel,
		Temperature: 0.3,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
		Stream: true,
	})
	if err != nil {
		errData, _ := json.Marshal(map[string]string{"error": fmt.Sprintf("Failed to generate letter: %v", err)})
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", errData)
		flusher.Flush()
		return
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			errData, _ := json.Marshal(map[string]string{"error": fmt.Sprintf("Stream error: %v", err)})
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", errData)
			flusher.Flush()
			return
		}

		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
			token := response.Choices[0].Delta.Content
			tokenJSON, _ := json.Marshal(token)
			fmt.Fprintf(w, "event: token\ndata: %s\n\n", tokenJSON)
			flusher.Flush()
		}
	}

	doneData, _ := json.Marshal(map[string]interface{}{
		"reference":   reference,
		"letter_type": reqBody.LetterType,
		"author":      reqBody.FirstName + " " + reqBody.LastName,
	})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneData)
	flusher.Flush()
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	var n int
	fmt.Sscanf(val, "%d", &n)
	if n <= 0 {
		return defaultVal
	}
	return n
}
