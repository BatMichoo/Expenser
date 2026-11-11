package handlers

import (
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExpenseFields(t *testing.T) {
	f, err := parseExpenseFields(url.Values{
		"typeID": {"4"}, "date": {"2026-11-20"}, "amount": {"15.5"}, "discount_amount": {" 1.5 "}, "notes": {" fiber "},
	})
	require.NoError(t, err)
	assert.Equal(t, expenseFields{TypeID: 4, Date: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Amount: 15.5, Discount: 1.5, Notes: "fiber"}, f)

	f, err = parseExpenseFields(url.Values{"typeID": {"4"}, "date": {"2026-11-20"}, "amount": {"0"}})
	require.NoError(t, err)
	assert.Zero(t, f.Discount, "discount is optional")
}

func TestParseExpenseFieldsErrors(t *testing.T) {
	base := func() url.Values {
		return url.Values{"typeID": {"4"}, "date": {"2026-11-20"}, "amount": {"15"}}
	}
	for name, tc := range map[string]struct{ key, val, msg string }{
		"type":              {"typeID", "x", "error.invalid_type"},
		"date":              {"date", "20.11.2026", "error.invalid_date"},
		"amount":            {"amount", "", "error.invalid_amount"},
		"negative amount":   {"amount", "-1", "error.amount_negative"},
		"bad discount":      {"discount_amount", "abc", "error.invalid_amount"},
		"negative discount": {"discount_amount", "-2", "error.amount_negative"},
	} {
		t.Run(name, func(t *testing.T) {
			v := base()
			v.Set(tc.key, tc.val)
			_, err := parseExpenseFields(v)
			var httpErr *HTTPError
			require.ErrorAs(t, err, &httpErr)
			assert.Equal(t, tc.msg, httpErr.MessageKey)
		})
	}
}

func TestParamID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	id, err := paramID(c)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	c.Params = gin.Params{{Key: "id", Value: "x"}}
	_, err = paramID(c)
	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, "error.invalid_id", httpErr.MessageKey)
}
