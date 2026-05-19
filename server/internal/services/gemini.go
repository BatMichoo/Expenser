package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"expenser/internal/models"
	"fmt"
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

// AnalyzeReceipt takes a receipt image data and returns parsed information.
func (gs *GeminiService) AnalyzeReceipt(ctx context.Context, receiptData []byte) (*models.ReceiptAnalysis, error) {
	if gs.apiKey == "" {
		return nil, fmt.Errorf("gemini API key is not configured")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", gs.apiKey)
	mimeType := http.DetectContentType(receiptData)
	imageB64 := base64.StdEncoding.EncodeToString(receiptData)

	prompt := `Analyze the receipt image and return JSON strictly in the format {"product": "...", "quantity": 0.0, "price": 0.0, "supermarket_name": "..."}. Do not add any extra text or formatting.`

	requestBody := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": prompt},
					{
						"inline_data": map[string]any{
							"mime_type": mimeType,
							"data":      imageB64,
						},
					},
				},
			},
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

	return &analysis, nil
}
