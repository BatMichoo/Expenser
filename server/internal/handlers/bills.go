package handlers

import (
	"database/sql"
	"errors"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BillsHandler serves the Bills section: recurring payments the user wants
// to be reminded about and can mark as paid.
type BillsHandler struct {
	BaseHandler BaseHandler
	bills       *services.RecurringPaymentService
}

func NewBillsHandler(rh *RootHandler) *BillsHandler {
	return &BillsHandler{
		BaseHandler: rh.BaseHandler,
		bills:       services.NewRecurringPaymentService(rh.BaseHandler.DB),
	}
}

// billsChanged tells the page that bills changed (and, when an expense was
// recorded, that expenses changed too) so open views refresh themselves.
func billsChanged(c *gin.Context, expenseRecorded bool) {
	if expenseRecorded {
		c.Header("HX-Trigger", "bills-changed, expenses-changed")
		return
	}
	c.Header("HX-Trigger", "bills-changed")
}

// billError turns service and validation errors into user-facing errors.
func billError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sql.ErrNoRows):
		return notFound("error.bill_not_found")
	case errors.Is(err, services.ErrStaleUpdate):
		return &HTTPError{Status: http.StatusConflict, MessageKey: "error.bill_changed", Err: err}
	case errors.Is(err, services.ErrInvalidPaidAmount):
		return badRequest("error.amount_negative", err)
	case errors.Is(err, models.ErrInvalidEmail):
		return badRequest("error.invalid_email", err)
	case errors.Is(err, models.ErrNameTooLong):
		return badRequest("error.bill_name_too_long", err)
	case errors.Is(err, models.ErrTypeMismatch), errors.Is(err, models.ErrInvalidCategory):
		return badRequest("error.bill_type_required", err)
	case errors.Is(err, models.ErrInvalidAmount),
		errors.Is(err, models.ErrInvalidCycleEvery),
		errors.Is(err, models.ErrInvalidCycleUnit),
		errors.Is(err, models.ErrInvalidRemindDays),
		errors.Is(err, models.ErrInvalidRepeat),
		errors.Is(err, models.ErrInvalidMaxReminders),
		errors.Is(err, models.ErrInvalidDateKind),
		errors.Is(err, models.ErrInvalidNotify),
		errors.Is(err, models.ErrMissingDueDate):
		return badRequest("error.bill_invalid", err)
	}
	return err
}

func billID(c *gin.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, badRequest("error.invalid_id", err)
	}
	return id, nil
}

// knownPaymentOptions are offered as checkboxes; anything else is "other".
var knownPaymentOptions = []string{
	models.PaymentOptionCash, models.PaymentOptionCard,
	models.PaymentOptionBankTransfer, models.PaymentOptionOnline,
}

