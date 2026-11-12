package handlers

import (
	"errors"
	"fmt"
	"testing"

	"expenser/internal/services/gemini"
	"expenser/internal/utilities"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReceiptErrorMessages(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	for name, tc := range map[string]struct {
		err    error
		status int
		says   string
	}{
		"model explains": {&gemini.ReplyError{Reply: "The text in this image is too small to be read."}, 502,
			"“The text in this image is too small to be read.”"},
		"cut off":         {&gemini.ReplyError{FinishReason: "MAX_TOKENS"}, 502, "too long"},
		"blocked":         {&gemini.ReplyError{FinishReason: "SAFETY"}, 502, "SAFETY"},
		"api error":       {&gemini.APIError{Code: 429, Status: "RESOURCE_EXHAUSTED", Message: "Resource has been exhausted (e.g. check quota)."}, 502, "Resource has been exhausted (e.g. check quota)."},
		"overloaded":      {&gemini.APIError{Code: 503, Status: "UNAVAILABLE", Message: "The model is overloaded."}, 503, "The model is overloaded."},
		"not configured":  {gemini.ErrNotConfigured, 503, "isn't set up"},
		"network failure": {fmt.Errorf("gemini request: %w", errors.New("dial tcp: i/o timeout")), 502, "couldn't read this receipt"},
	} {
		t.Run(name, func(t *testing.T) {
			var httpErr *HTTPError
			require.ErrorAs(t, receiptError(tc.err), &httpErr)
			assert.Equal(t, tc.status, httpErr.Status)
			msg := httpErr.message("en")
			assert.Contains(t, msg, tc.says)
			assert.NotContains(t, msg, "%!", "format arguments filled in")
		})
	}
}
