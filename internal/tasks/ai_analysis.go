package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/smartplymouth/backend/internal/config"
)

type analysisResult struct {
	PotentialImpactScore int      `json:"potential_impact_score"`
	EstimatedSize        int      `json:"estimated_size"`
	Tags                 []string `json:"tags"`
	AIRationalisation    string   `json:"rationalisation"`
	Pros                 []string `json:"pros"`
	Cons                 []string `json:"cons"`
}

type objectionResult struct {
	Objection         string `json:"objection"`
	AIRationalisation string `json:"ai_rationalisation"`
}

type supportResult struct {
	SupportReason     string `json:"support_reason"`
	AIRationalisation string `json:"ai_rationalisation"`
}

func newOpenAIClient(cfg *config.Config) *openai.Client {
	clientCfg := openai.DefaultConfig(cfg.NscaleToken)
	clientCfg.BaseURL = cfg.NscaleBaseURL
	return openai.NewClientWithConfig(clientCfg)
}

func runAIAnalysis(metadata map[string]string, documentTexts []string, reference string, cfg *config.Config) *analysisResult {
	client := newOpenAIClient(cfg)

	metadataText := formatMetadata(metadata)

	// Truncate documents
	combinedDocs := strings.Join(limitSlice(documentTexts, 10), "\n\n---\n\n")
	if len(combinedDocs) > 15000 {
		combinedDocs = combinedDocs[:15000] + "\n\n[... truncated ...]"
	}
	if combinedDocs == "" {
		combinedDocs = "No documents available."
	}

	systemPrompt := `You are an expert planning analyst. Analyse the following planning application and provide a structured assessment. You must respond with ONLY valid JSON, no other text or explanation.

The JSON must have exactly these fields:
- potential_impact_score: integer 1-10 (1=minimal impact, 10=transformative/major impact)
- estimated_size: integer 1-10 (1=very small e.g. minor alteration, 10=massive e.g. large housing estate)
- tags: array of lowercase string tags describing the application (e.g. residential, commercial, change-of-use, demolition, new-build, extension, listed-building, conservation-area, HMO, retail, industrial, infrastructure)
- rationalisation: a string containing several paragraphs explaining WHY you chose the given impact score and size score
- pros: an array of strings listing the positive aspects and benefits of the application
- cons: an array of strings listing the negative aspects and drawbacks of the application

## Scoring Rules:
### Minor homeowner changes: potential_impact_score MUST be 1 or 2, estimated_size MUST be 1 or 2
### Tree works: potential_impact_score MUST be 1 or 2, estimated_size MUST be 1 or 2
### Single new dwelling: potential_impact_score MUST NOT exceed 3, estimated_size MUST NOT exceed 3. HMOs are NOT single dwellings.
### Larger/complex applications: minimum 2 or 3, no upper cap

## GROUNDING RULE: ONLY make claims about things explicitly stated in the application metadata and documents provided.`

	userMessage := fmt.Sprintf("Planning Application Reference: %s\n\n## Application Metadata\n%s\n\n## Document Contents\n%s\n\nProvide your analysis as JSON only.",
		reference, metadataText, combinedDocs)

	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model:       cfg.LLMModel,
		Temperature: 0.1,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
	})
	if err != nil {
		log.Printf("AI analysis request failed for %s: %v", reference, err)
		return nil
	}

	responseText := strings.TrimSpace(resp.Choices[0].Message.Content)
	responseText = cleanJSONResponse(responseText)

	var result analysisResult
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		log.Printf("Failed to parse AI response for %s: %v", reference, err)
		return nil
	}

	// Clamp values
	result.PotentialImpactScore = clamp(result.PotentialImpactScore, 1, 10)
	result.EstimatedSize = clamp(result.EstimatedSize, 1, 10)

	// Clean tags
	var cleanTags []string
	for _, t := range result.Tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" {
			cleanTags = append(cleanTags, t)
		}
	}
	result.Tags = cleanTags

	return &result
}