// parseBillForm reads the add/edit bill form. The model validates the
// values; this only rejects input that cannot be parsed.
func parseBillForm(v url.Values) (*models.RecurringPayment, error) {
	p := &models.RecurringPayment{
		Category:           models.RecurringPaymentCategory(v.Get("category")),
		Name:               v.Get("name"),
		CycleUnit:          models.CycleUnit(v.Get("cycle_unit")),
		PaymentHours:       strings.TrimSpace(v.Get("payment_hours")),
		Notes:              strings.TrimSpace(v.Get("notes")),
		RememberLastAmount: v.Get("remember_last_amount") != "",
		Active:             v.Get("paused") == "",
	}

	typeID, err := strconv.Atoi(v.Get("typeID"))
	if err != nil {
		return nil, fmt.Errorf("type: %w", models.ErrTypeMismatch)
	}
	if p.Category == models.RecurringCategoryCar {
		p.CarExpenseTypeID = &typeID
	} else {
		p.UtilityTypeID = &typeID
	}

	if p.DueDate, err = time.Parse(utilities.DateFormats.HTML, v.Get("due_date")); err != nil {
		return nil, fmt.Errorf("due date: %w", models.ErrMissingDueDate)
	}
	if s := strings.TrimSpace(v.Get("expected_amount")); s != "" {
		amount, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("expected amount: %w", models.ErrInvalidAmount)
		}
		p.ExpectedAmount = &amount
	}
	if p.CycleEvery, err = strconv.Atoi(v.Get("cycle_every")); err != nil {
		return nil, fmt.Errorf("cycle: %w", models.ErrInvalidCycleEvery)
	}
	if s := v.Get("remind_days_before"); s != "" {
		if p.RemindDaysBefore, err = strconv.Atoi(s); err != nil {
			return nil, fmt.Errorf("remind: %w", models.ErrInvalidRemindDays)
		}
	}

	p.PaymentOptions = append([]string{}, v["payment_options"]...)
	if other := strings.TrimSpace(v.Get("payment_option_other")); other != "" {
		p.PaymentOptions = append(p.PaymentOptions, other)
	}

	p.DateKind = models.BillDateKind(v.Get("date_kind"))
	p.Notify = models.NotifyType(v.Get("notify"))
	p.NotifyEmail = v.Get("notify_email")
	if s := v.Get("remind_days_after"); s != "" {
		if p.RemindDaysAfter, err = strconv.Atoi(s); err != nil {
			return nil, fmt.Errorf("remind after: %w", models.ErrInvalidRemindDays)
		}
	}
	if v.Get("keep_reminding") != "" {
		every, err := strconv.Atoi(v.Get("repeat_every_days"))
		if err != nil {
			return nil, fmt.Errorf("repeat: %w", models.ErrInvalidRepeat)
		}
		p.RepeatEveryDays = &every
	}
	if s := strings.TrimSpace(v.Get("max_reminders")); s != "" {
		max, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("max reminders: %w", models.ErrInvalidMaxReminders)
		}
		p.MaxReminders = &max
	}
	if s := v.Get("snoozed_until"); s != "" {
		d, err := time.Parse(utilities.DateFormats.HTML, s)
		if err != nil {
			return nil, fmt.Errorf("snooze: %w", models.ErrMissingDueDate)
		}
		p.SnoozedUntil = &d
	}

	if s := v.Get("version"); s != "" {
		if p.UpdatedAt, err = time.Parse(time.RFC3339Nano, s); err != nil {
			return nil, fmt.Errorf("version: %w", err)
		}
	}
	return p, nil
}

func (h *BillsHandler) view(c *gin.Context) (*models.BillsView, error) {
	payments, err := h.bills.List(c.Request.Context(), currentUserID(c))
	if err != nil {
		return nil, err
	}
	v := services.BuildBillsView(payments, time.Now())
	v.Lang = currentLang(c)
	return &v, nil
}

// GetBills renders the Bills page (or just its content for HTMX).
func (h *BillsHandler) GetBills(c *gin.Context) error {
	v, err := h.view(c)
	if err != nil {
		return err
	}
	renderPage(c, utilities.Templates.Pages.Bills, v)
	return nil
}

// GetBillsView renders the bills list; it reloads itself on bills-changed.
func (h *BillsHandler) GetBillsView(c *gin.Context) error {
	v, err := h.view(c)
	if err != nil {
		return err
	}
	c.HTML(http.StatusOK, "bills-view", v)
	return nil
}

// billFormData is what the add/edit form template needs.
type billFormData struct {
	Bill         *models.RecurringPayment
	IsEdit       bool
	Types        []models.TypeOption
	TypeID       int
	Category     string
	DueDate      string
	Version      string
	Options      map[string]bool
	OtherOption  string
	KnownOptions []string
	UserEmail    string // the user's reminder email, shown as the default
	Lang         string
}

func (h *BillsHandler) typesFor(category string) ([]models.TypeOption, error) {
	var opts []models.TypeOption
	if category == string(models.RecurringCategoryCar) {
		types, err := h.BaseHandler.DB.GetCarExpenseTypes()
		if err != nil {
			return nil, err
		}
		for _, t := range *types {
			opts = append(opts, models.TypeOption{ID: t.ID, Name: t.Name})
		}
		return opts, nil
	}
	types, err := h.BaseHandler.DB.GetHouseUtilityTypes()
	if err != nil {
		return nil, err
	}
	for _, t := range *types {
		opts = append(opts, models.TypeOption{ID: t.ID, Name: t.Name})
	}
	return opts, nil
}

