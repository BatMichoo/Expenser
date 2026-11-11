package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"expenser/internal/models"
	"fmt"
	"log"
	"net/http"
	"regexp"
)

// GeminiService handles interactions with the Gemini API.
type GeminiService struct {
	apiKey string
}

// NewGeminiService creates a new instance of GeminiService.
func NewGeminiService(apiKey string) *GeminiService {
	return &GeminiService{
		apiKey: apiKey,
	}
}

// AnalyzeReceipt reads a receipt (a photo or a PDF) and returns its items.
func (gs *GeminiService) AnalyzeReceipt(ctx context.Context, receiptData []byte) (*models.ReceiptAnalysis, error) {
	if gs.apiKey == "" {
		return nil, fmt.Errorf("gemini API key is not configured")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", gs.apiKey)
	requestBody := map[string]any{
		"contents": []map[string]any{
			{"parts": receiptParts(ctx, receiptData, pdfToText)},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var geminiResp struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, err
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("gemini api error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no response from gemini")
	}

	text := geminiResp.Candidates[0].Content.Parts[0].Text
	fmt.Printf("Raw Gemini response: %s\n", text)

	// Clean text (remove ```json and ``` if present)
	re := regexp.MustCompile("(?s)```json\\s*(.*?)\\s*```")
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		text = match[1]
	}

	var analysis models.ReceiptAnalysis
	if err := json.Unmarshal([]byte(text), &analysis); err != nil {
		return nil, err
	}

	// Discounts printed apart from their items (Kaufland's "Позиции") are put
	// on those items here, in exact cents, instead of trusting the model's
	// arithmetic.
	for _, d := range analysis.ApplyDiscounts() {
		log.Printf("receipt discount not matched to items: %q %.2f positions %v", d.Description, d.Amount, d.Positions)
	}

	return &analysis, nil
}
