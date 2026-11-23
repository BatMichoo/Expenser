package handlers

import (
	"testing"
	"time"

	"expenser/internal/models"
	"expenser/internal/utilities"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDueDateText(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	bill := func(y int, m time.Month, d int, kind models.BillDateKind) *models.RecurringPayment {
		return &models.RecurringPayment{DueDate: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), DateKind: kind}
	}
	assert.Equal(t, "09.12.2026", dueDateText("en", bill(2026, 12, 9, models.DatePayBy)))
	assert.Equal(t, "11.12.2026 (due Sat 12.12)", dueDateText("en", bill(2026, 12, 12, models.DatePayBy)))
	assert.Equal(t, "11.12.2026 (due Sun 13.12)", dueDateText("en", bill(2026, 12, 13, models.DatePayBy)))
	assert.Equal(t, "11.12.2026 (срок съб 12.12)", dueDateText("bg", bill(2026, 12, 12, models.DatePayBy)))
	assert.Equal(t, "13.12.2026", dueDateText("en", bill(2026, 12, 13, models.DateArrives)))
}