func (h *BillsHandler) formData(c *gin.Context, p *models.RecurringPayment, isEdit bool) (*billFormData, error) {
	category := string(p.Category)
	if category == "" {
		category = string(models.RecurringCategoryHouse)
	}
	types, err := h.typesFor(category)
	if err != nil {
		return nil, err
	}

	d := &billFormData{
		Bill: p, IsEdit: isEdit, Types: types, Category: category,
		Options: map[string]bool{}, KnownOptions: knownPaymentOptions, Lang: currentLang(c),
	}
	if user, err := h.BaseHandler.DB.GetUserByID(currentUserID(c)); err == nil {
		d.UserEmail = user.Email
	}
	if p.UtilityTypeID != nil {
		d.TypeID = *p.UtilityTypeID
	}
	if p.CarExpenseTypeID != nil {
		d.TypeID = *p.CarExpenseTypeID
	}
	if !p.DueDate.IsZero() {
		d.DueDate = p.DueDate.Format(utilities.DateFormats.HTML)
	}
	if !p.UpdatedAt.IsZero() {
		d.Version = p.UpdatedAt.Format(time.RFC3339Nano)
	}
	for _, o := range p.PaymentOptions {
		known := false
		for _, k := range knownPaymentOptions {
			if strings.EqualFold(o, k) {
				d.Options[k] = true
				known = true
			}
		}
		if !known && d.OtherOption == "" {
			d.OtherOption = o
		}
	}
	return d, nil
}

// defaultRepeat: new bills keep reminding every 3 days until paid.
var defaultRepeat = 3

// GetNewBillForm renders the add form: house by default, monthly, due in a
// month, cash pre-selected.
func (h *BillsHandler) GetNewBillForm(c *gin.Context) error {
	p := &models.RecurringPayment{
		Category:         models.RecurringPaymentCategory(c.DefaultQuery("category", "house")),
		DueDate:          time.Now().AddDate(0, 1, 0),
		CycleUnit:        models.CycleMonth,
		CycleEvery:       1,
		RemindDaysBefore: 3,
		DateKind:         models.DatePayBy,
		Notify:           models.NotifyEmail,
		RepeatEveryDays:  &defaultRepeat,
		PaymentOptions:   models.DefaultPaymentOptions(),
		Active:           true,
	}
	d, err := h.formData(c, p, false)
	if err != nil {
		return err
	}
	c.HTML(http.StatusOK, "bill-form", d)
	return nil
}

// GetBillTypes swaps the type chips when the House/Car switch changes.
func (h *BillsHandler) GetBillTypes(c *gin.Context) error {
	category := c.Query("category")
	types, err := h.typesFor(category)
	if err != nil {
		return err
	}
	c.HTML(http.StatusOK, "bill-type-chips", gin.H{"Types": types, "Selected": 0, "Lang": currentLang(c)})
	return nil
}

func (h *BillsHandler) CreateBill(c *gin.Context) error {
	if err := c.Request.ParseForm(); err != nil {
		return badRequest("error.bill_invalid", err)
	}
	p, err := parseBillForm(c.Request.PostForm)
	if err != nil {
		return billError(err)
	}
	p.UpdatedAt = time.Time{}
	if err := h.bills.Create(c.Request.Context(), currentUserID(c), p); err != nil {
		return billError(err)
	}
	lang := currentLang(c)
	billsChanged(c, false)
	renderSuccess(c, http.StatusCreated, utilities.T(lang, "bills.created"), billLabel(lang, p))
	return nil
}

func (h *BillsHandler) GetEditBillForm(c *gin.Context) error {
	id, err := billID(c)
	if err != nil {
		return err
	}
	p, err := h.bills.Get(c.Request.Context(), currentUserID(c), id)
	if err != nil {
		return billError(err)
	}
	d, err := h.formData(c, p, true)
	if err != nil {
		return err
	}
	c.HTML(http.StatusOK, "bill-form", d)
	return nil
}

func (h *BillsHandler) UpdateBill(c *gin.Context) error {
	id, err := billID(c)
	if err != nil {
		return err
	}
	if err := c.Request.ParseForm(); err != nil {
		return badRequest("error.bill_invalid", err)
	}
	p, err := parseBillForm(c.Request.PostForm)
	if err != nil {
		return billError(err)
	}
	p.ID = id
	if err := h.bills.Update(c.Request.Context(), currentUserID(c), p); err != nil {
		return billError(err)
	}
	lang := currentLang(c)
	billsChanged(c, false)
	renderSuccess(c, http.StatusOK, utilities.T(lang, "bills.updated"), billLabel(lang, p))
	return nil
}

