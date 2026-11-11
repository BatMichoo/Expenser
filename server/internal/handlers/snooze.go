package handlers

import (
	"context"
	"errors"
	"log"
	"time"

	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SnoozeStore is what the email snooze links need from the database.
type SnoozeStore interface {
	GetRecurringPaymentByIDForSnooze(ctx context.Context, id uuid.UUID) (*models.RecurringPayment, error)
	SnoozeRecurringPayment(ctx context.Context, id, userID uuid.UUID, until time.Time) error
}

// SnoozeHandler serves the snooze links in reminder emails. They are signed
// (no login needed) and only work for the reminder they came in: the bill
// must still be active and due on the same date.
type SnoozeHandler struct {
	store  SnoozeStore
	signer *services.SnoozeSigner
	now    func() time.Time
}

func NewSnoozeHandler(store SnoozeStore, signer *services.SnoozeSigner) *SnoozeHandler {
	return &SnoozeHandler{store: store, signer: signer, now: time.Now}
}

// resolve checks the link and, when apply is set, snoozes the bill. It
// returns the page state and its data.
func (h *SnoozeHandler) resolve(c *gin.Context, apply bool) (string, gin.H) {
	lang := currentLang(c)
	data := gin.H{"Lang": lang}
	token := c.Param("token")
	claim, err := h.signer.Verify(token, h.now())
	switch {
	case errors.Is(err, services.ErrSnoozeLinkExpired):
		return "expired", data
	case err != nil:
		return "invalid", data
	}
	ctx := c.Request.Context()
	bill, err := h.store.GetRecurringPaymentByIDForSnooze(ctx, claim.BillID)
	if err != nil || !bill.Active || !models.DateOnly(bill.DueDate).Equal(models.DateOnly(claim.DueFor)) {
		return "stale", data
	}
	data["Bill"] = bill.Name
	if bill.Name == "" {
		data["Bill"] = utilities.T(lang, "type."+bill.TypeName)
	}
	data["Until"], data["Token"] = claim.Until, token
	if !apply {
		return "confirm", data
	}
	if err := h.store.SnoozeRecurringPayment(ctx, bill.ID, bill.UserID, claim.Until); err != nil {
		log.Printf("snooze %s: %v", bill.ID, err)
		return "stale", data
	}
	return "done", data
}

func (h *SnoozeHandler) render(c *gin.Context, apply bool) error {
	state, data := h.resolve(c, apply)
	data["State"] = state
	renderPage(c, "snooze-page", data)
	return nil
}

// GetSnooze shows what the link will do; nothing changes yet.
func (h *SnoozeHandler) GetSnooze(c *gin.Context) error { return h.render(c, false) }

// PostSnooze pauses the bill's reminders.
func (h *SnoozeHandler) PostSnooze(c *gin.Context) error { return h.render(c, true) }
