package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	database "expenser/internal/db"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BackfillHandler adds the same house or car expense to a range of months
// ("Several months" in the add panel): a preview first, then the payments.
type BackfillHandler struct {
	db  *database.DB
	svc *services.BackfillService
}

func NewBackfillHandler(db *database.DB) *BackfillHandler {
	return &BackfillHandler{db: db, svc: services.NewBackfillService(db)}
}

// parseBackfillForm reads the add panel with "Several months" on. The day
// of the date is the day of the month for every payment.
func parseBackfillForm(v url.Values, category models.RecurringPaymentCategory, userID uuid.UUID) (services.BackfillRequest, error) {
	var req services.BackfillRequest
	typeID, err := strconv.Atoi(v.Get("typeID"))
	if err != nil {
		return req, badRequest("error.invalid_type", err)
	}
	amount, err := strconv.ParseFloat(v.Get("amount"), 64)
	if err != nil {
		return req, badRequest("error.invalid_amount", err)
	}
	discount := 0.0
	if s := strings.TrimSpace(v.Get("discount_amount")); s != "" {
		if discount, err = strconv.ParseFloat(s, 64); err != nil {
			return req, badRequest("error.invalid_amount", err)
		}
	}
	if amount < 0 || discount < 0 {
		return req, badRequest("error.amount_negative", fmt.Errorf("amount %v, discount %v", amount, discount))
	}
	date, err := time.Parse(utilities.DateFormats.Input, v.Get("date"))
	if err != nil {
		return req, badRequest("error.invalid_date", err)
	}
	from, err := time.Parse(utilities.DateFormats.MonthOnly, v.Get("backfill_from"))
	if err != nil {
		return req, badRequest("error.backfill_months", err)
	}
	to, err := time.Parse(utilities.DateFormats.MonthOnly, v.Get("backfill_to"))
	if err != nil {
		return req, badRequest("error.backfill_months", err)
	}
	req.From = services.YearMonth{Year: from.Year(), Month: from.Month()}
	req.To = services.YearMonth{Year: to.Year(), Month: to.Month()}
	req.Day = date.Day()
	req.Expense = models.BackfillExpense{
		UserID: userID, Category: category, TypeID: typeID,
		Amount: amount, Discount: discount, Notes: strings.TrimSpace(v.Get("notes")),
	}
	return req, nil
}

// backfillError turns a refused backfill into a user-facing message.
func backfillError(err error) error {
	switch {
	case errors.Is(err, services.ErrBackfillRange):
		return badRequest("error.backfill_months", err)
	case errors.Is(err, services.ErrBackfillFuture):
		return badRequest("error.backfill_future", err)
	case errors.Is(err, services.ErrBackfillTooLong):
		return badRequest("error.backfill_too_long", err)
	}
	return err
}

// typeName checks the type belongs to the category and returns its name.
func (h *BackfillHandler) typeName(category models.RecurringPaymentCategory, typeID int) (string, error) {
	if category == models.RecurringCategoryCar {
		types, err := h.db.GetCarExpenseTypes()
		if err != nil {
			return "", err
		}
		for _, t := range *types {
			if t.ID == typeID {
				return t.Name, nil
			}
		}
	} else {
		types, err := h.db.GetHouseUtilityTypes()
		if err != nil {
			return "", err
		}
		for _, t := range *types {
			if t.ID == typeID {
				return t.Name, nil
			}
		}
	}
	return "", badRequest("error.invalid_type", fmt.Errorf("type %d is not a %s type", typeID, category))
}

func (h *BackfillHandler) request(c *gin.Context, category models.RecurringPaymentCategory) (services.BackfillRequest, string, error) {
	if err := c.Request.ParseForm(); err != nil {
		return services.BackfillRequest{}, "", badRequest("error.invalid_form", err)
	}
	req, err := parseBackfillForm(c.Request.PostForm, category, currentUserID(c))
	if err != nil {
		return req, "", err
	}
	name, err := h.typeName(category, req.Expense.TypeID)
	return req, name, err
}

// WithPreview sends the add panel to the backfill preview when "Several
// months" is on, and to the normal single-expense create otherwise.
func (h *BackfillHandler) WithPreview(category models.RecurringPaymentCategory, create HandlerFunc) HandlerFunc {
	return func(c *gin.Context) error {
		if c.PostForm("backfill") == "" {
			return create(c)
		}
		req, name, err := h.request(c, category)
		if err != nil {
			return err
		}
		plan, err := h.svc.Preview(c.Request.Context(), req, time.Now())
		if err != nil {
			return backfillError(err)
		}
		// The preview replaces the panel; the add form keeps the dialog open
		// for a retargeted response.
		c.Header("HX-Retarget", "#action-dialog")
		c.Header("HX-Reswap", "innerHTML")
		data := gin.H{
			"Plan": plan, "TypeName": name, "Section": string(category),
			"Values": c.Request.PostForm, "Lang": currentLang(c),
		}
		if n := len(plan.Dates); n > 0 {
			data["First"], data["Last"] = plan.Dates[0], plan.Dates[n-1]
		}
		c.HTML(http.StatusOK, "backfill-preview", data)
		return nil
	}
}

// Confirm adds the payments shown in the preview, skipping any month that
// got one in the meantime.
func (h *BackfillHandler) Confirm(category models.RecurringPaymentCategory) HandlerFunc {
	return func(c *gin.Context) error {
		req, name, err := h.request(c, category)
		if err != nil {
			return err
		}
		plan, err := h.svc.Apply(c.Request.Context(), req, time.Now())
		if err != nil {
			return backfillError(err)
		}
		lang := currentLang(c)
		message := utilities.T(lang, "type."+name)
		if n := len(plan.Skipped); n > 0 {
			message += " · " + fmt.Sprintf(utilities.T(lang, "backfill.skipped_n"), n)
		}
		expensesChanged(c)
		renderSuccess(c, http.StatusCreated, fmt.Sprintf(utilities.T(lang, "backfill.added"), len(plan.Dates)), message)
		return nil
	}
}