func (h *BillsHandler) GetDeleteBillConfirm(c *gin.Context) error {
	id, err := billID(c)
	if err != nil {
		return err
	}
	p, err := h.bills.Get(c.Request.Context(), currentUserID(c), id)
	if err != nil {
		return billError(err)
	}
	lang := currentLang(c)
	c.HTML(http.StatusOK, utilities.Templates.Components.ModalConfirm, &models.ModalConfirmContent{
		Title:        utilities.T(lang, "modal.delete_confirm"),
		Method:       "DELETE",
		Endpoint:     template.URL("/bills/" + id.String()),
		Target:       "#bill-" + id.String(),
		Message:      fmt.Sprintf(utilities.T(lang, "bills.delete_message"), billLabel(lang, p)),
		ConfirmLabel: utilities.T(lang, "common.delete"),
		Lang:         lang,
	})
	return nil
}

func (h *BillsHandler) DeleteBill(c *gin.Context) error {
	id, err := billID(c)
	if err != nil {
		return err
	}
	deleted, err := h.bills.Delete(c.Request.Context(), currentUserID(c), id)
	if err != nil {
		return err
	}
	if !deleted {
		return notFound("error.bill_not_found")
	}
	lang := currentLang(c)
	billsChanged(c, false)
	renderSuccess(c, http.StatusOK, utilities.T(lang, "bills.deleted"), "")
	return nil
}

// GetPayForm renders the "mark paid" panel with the expected amount and
// today pre-filled.
func (h *BillsHandler) GetPayForm(c *gin.Context) error {
	id, err := billID(c)
	if err != nil {
		return err
	}
	p, err := h.bills.Get(c.Request.Context(), currentUserID(c), id)
	if err != nil {
		return billError(err)
	}
	c.HTML(http.StatusOK, "bill-pay-form", gin.H{
		"Bill":  p,
		"Label": billLabel(currentLang(c), p),
		"Today": time.Now().Format(utilities.DateFormats.HTML),
		"Lang":  currentLang(c),
	})
	return nil
}

// MarkBillPaid records the expense and moves the bill to its next due date.
func (h *BillsHandler) MarkBillPaid(c *gin.Context) error {
	id, err := billID(c)
	if err != nil {
		return err
	}
	var amount *float64
	if s := strings.TrimSpace(c.PostForm("amount")); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return badRequest("error.invalid_amount", err)
		}
		amount = &v
	}
	var paidOn time.Time
	if s := c.PostForm("paid_on"); s != "" {
		d, err := time.ParseInLocation(utilities.DateFormats.HTML, s, time.Local)
		if err != nil {
			return badRequest("error.invalid_date", err)
		}
		paidOn = d.Add(12 * time.Hour) // midday, so the date survives time zones
	}

	paid, err := h.bills.MarkPaid(c.Request.Context(), currentUserID(c), id, amount, paidOn)
	if err != nil {
		if errors.Is(err, services.ErrAmountRequired) {
			return badRequest("error.bill_amount_required", err)
		}
		return billError(err)
	}
	lang := currentLang(c)
	billsChanged(c, true)
	renderSuccess(c, http.StatusOK, utilities.T(lang, "bills.paid"),
		fmt.Sprintf(utilities.T(lang, "bills.paid_message"), billLabel(lang, paid.Payment),
			paid.Payment.DueDate.Format("02.01.2006")))
	return nil
}

// SkipMissed moves a bill that is several cycles behind to its next due
// date without recording the missed payments.
func (h *BillsHandler) SkipMissed(c *gin.Context) error {
	id, err := billID(c)
	if err != nil {
		return err
	}
	p, err := h.bills.SkipMissed(c.Request.Context(), currentUserID(c), id)
	if err != nil {
		return billError(err)
	}
	lang := currentLang(c)
	billsChanged(c, false)
	renderSuccess(c, http.StatusOK, utilities.T(lang, "bills.skipped"),
		fmt.Sprintf(utilities.T(lang, "bills.next_due"), p.DueDate.Format("02.01.2006")))
	return nil
}

// billLabel names a bill for messages: its own name, or its type.
func billLabel(lang string, p *models.RecurringPayment) string {
	if strings.TrimSpace(p.Name) != "" {
		return strings.TrimSpace(p.Name)
	}
	if p.TypeName != "" {
		return utilities.T(lang, "type."+p.TypeName)
	}
	return utilities.T(lang, "bills.bill")
}