func generateObjections(metadata map[string]string, documentTexts []string, reference string, analysis *analysisResult, cfg *config.Config) []objectionResult {
	client := newOpenAIClient(cfg)

	metadataText := formatMetadata(metadata)
	combinedDocs := strings.Join(limitSlice(documentTexts, 10), "\n\n---\n\n")
	if len(combinedDocs) > 15000 {
		combinedDocs = combinedDocs[:15000] + "\n\n[... truncated ...]"
	}
	if combinedDocs == "" {
		combinedDocs = "No documents available."
	}

	systemPrompt := `You are an expert UK planning consultant. Your job is to identify MULTIPLE SEPARATE grounds for objection to a planning application.

Each objection must address a DIFFERENT planning concern specific to THIS application.

## GROUNDING RULE: ONLY make claims about things explicitly stated in the application metadata and documents provided.

## JSON OUTPUT FORMAT:
Respond with a JSON object containing a single key "objections" whose value is an array. Each element has:
- "objection": a short one-sentence summary of the ground for objection
- "ai_rationalisation": 1-2 paragraphs explaining why this is a valid objection

## WHAT NOT TO DO:
- Do NOT combine multiple concerns into one objection
- Do NOT write generic objections
- Do NOT include objections based on property values, business competition, personal disputes, or loss of private views`

	userMessage := fmt.Sprintf(`Planning Application Reference: %s

## AI Assessment
- Impact Score: %d/10
- Size Score: %d/10
- Tags: %s

## Application Metadata
%s

## Document Contents
%s

Identify ALL legitimate grounds for objection. Return JSON with key "objections" containing the array.`,
		reference, analysis.PotentialImpactScore, analysis.EstimatedSize,
		strings.Join(analysis.Tags, ", "), metadataText, combinedDocs)

	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model:       cfg.LLMModel,
		Temperature: 0.1,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
	})
	if err != nil {
		log.Printf("Objections generation failed for %s: %v", reference, err)
		return nil
	}

	responseText := strings.TrimSpace(resp.Choices[0].Message.Content)
	responseText = cleanJSONResponse(responseText)

	var wrapper struct {
		Objections []objectionResult `json:"objections"`
	}
	if err := json.Unmarshal([]byte(responseText), &wrapper); err != nil {
		// Try as raw array
		var arr []objectionResult
		if err2 := json.Unmarshal([]byte(responseText), &arr); err2 != nil {
			log.Printf("Failed to parse objections for %s: %v", reference, err)
			return nil
		}
		return arr
	}
	return wrapper.Objections
}

func generateSupports(metadata map[string]string, documentTexts []string, reference string, analysis *analysisResult, cfg *config.Config) []supportResult {
	client := newOpenAIClient(cfg)

	metadataText := formatMetadata(metadata)
	combinedDocs := strings.Join(limitSlice(documentTexts, 10), "\n\n---\n\n")
	if len(combinedDocs) > 15000 {
		combinedDocs = combinedDocs[:15000] + "\n\n[... truncated ...]"
	}
	if combinedDocs == "" {
		combinedDocs = "No documents available."
	}

	systemPrompt := `You are an expert UK planning consultant. Your job is to identify MULTIPLE SEPARATE grounds for SUPPORT of a planning application.

Each reason for support must address a DIFFERENT planning benefit specific to THIS application.

## GROUNDING RULE: ONLY make claims about things explicitly stated in the application metadata and documents provided.

## JSON OUTPUT FORMAT:
Respond with a JSON object containing a single key "supports" whose value is an array. Each element has:
- "support_reason": a short one-sentence summary of the ground for support
- "ai_rationalisation": 1-2 paragraphs explaining why this is a valid reason for support

## WHAT NOT TO DO:
- Do NOT combine multiple benefits into one entry
- Do NOT write generic reasons
- Do NOT include reasons based on personal benefit, property values, or unsubstantiated claims`

	userMessage := fmt.Sprintf(`Planning Application Reference: %s

## AI Assessment
- Impact Score: %d/10
- Size Score: %d/10
- Tags: %s

## Application Metadata
%s

## Document Contents
%s

Identify ALL legitimate grounds for support. Return JSON with key "supports" containing the array.`,
		reference, analysis.PotentialImpactScore, analysis.EstimatedSize,
		strings.Join(analysis.Tags, ", "), metadataText, combinedDocs)

	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model:       cfg.LLMModel,
		Temperature: 0.1,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
	})
	if err != nil {
		log.Printf("Supports generation failed for %s: %v", reference, err)
		return nil
	}

	responseText := strings.TrimSpace(resp.Choices[0].Message.Content)
	responseText = cleanJSONResponse(responseText)

	var wrapper struct {
		Supports []supportResult `json:"supports"`
	}
	if err := json.Unmarshal([]byte(responseText), &wrapper); err != nil {
		var arr []supportResult
		if err2 := json.Unmarshal([]byte(responseText), &arr); err2 != nil {
			log.Printf("Failed to parse supports for %s: %v", reference, err)
			return nil
		}
		return arr
	}
	return wrapper.Supports
}

// Helper functions

func formatMetadata(metadata map[string]string) string {
	var lines []string
	for k, v := range metadata {
		lines = append(lines, fmt.Sprintf("- %s: %s", k, v))
	}
	return strings.Join(lines, "\n")
}

func cleanJSONResponse(text string) string {
	// Remove markdown code fences
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		var cleaned []string
		for _, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), "```") {
				cleaned = append(cleaned, line)
			}
		}
		text = strings.Join(cleaned, "\n")
	}

	// Remove <think> tags
	if idx := strings.Index(text, "</think>"); idx != -1 {
		text = strings.TrimSpace(text[idx+len("</think>"):])
	}

	// Extract JSON object or array
	jsonStart := strings.Index(text, "{")
	jsonEnd := strings.LastIndex(text, "}")
	if jsonStart != -1 && jsonEnd > jsonStart {
		return text[jsonStart : jsonEnd+1]
	}

	arrStart := strings.Index(text, "[")
	arrEnd := strings.LastIndex(text, "]")
	if arrStart != -1 && arrEnd > arrStart {
		return text[arrStart : arrEnd+1]
	}

	return text
}

func limitSlice(s []string, max int) []string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func clamp(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
