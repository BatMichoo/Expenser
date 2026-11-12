package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeGemini(t *testing.T, status int, body string) (*GeminiService, *http.Request) {
	t.Helper()
	var got http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = *r
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	gs := NewGeminiService("secret-key", "test-model")
	gs.endpoint = srv.URL + "/%s"
	gs.waits = nil // no retries unless a test asks for them
	return gs, &got
}

// modelAnswer is how the fake answers one model: status and body.
type modelAnswer struct {
	status int
	body   string
	hang   bool // never answer (until the request is cancelled)
}

// fakeModels answers per model (the URL path) and records each call.
func fakeModels(t *testing.T, answers map[string][]modelAnswer, models ...string) (*GeminiService, *[]string, *[]time.Duration) {
	t.Helper()
	var calls []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		model := strings.TrimPrefix(r.URL.Path, "/")
		mu.Lock()
		calls = append(calls, model)
		list := answers[model]
		a := modelAnswer{status: 503, body: overloaded}
		if len(list) > 0 {
			a = list[0]
			if len(list) > 1 {
				answers[model] = list[1:]
			}
		}
		mu.Unlock()
		if a.hang {
			<-r.Context().Done()
			return
		}
		w.WriteHeader(a.status)
		_, _ = io.WriteString(w, a.body)
	}))
	t.Cleanup(srv.Close)
	gs := NewGeminiService("secret-key", models...)
	gs.endpoint = srv.URL + "/%s"
	var slept []time.Duration
	gs.sleep = func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }
	return gs, &calls, &slept
}

const overloaded = `{"error":{"code":503,"message":"This model is currently experiencing high demand.","status":"UNAVAILABLE"}}`

var jpeg = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0}

func reply(text, finish string) string {
	return `{"candidates":[{"content":{"parts":[{"text":` + jsonString(text) + `}]},"finishReason":"` + finish + `"}]}`
}

func jsonString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s) + `"`
}

func TestAnalyzeReceiptReadsItems(t *testing.T) {
	gs, req := fakeGemini(t, 200, reply("```json\n{\"store\":\"Lidl\",\"purchased_at\":\"2026-11-10T14:05\",\"items\":[{\"name\":\"Milk\",\"quantity\":1,\"total_price\":2.4}]}\n```", "STOP"))
	a, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	require.NoError(t, err)
	assert.Equal(t, "Lidl", a.Store)
	assert.Equal(t, "2026-11-10T14:05", a.PurchasedAt)
	require.Len(t, a.Items, 1)
	assert.Equal(t, "secret-key", req.Header.Get("x-goog-api-key"), "key goes in a header")
	assert.NotContains(t, req.URL.RawQuery, "secret-key", "never in the URL, which can end up in logs")
}

func TestAnalyzeReceiptAPIError(t *testing.T) {
	gs, _ := fakeGemini(t, 429, `{"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota).","status":"RESOURCE_EXHAUSTED"}}`)
	_, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 429, apiErr.Code)
	assert.Equal(t, "RESOURCE_EXHAUSTED", apiErr.Status)
	assert.Equal(t, "Resource has been exhausted (e.g. check quota).", apiErr.Message)
	assert.False(t, apiErr.Unavailable())

	gs, _ = fakeGemini(t, 503, `{"error":{"code":503,"message":"The model is overloaded. Please try again later.","status":"UNAVAILABLE"}}`)
	_, err = gs.AnalyzeReceipt(context.Background(), jpeg)
	require.ErrorAs(t, err, &apiErr)
	assert.True(t, apiErr.Unavailable())
}

func TestAnalyzeReceiptAPIErrorWithoutJSON(t *testing.T) {
	gs, _ := fakeGemini(t, 502, `<html>Bad Gateway</html>`)
	_, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 502, apiErr.Code)
	assert.Equal(t, "Bad Gateway", apiErr.Message)
}

func TestAnalyzeReceiptModelSaysWhy(t *testing.T) {
	gs, _ := fakeGemini(t, 200, reply("I'm sorry, but the text in this image is too small to be read.", "STOP"))
	_, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	var replyErr *ReplyError
	require.ErrorAs(t, err, &replyErr)
	assert.Equal(t, "I'm sorry, but the text in this image is too small to be read.", replyErr.Reply)
}

