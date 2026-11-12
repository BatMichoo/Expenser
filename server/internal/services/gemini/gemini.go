package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"expenser/internal/models"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// geminiAPI is the Gemini generateContent endpoint for a model.
const geminiAPI = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

// ErrNotConfigured means there is no Gemini API key.
var ErrNotConfigured = errors.New("gemini API key is not configured")

// APIError is an error returned by the Gemini API itself, e.g. quota
// exhausted or the model overloaded. Message is Google's own wording.
type APIError struct {
	Code    int    // HTTP status
	Status  string // e.g. RESOURCE_EXHAUSTED, UNAVAILABLE
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("gemini api error %d %s: %s", e.Code, e.Status, e.Message)
}

// Unavailable reports a temporary outage worth retrying later.
func (e *APIError) Unavailable() bool {
	return e.Code == http.StatusServiceUnavailable || e.Status == "UNAVAILABLE"
}

// ReplyError means the model answered, but not with a receipt: Reply is
// what it said instead (e.g. "the text is too small to be read"), shortened;
// FinishReason is why it stopped when that wasn't normal (MAX_TOKENS when
// the answer got cut off, SAFETY when it was blocked).
type ReplyError struct {
	Reply        string
	FinishReason string
	Err          error
}

func (e *ReplyError) Error() string {
	return fmt.Sprintf("gemini reply is not a receipt (finish %s): %q: %v", e.FinishReason, e.Reply, e.Err)
}

func (e *ReplyError) Unwrap() error { return e.Err }

// maxReplyRunes caps how much of the model's words reach the user.
const maxReplyRunes = 300

// shortReply collapses whitespace and trims the model's words for display.
func shortReply(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxReplyRunes {
		return string(r[:maxReplyRunes]) + "…"
	}
	return s
}

// When Gemini is overloaded (503) the read is retried: every model in turn,
// then again after each wait. Receipts are read in the background, so the
// waits cost the user nothing but time on the "Reading…" panel.
var (
	defaultRetryWaits     = []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
	defaultAttemptTimeout = 90 * time.Second // one model that hangs mustn't use up the job
)

// GeminiService handles interactions with the Gemini API.
type GeminiService struct {
	apiKey   string
	models   []string // the main model first, then fallbacks
	endpoint string   // URL with %s for the model
	client   *http.Client

	waits          []time.Duration
	attemptTimeout time.Duration
	sleep          func(ctx context.Context, d time.Duration) error
}

// NewGeminiService reads receipts with the given models: the first one
// (config.ReceiptAIModel), falling back to the rest when it's overloaded.
func NewGeminiService(apiKey string, models ...string) *GeminiService {
	return &GeminiService{
		apiKey: apiKey, models: models, endpoint: geminiAPI, client: http.DefaultClient,
		waits: defaultRetryWaits, attemptTimeout: defaultAttemptTimeout, sleep: sleepCtx,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type geminiResponse struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

var jsonFence = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

// AnalyzeReceipt reads a receipt (a photo or a PDF) and returns its items.
// Failures come back as ErrNotConfigured, *APIError (Gemini refused) or
// *ReplyError (Gemini answered, but not with a receipt), so callers can
// tell the user what actually went wrong.
func (gs *GeminiService) AnalyzeReceipt(ctx context.Context, receiptData []byte) (*models.ReceiptAnalysis, error) {
	if gs.apiKey == "" || len(gs.models) == 0 {
		return nil, ErrNotConfigured
	}
	jsonBody, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{"parts": receiptParts(ctx, receiptData, pdfToText)},
		},
	})
	if err != nil {
		return nil, err
	}

	reply, finish, err := gs.generate(ctx, jsonBody)
	if err != nil {
		return nil, err
	}
	var analysis models.ReceiptAnalysis
	if err := json.Unmarshal([]byte(reply), &analysis); err != nil {
		return nil, badReply(reply, finish, err)
	}

	// Discounts printed apart from their items (Kaufland's "Позиции") are put
	// on those items here, in exact cents, instead of trusting the model's
	// arithmetic.
	for _, d := range analysis.ApplyDiscounts() {
		log.Printf("receipt discount not matched to items: %q %.2f positions %v", d.Description, d.Amount, d.Positions)
	}
	return &analysis, nil
}

// badReply is the error for a model answer that isn't the JSON asked for.
func badReply(reply, finish string, err error) *ReplyError {
	replyErr := &ReplyError{Err: err}
	if finish != "" && finish != "STOP" {
		replyErr.FinishReason = finish
	}
	// Words are worth showing ("the text is too small to read"); a
	// broken or cut-off JSON object is not.
	if trimmed := strings.TrimSpace(reply); trimmed != "" && !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		replyErr.Reply = shortReply(trimmed)
	}
	return replyErr
}

// generate sends the request body to every model in turn, again after each
// wait while they are busy, and returns the first answer: its text with any
// ```json fence removed, and why the model stopped.
func (gs *GeminiService) generate(ctx context.Context, jsonBody []byte) (reply, finish string, err error) {
	var lastErr error
	for round := 0; ; round++ {
		for _, model := range gs.models {
			reply, finish, err := gs.attempt(ctx, model, jsonBody)
			if err == nil {
				log.Printf("gemini answered by %s (round %d)", model, round+1)
				return reply, finish, nil
			}
			if !gs.retryable(ctx, err) {
				return "", "", err
			}
			log.Printf("gemini: %s busy, trying the next: %v", model, err)
			lastErr = err
		}
		if round >= len(gs.waits) || gs.sleep(ctx, gs.waits[round]) != nil {
			return "", "", lastErr
		}
	}
}

// retryable is an overloaded model, or one that didn't answer in time while
// the job itself still has time.
func (gs *GeminiService) retryable(ctx context.Context, err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Unavailable()
	}
	return errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil
}

// attempt asks one model once.
func (gs *GeminiService) attempt(ctx context.Context, model string, jsonBody []byte) (reply, finish string, err error) {
	ctx, cancel := context.WithTimeout(ctx, gs.attemptTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf(gs.endpoint, model), bytes.NewReader(jsonBody))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	// In a header, not the URL: a failed request's error message quotes the
	// URL, and that would put the key in the log.
	req.Header.Set("x-goog-api-key", gs.apiKey)

	resp, err := gs.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", "", fmt.Errorf("gemini response: %w", err)
	}

	var gr geminiResponse
	decodeErr := json.Unmarshal(body, &gr)
	if gr.Error != nil {
		return "", "", &APIError{Code: gr.Error.Code, Status: gr.Error.Status, Message: gr.Error.Message}
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", &APIError{Code: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	}
	if decodeErr != nil {
		return "", "", fmt.Errorf("gemini response: %w", decodeErr)
	}
	if len(gr.Candidates) == 0 {
		reason := "NO_ANSWER"
		if gr.PromptFeedback != nil && gr.PromptFeedback.BlockReason != "" {
			reason = gr.PromptFeedback.BlockReason
		}
		return "", "", &ReplyError{FinishReason: reason, Err: errors.New("no candidates")}
	}

	cand := gr.Candidates[0]
	var text strings.Builder
	for _, p := range cand.Content.Parts {
		text.WriteString(p.Text)
	}
	raw := text.String()
	log.Printf("Raw Gemini response (%s): %s", model, raw)

	reply = raw
	if m := jsonFence.FindStringSubmatch(raw); len(m) > 1 {
		reply = m[1]
	}
	return reply, cand.FinishReason, nil
}
