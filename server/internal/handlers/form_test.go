package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"expenser/internal/models"
	"expenser/internal/services"

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

func TestMapError(t *testing.T) {
	errA, errB, errC := errors.New("a"), errors.New("b"), errors.New("c")
	cases := []errorCase{
		{Status: 404, Key: "not.found", Errs: []error{errA}},
		{Status: 400, Key: "bad.input", Errs: []error{errB, errC}},
	}
	assert.NoError(t, mapError(nil, cases))

	var httpErr *HTTPError
	require.ErrorAs(t, mapError(fmt.Errorf("wrapped: %w", errC), cases), &httpErr)
	assert.Equal(t, 400, httpErr.Status)
	assert.Equal(t, "bad.input", httpErr.MessageKey)
	assert.ErrorIs(t, httpErr, errC, "the cause is kept for the log")

	require.ErrorAs(t, mapError(errA, cases), &httpErr)
	assert.Equal(t, 404, httpErr.Status)

	other := errors.New("db down")
	assert.Equal(t, other, mapError(other, cases), "unknown errors pass through")
}

func TestBillErrorMessages(t *testing.T) {
	for err, want := range map[error]struct {
		status int
		key    string
	}{
		sql.ErrNoRows:                 {404, "error.bill_not_found"},
		services.ErrStaleUpdate:       {409, "error.bill_changed"},
		services.ErrInvalidPaidAmount: {400, "error.amount_negative"},
		models.ErrInvalidEmail:        {400, "error.invalid_email"},
		models.ErrNameTooLong:         {400, "error.bill_name_too_long"},
		models.ErrTypeMismatch:        {400, "error.bill_type_required"},
		models.ErrInvalidCategory:     {400, "error.bill_type_required"},
		models.ErrInvalidRepeat:       {400, "error.bill_invalid"},
		models.ErrMissingDueDate:      {400, "error.bill_invalid"},
	} {
		var httpErr *HTTPError
		require.ErrorAs(t, billError(err), &httpErr, err.Error())
		assert.Equal(t, want.status, httpErr.Status, err.Error())
		assert.Equal(t, want.key, httpErr.MessageKey, err.Error())
	}
}

func TestParseReceiptPurchaseTime(t *testing.T) {
	now := time.Date(2026, 11, 12, 18, 30, 0, 0, time.Local)

	got, err := parseReceiptPurchaseTime(url.Values{"purchased_at": {"2026-11-10T14:05"}}, now)
	require.NoError(t, err)
	assert.True(t, time.Date(2026, 11, 10, 14, 5, 0, 0, time.Local).Equal(got))

	got, err = parseReceiptPurchaseTime(url.Values{}, now)
	require.NoError(t, err)
	assert.True(t, now.Equal(got), "no date sent: now")

	_, err = parseReceiptPurchaseTime(url.Values{"purchased_at": {"10.11.2026"}}, now)
	assert.Error(t, err)
}