func TestAnalyzeReceiptCutOff(t *testing.T) {
	gs, _ := fakeGemini(t, 200, reply(`{"store":"Kaufland","items":[{"name":"Картофи`, "MAX_TOKENS"))
	_, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	var replyErr *ReplyError
	require.ErrorAs(t, err, &replyErr)
	assert.Equal(t, "MAX_TOKENS", replyErr.FinishReason)
	assert.Empty(t, replyErr.Reply, "half a JSON object is not worth showing")
}

func TestAnalyzeReceiptBlocked(t *testing.T) {
	gs, _ := fakeGemini(t, 200, `{"candidates":[],"promptFeedback":{"blockReason":"SAFETY"}}`)
	_, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	var replyErr *ReplyError
	require.ErrorAs(t, err, &replyErr)
	assert.Equal(t, "SAFETY", replyErr.FinishReason)
}

func TestAnalyzeReceiptNotConfigured(t *testing.T) {
	_, err := NewGeminiService("", "test-model").AnalyzeReceipt(context.Background(), jpeg)
	assert.True(t, errors.Is(err, ErrNotConfigured))
}

func TestReplyIsTrimmed(t *testing.T) {
	long := strings.Repeat("слово ", 200)
	assert.LessOrEqual(t, len([]rune(shortReply(long))), maxReplyRunes+1)
	assert.True(t, strings.HasSuffix(shortReply(long), "…"))
	assert.Equal(t, "two lines", shortReply("  two\n\n lines "))
}

var okReceipt = reply(`{"store":"Kaufland","items":[{"name":"Milk","quantity":1,"total_price":2.4}]}`, "STOP")

func TestFallsBackToTheNextModel(t *testing.T) {
	gs, calls, slept := fakeModels(t, map[string][]modelAnswer{
		"main":   {{status: 503, body: overloaded}},
		"backup": {{status: 200, body: okReceipt}},
	}, "main", "backup", "spare")
	a, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	require.NoError(t, err)
	assert.Equal(t, "Kaufland", a.Store)
	assert.Equal(t, []string{"main", "backup"}, *calls, "the spare is not needed")
	assert.Empty(t, *slept, "no waiting when a fallback answers")
}

func TestRetriesAfterWaitingWhenAllAreBusy(t *testing.T) {
	gs, calls, slept := fakeModels(t, map[string][]modelAnswer{
		"main":   {{status: 503, body: overloaded}, {status: 503, body: overloaded}, {status: 200, body: okReceipt}},
		"backup": {{status: 503, body: overloaded}},
	}, "main", "backup")
	gs.waits = []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
	_, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	require.NoError(t, err)
	assert.Equal(t, []string{"main", "backup", "main", "backup", "main"}, *calls, "every model each round, the main one first")
	assert.Equal(t, []time.Duration{5 * time.Second, 15 * time.Second}, *slept)
}

func TestGivesUpWithGeminisMessage(t *testing.T) {
	gs, calls, slept := fakeModels(t, map[string][]modelAnswer{}, "main", "backup")
	gs.waits = []time.Duration{time.Second, 2 * time.Second}
	_, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.True(t, apiErr.Unavailable())
	assert.Len(t, *calls, 6, "3 rounds of 2 models")
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second}, *slept)
}

func TestOtherErrorsAreNotRetried(t *testing.T) {
	gs, calls, slept := fakeModels(t, map[string][]modelAnswer{
		"main": {{status: 429, body: `{"error":{"code":429,"message":"Quota exceeded.","status":"RESOURCE_EXHAUSTED"}}`}},
	}, "main", "backup")
	gs.waits = []time.Duration{time.Second}
	_, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 429, apiErr.Code)
	assert.Equal(t, []string{"main"}, *calls, "quota is not overload: other models share it")
	assert.Empty(t, *slept)

	gs, calls, _ = fakeModels(t, map[string][]modelAnswer{
		"main": {{status: 200, body: reply("The text is too small to read.", "STOP")}},
	}, "main", "backup")
	_, err = gs.AnalyzeReceipt(context.Background(), jpeg)
	var replyErr *ReplyError
	require.ErrorAs(t, err, &replyErr)
	assert.Equal(t, []string{"main"}, *calls, "a model that answered is not asked again")
}

