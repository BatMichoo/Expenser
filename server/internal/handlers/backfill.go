package handlers

import (
	"fmt"
	"net/http"
	"net/url"
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
func parseBackfillForm(v url.Values, section models.Section, userID uuid.UUID) (services.BackfillRequest, error) {
	var req services.BackfillRequest
	f, err := parseExpenseFields(v)
	if err != nil {
		return req, err
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
	req.Day = f.Date.Day()
	req.Expense = models.BackfillExpense{
		UserID: userID, Section: section, TypeID: f.TypeID,
		Amount: f.Amount, Discount: f.Discount, Notes: f.Notes,
	}
	return req, nil
}

// backfillErrors explain a refused backfill.
var backfillErrors = []errorCase{
	badInput("error.backfill_months", services.ErrBackfillRange),
	badInput("error.backfill_future", services.ErrBackfillFuture),
	badInput("error.backfill_too_long", services.ErrBackfillTooLong),
}

func backfillError(err error) error { return mapError(err, backfillErrors) }

// typeName checks the type belongs to the section and returns its name.
func (h *BackfillHandler) typeName(section models.Section, typeID int) (string, error) {
	types, err := h.db.GetSectionTypes(section)
	if err != nil {
		return "", err
	}
	for _, t := range types {
		if t.ID == typeID {
			return t.Name, nil
		}
	}
	return "", badRequest("error.invalid_type", fmt.Errorf("type %d is not a %s type", typeID, section))
}

func (h *BackfillHandler) request(c *gin.Context, section models.Section) (services.BackfillRequest, string, error) {
	if err := c.Request.ParseForm(); err != nil {
		return services.BackfillRequest{}, "", badRequest("error.invalid_form", err)
	}
	req, err := parseBackfillForm(c.Request.PostForm, section, currentUserID(c))
	if err != nil {
		return req, "", err
	}
	name, err := h.typeName(section, req.Expense.TypeID)
	return req, name, err
}

// WithPreview sends the add panel to the backfill preview when "Several
// months" is on, and to the normal single-expense create otherwise.
func (h *BackfillHandler) WithPreview(section models.Section, create HandlerFunc) HandlerFunc {
	return func(c *gin.Context) error {
		if c.PostForm("backfill") == "" {
			return create(c)
		}
		req, name, err := h.request(c, section)
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
			"Plan": plan, "TypeName": name, "Section": string(section),
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
func (h *BackfillHandler) Confirm(section models.Section) HandlerFunc {
	return func(c *gin.Context) error {
		req, name, err := h.request(c, section)
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
