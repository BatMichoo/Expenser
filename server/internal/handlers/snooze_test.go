package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"expenser/internal/models"
	"expenser/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSnoozeStore struct {
	bill    *models.RecurringPayment
	snoozed *time.Time
}

func (f *fakeSnoozeStore) GetRecurringPaymentByIDForSnooze(_ context.Context, id uuid.UUID) (*models.RecurringPayment, error) {
	if f.bill == nil || f.bill.ID != id {
		return nil, fmt.Errorf("get: %w", sql.ErrNoRows)
	}
	b := *f.bill
	return &b, nil
}

func (f *fakeSnoozeStore) SnoozeRecurringPayment(_ context.Context, _, userID uuid.UUID, until time.Time) error {
	if userID != f.bill.UserID {
		return fmt.Errorf("snooze: %w", sql.ErrNoRows)
	}
	f.snoozed = &until
	return nil
}

func snoozeCall(t *testing.T, h *SnoozeHandler, method, token string) (string, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/snooze/"+token, nil)
	c.Params = gin.Params{{Key: "token", Value: token}}
	state, data := h.resolve(c, method == http.MethodPost)
	return state, data
}

func TestSnoozeFlow(t *testing.T) {
	now := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	signer := services.NewSnoozeSigner("secret")
	bill := &models.RecurringPayment{ID: uuid.New(), UserID: uuid.New(), Name: "Internet", Active: true, DueDate: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)}
	store := &fakeSnoozeStore{bill: bill}
	h := &SnoozeHandler{store: store, signer: signer, now: func() time.Time { return now }}
	until := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	token := signer.Sign(services.SnoozeClaim{BillID: bill.ID, DueFor: bill.DueDate, Until: until, Expires: now})

	state, data := snoozeCall(t, h, http.MethodGet, token)
	assert.Equal(t, "confirm", state)
	assert.Equal(t, "Internet", data["Bill"])
	assert.Nil(t, store.snoozed, "opening the link changes nothing")

	state, _ = snoozeCall(t, h, http.MethodPost, token)
	assert.Equal(t, "done", state)
	require.NotNil(t, store.snoozed)
	assert.True(t, store.snoozed.Equal(until))

	store.snoozed = nil
	bill.DueDate = bill.DueDate.AddDate(0, 1, 0) // paid: moved on to the next cycle
	state, _ = snoozeCall(t, h, http.MethodPost, token)
	assert.Equal(t, "stale", state)
	assert.Nil(t, store.snoozed)

	bill.DueDate = bill.DueDate.AddDate(0, -1, 0)
	bill.Active = false
	state, _ = snoozeCall(t, h, http.MethodGet, token)
	assert.Equal(t, "stale", state, "paused bills send no reminders")

	store.bill = nil
	state, _ = snoozeCall(t, h, http.MethodGet, token)
	assert.Equal(t, "stale", state, "deleted bill")

	state, _ = snoozeCall(t, h, http.MethodGet, "garbage")
	assert.Equal(t, "invalid", state)

	h.now = func() time.Time { return now.Add(24 * time.Hour) }
	state, _ = snoozeCall(t, h, http.MethodGet, token)
	assert.Equal(t, "expired", state)
}
