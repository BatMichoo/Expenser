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

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", gs.apiKey)
	mimeType := http.DetectContentType(receiptData)
	imageB64 := base64.StdEncoding.EncodeToString(receiptData)

	prompt := `Extract the data from this receipt image and output it as a structured JSON object.

	Crucial layout instruction for this specific receipt format: The multiplier line containing the quantity and unit price (e.g., 2.000 x 1.78) applies to the item printed on the line immediately below it, not the item above it. The total price for that item appears on the same line as the item name. If an item does not have a multiplier line preceding it, assume a quantity of 1 and that the unit price equals the total line price.

	Categorization instruction: For each item in "items", assign a category in the "category" field. The value MUST be one of the following exact string values, matching case and spelling:
	"Tomatoes", "Cucumbers", "Milk", "Pork", "Beef", "Chicken", "Eggs", "Cheese", "Bread", "Potatoes", "Apples", "Bananas", "Other".
	If an item does not clearly fit into the specific food items listed, assign "Other". Do not use any other category names.

	Use the following JSON schema:
	{
	  "store": "string",
	  "address": "string",
	  "vat_number": "string",
	  "items": [
	    {
	      "name": "string",
	      "quantity": number,
	      "unit_price": number,
	      "total_price": number,
	      "category": "string"
	    }
	  ],
	  "discounts": [
	    {
	      "description": "string",
	      "amount": number
	    }
	  ],
	  "totals": {
	    "total_eur": number,
	    "total_bgn": number,
	    "exchange_rate": number
	  }
	}
	Return only the valid, parsed JSON object without any additional formatting or markdown blocks.
	`

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

	return &analysis, nil
}
