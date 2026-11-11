package handlers

import (
	"net/url"
	"testing"
	"time"

	"expenser/internal/models"
	"expenser/internal/services"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBackfillForm(t *testing.T) {
	user := uuid.New()
	req, err := parseBackfillForm(url.Values{
		"typeID": {"4"}, "amount": {"15"}, "discount_amount": {"1.5"}, "notes": {" fiber "},
		"date": {"2026-11-20"}, "backfill_from": {"2024-01"}, "backfill_to": {"2025-10"},
	}, models.RecurringCategoryHouse, user)
	require.NoError(t, err)
	assert.Equal(t, services.YearMonth{Year: 2024, Month: time.January}, req.From)
	assert.Equal(t, services.YearMonth{Year: 2025, Month: time.October}, req.To)
	assert.Equal(t, 20, req.Day, "the day of the date")
	assert.Equal(t, models.BackfillExpense{UserID: user, Category: models.RecurringCategoryHouse, TypeID: 4, Amount: 15, Discount: 1.5, Notes: "fiber"}, req.Expense)
}

func TestParseBackfillFormErrors(t *testing.T) {
	base := func() url.Values {
		return url.Values{"typeID": {"4"}, "amount": {"15"}, "date": {"2026-11-20"}, "backfill_from": {"2024-01"}, "backfill_to": {"2025-10"}}
	}
	for name, tc := range map[string]struct{ key, val, msg string }{
		"type":     {"typeID", "", "error.invalid_type"},
		"amount":   {"amount", "x", "error.invalid_amount"},
		"negative": {"amount", "-1", "error.amount_negative"},
		"discount": {"discount_amount", "-2", "error.amount_negative"},
		"date":     {"date", "", "error.invalid_date"},
		"from":     {"backfill_from", "", "error.backfill_months"},
		"to":       {"backfill_to", "2025-13", "error.backfill_months"},
	} {
		t.Run(name, func(t *testing.T) {
			v := base()
			v.Set(tc.key, tc.val)
			_, err := parseBackfillForm(v, models.RecurringCategoryHouse, uuid.New())
			var httpErr *HTTPError
			require.ErrorAs(t, err, &httpErr)
			assert.Equal(t, tc.msg, httpErr.MessageKey)
		})
	}
}

func TestBackfillErrorMessages(t *testing.T) {
	for err, key := range map[error]string{
		services.ErrBackfillRange:   "error.backfill_months",
		services.ErrBackfillFuture:  "error.backfill_future",
		services.ErrBackfillTooLong: "error.backfill_too_long",
	} {
		var httpErr *HTTPError
		require.ErrorAs(t, backfillError(err), &httpErr)
		assert.Equal(t, key, httpErr.MessageKey)
	}
}
