package services

import (
	"expenser/internal/models"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bill(name string, due time.Time, active bool, expected float64, remind int) models.RecurringPayment {
	e := expected
	return models.RecurringPayment{
		ID: uuid.New(), Name: name, Active: active, StartDate: due, DueDate: due,
		CycleUnit: models.CycleMonth, CycleEvery: 1, ExpectedAmount: &e, RemindDaysBefore: remind,
	}
}

func names(rows []models.BillRow) []string {
	out := []string{}
	for _, r := range rows {
		out = append(out, r.Payment.Name)
	}
	return out
}

func TestBuildBillsView(t *testing.T) {
	today := day(2026, 11, 11)
	payments := []models.RecurringPayment{
		bill("later", day(2026, 12, 20), true, 50, 3),
		bill("overdue", day(2026, 11, 5), true, 20, 3),
		bill("soon", day(2026, 11, 20), true, 30, 3),
		bill("paused", day(2026, 11, 12), false, 99, 3),
		bill("long reminder", day(2026, 12, 1), true, 10, 25), // 20 days away but reminded 25 days before
		bill("way behind", day(2026, 9, 1), true, 5, 0),
		bill("next month end", day(2026, 11, 30), true, 7, 0),
	}

	v := BuildBillsView(payments, today)

	assert.Equal(t, []string{"way behind", "overdue"}, names(v.Overdue), "oldest first")
	assert.Equal(t, []string{"soon", "long reminder"}, names(v.DueSoon))
	assert.Equal(t, []string{"next month end", "later"}, names(v.Later))
	assert.Equal(t, []string{"paused"}, names(v.Paused))

	require.Len(t, v.Overdue, 2)
	assert.Equal(t, -71, v.Overdue[0].DaysLeft)
	assert.True(t, v.Overdue[0].Missed, "more than one cycle behind can be skipped")
	assert.False(t, v.Overdue[1].Missed)

	assert.Equal(t, 2, v.OverdueCount)
	assert.InDelta(t, 5+20+30+7, v.DueThisMonth, 0.001, "active bills due by the end of November, overdue included")
	require.NotNil(t, v.Next)
	assert.Equal(t, "soon", v.Next.Payment.Name, "next upcoming, not overdue")
	assert.Equal(t, 7, v.Total)
}

func TestBuildBillsViewEmpty(t *testing.T) {
	v := BuildBillsView(nil, day(2026, 11, 11))
	assert.Zero(t, v.Total)
	assert.Nil(t, v.Next)
}

func TestBuildBillsViewWaiting(t *testing.T) {
	gas := bill("gas", day(2026, 11, 8), true, 40, 0)
	gas.DateKind = models.DateArrives
	late := bill("internet", day(2026, 11, 5), true, 30, 3)
	v := BuildBillsView([]models.RecurringPayment{gas, late}, day(2026, 11, 11))
	assert.Equal(t, []string{"gas"}, names(v.Waiting), "no deadline: waiting, not overdue")
	assert.Equal(t, []string{"internet"}, names(v.Overdue))
	assert.Equal(t, 1, v.OverdueCount)
	assert.Equal(t, 1, v.WaitingCount)
}

func TestBillsViewNeedsAttention(t *testing.T) {
	today := day(2026, 11, 11)
	arrived := bill("Gas", day(2026, 11, 8), true, 40, 0)
	arrived.DateKind = models.DateArrives
	v := BuildBillsView([]models.RecurringPayment{
		bill("Overdue", day(2026, 11, 5), true, 10, 3),
		arrived,
		bill("Soon", day(2026, 11, 13), true, 10, 3),
		bill("Later", day(2026, 12, 20), true, 10, 3),
		bill("Paused", day(2026, 11, 1), false, 10, 3),
	}, today)
	assert.Equal(t, 3, v.NeedsAttention(), "overdue, waiting and due soon; not later or paused")
	assert.Equal(t, 0, models.BillsView{}.NeedsAttention())
}