func TestHangingModelGetsSkipped(t *testing.T) {
	gs, calls, _ := fakeModels(t, map[string][]modelAnswer{
		"main":   {{hang: true}},
		"backup": {{status: 200, body: okReceipt}},
	}, "main", "backup")
	gs.attemptTimeout = 50 * time.Millisecond
	a, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	require.NoError(t, err)
	assert.Equal(t, "Kaufland", a.Store)
	assert.Equal(t, []string{"main", "backup"}, *calls)
}

func TestStopsWhenTheJobRunsOutOfTime(t *testing.T) {
	gs, _, _ := fakeModels(t, map[string][]modelAnswer{}, "main")
	gs.waits = []time.Duration{time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	gs.sleep = func(ctx context.Context, d time.Duration) error { cancel(); return ctx.Err() }
	_, err := gs.AnalyzeReceipt(ctx, jpeg)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr, "reports the last overload, not a bare 'context canceled'")
}

func TestAnalyzeReceiptReadsUnits(t *testing.T) {
	gs, _ := fakeGemini(t, 200, reply(`{"store":"Kaufland","items":[{"name":"Banani","quantity":0.995,"total_price":2.4,"unit":"kg","unit_size":1}]}`, "STOP"))
	a, err := gs.AnalyzeReceipt(context.Background(), jpeg)
	require.NoError(t, err)
	require.Len(t, a.Items, 1)
	assert.Equal(t, "kg", a.Items[0].Unit)
	assert.Equal(t, 1.0, a.Items[0].UnitSize)
}

func TestReceiptPromptAsksForUnits(t *testing.T) {
	assert.Contains(t, Prompts.GroceriesReceipt, `"unit"`)
	assert.Contains(t, Prompts.GroceriesReceipt, `"unit_size"`)
}

func TestSuggestSizes(t *testing.T) {
	var body []byte
	gs, _ := fakeGemini(t, 200, reply("```json\n[{\"unit\":\"l\",\"unit_size\":1},{\"unit\":\"kg\",\"unit_size\":0.4}]\n```", "STOP"))
	gs.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		return http.DefaultTransport.RoundTrip(r)
	})}
	got, err := gs.SuggestSizes(context.Background(), []string{"Прясно мляко 1л", "Кисело мляко 400г"})
	require.NoError(t, err)
	assert.Equal(t, []SizeSuggestion{{Unit: "l", UnitSize: 1}, {Unit: "kg", UnitSize: 0.4}}, got)

	var req struct {
		Contents []struct {
			Parts []map[string]any `json:"parts"`
		} `json:"contents"`
	}
	require.NoError(t, json.Unmarshal(body, &req))
	for _, c := range req.Contents {
		for _, p := range c.Parts {
			assert.Contains(t, p, "text")
			assert.NotContains(t, p, "inline_data")
			assert.NotContains(t, p, "inlineData")
		}
	}
	assert.Contains(t, string(body), "Кисело мляко 400г")
}

func TestSuggestSizesInvalidEntry(t *testing.T) {
	gs, _ := fakeGemini(t, 200, reply(`[{"unit":"box","unit_size":2},{"unit":"kg","unit_size":-1},{"unit":"pc","unit_size":10}]`, "STOP"))
	got, err := gs.SuggestSizes(context.Background(), []string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Equal(t, []SizeSuggestion{{}, {}, {Unit: "pc", UnitSize: 10}}, got)
}

func TestSuggestSizesLengthMismatch(t *testing.T) {
	gs, _ := fakeGemini(t, 200, reply(`[{"unit":"l","unit_size":1}]`, "STOP"))
	_, err := gs.SuggestSizes(context.Background(), []string{"a", "b"})
	var replyErr *ReplyError
	require.ErrorAs(t, err, &replyErr)
}

func TestSuggestSizesNotConfigured(t *testing.T) {
	_, err := NewGeminiService("", "m").SuggestSizes(context.Background(), []string{"a"})
	assert.ErrorIs(t, err, ErrNotConfigured)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
