package handlers

import (
	"bytes"
	"errors"
	"expenser/internal/groceryunits"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/services/gemini"
	"expenser/internal/utilities"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type GroceriesHandler struct {
	BaseHandler BaseHandler
	GeminiS     *gemini.GeminiService
	months      *services.MonthViewService
	receipts    *services.ReceiptJobs // receipts being read in the background
}

func NewGroceriesHandler(rh *RootHandler, gs *gemini.GeminiService) *GroceriesHandler {
	return &GroceriesHandler{
		BaseHandler: rh.BaseHandler,
		GeminiS:     gs,
		months:      services.NewMonthViewService(rh.BaseHandler.DB),
		receipts:    services.NewReceiptJobs(gs.AnalyzeReceipt),
	}
}

// renderGroceryRows renders the given rows and tells the page that expenses
// changed, so the month view refreshes its totals.
func (h *GroceriesHandler) renderGroceryRows(c *gin.Context, status int, rows []gin.H) error {
	expensesChanged(c)
	c.HTML(status, "grocery-rows", gin.H{
		"Rows": rows,
		"Lang": currentLang(c),
	})
	return nil
}

// groceryRowData is the data the "grocery-row" template expects.
func groceryRowData(e *models.GroceriesExpense, lang string) gin.H {
	return gin.H{"G": e, "Lang": lang}
}

// parseGroceriesExpenseForm reads the fields shared by the create and edit forms.
func parseGroceriesExpenseForm(c *gin.Context) *models.GroceriesExpense {
	categoryID, _ := strconv.Atoi(c.PostForm("category_id"))
	quantity, _ := strconv.ParseFloat(c.PostForm("quantity"), 64)
	unitPrice, _ := strconv.ParseFloat(c.PostForm("unit_price"), 64)
	discountPerUnit, _ := strconv.ParseFloat(c.PostForm("discount_per_unit"), 64)
	totalPrice, _ := strconv.ParseFloat(c.PostForm("total_price"), 64)
	totalDiscount, _ := strconv.ParseFloat(c.PostForm("total_discount"), 64)

	unit, unitSize := parseSizeFields(c.PostForm("unit"), c.PostForm("unit_size"))

	return &models.GroceriesExpense{
		Unit:            unit,
		UnitSize:        unitSize,
		CategoryID:      categoryID,
		Product:         c.PostForm("product"),
		Quantity:        quantity,
		UnitPrice:       unitPrice,
		DiscountPerUnit: discountPerUnit,
		TotalPrice:      totalPrice,
		TotalDiscount:   totalDiscount,
		SupermarketName: c.PostForm("supermarket_name"),
		PurchaseDate:    time.Now(),
	}
}

// parseSizeFields reads the unit and size inputs. Only known units are
// accepted; without one the size is dropped too.
func parseSizeFields(unit, size string) (string, float64) {
	unit = strings.TrimSpace(unit)
	if !groceryunits.Unit(unit).Valid() {
		return "", 0
	}
	per, _ := strconv.ParseFloat(strings.TrimSpace(size), 64)
	return unit, per
}

// receiptItemSize is the unit and size to prefill for a receipt item: the
// one the name and the reader agree on, or nothing.
func receiptItemSize(item models.ReceiptItem) (string, float64) {
	size, ok := groceryunits.Resolve(item.Name, item.Quantity, item.Unit, item.UnitSize)
	if !ok {
		return "", 0
	}
	return string(size.Unit), size.Per
}

// sizeGuess is what the size fields show for a product: the posted values
// when a unit is already chosen, else the size read from the name.
func sizeGuess(product, quantity, unit, unitSize string) (string, float64) {
	if unit != "" {
		return parseSizeFields(unit, unitSize)
	}
	q, _ := strconv.ParseFloat(quantity, 64)
	if size, ok := groceryunits.Parse(product, q); ok {
		return string(size.Unit), size.Per
	}
	return "", 0
}

// GetSizeGuess re-renders the size fields as the product is typed.
func (h *GroceriesHandler) GetSizeGuess(c *gin.Context) error {
	unit, size := sizeGuess(c.Query("product"), c.Query("quantity"), c.Query("unit"), c.Query("unit_size"))
	c.HTML(http.StatusOK, "size-fields", gin.H{
		"Prefix": c.Query("prefix"),
		"ID":     sizeFieldsID(c.Query("id")),
		"Unit":   unit,
		"Size":   size,
		"Lang":   currentLang(c),
	})
	return nil
}

// sizeFieldsID keeps the wrapper id to the plain ids the forms use.
func sizeFieldsID(id string) string {
	if id == "size-fields" || strings.HasPrefix(id, "size-fields-") {
		for _, r := range strings.TrimPrefix(id, "size-fields-") {
			if r < '0' || r > '9' {
				return "size-fields"
			}
		}
		return id
	}
	return "size-fields"
}

// receiptError tells the user why a receipt couldn't be read, in Gemini's
// own words where it gave some: what the model said instead of a receipt,
// or the API's error message. Failures with nothing useful to say (network,
// unreadable response) get the general message.
func receiptError(err error) error {
	var (
		apiErr   *gemini.APIError
		replyErr *gemini.ReplyError
	)
	gateway := func(key string, args ...any) error {
		return &HTTPError{Status: http.StatusBadGateway, MessageKey: key, Args: args, Err: err}
	}
	switch {
	case errors.Is(err, gemini.ErrNotConfigured):
		return &HTTPError{Status: http.StatusServiceUnavailable, MessageKey: "error.ai_not_configured", Err: err}
	case errors.As(err, &apiErr) && apiErr.Unavailable():
		return &HTTPError{Status: http.StatusServiceUnavailable, MessageKey: "error.ai_unavailable_said", Args: []any{apiErr.Message}, Err: err}
	case errors.As(err, &apiErr):
		return gateway("error.ai_error", apiErr.Message)
	case errors.As(err, &replyErr) && replyErr.Reply != "":
		return gateway("error.receipt_ai_said", replyErr.Reply)
	case errors.As(err, &replyErr) && replyErr.FinishReason == "MAX_TOKENS":
		return gateway("error.receipt_too_long")
	case errors.As(err, &replyErr) && replyErr.FinishReason != "":
		return gateway("error.receipt_ai_refused", replyErr.FinishReason)
	}
	return gateway("error.receipt_not_analyzed")
}

// UploadReceipt starts reading the receipt in the background and shows a
// panel that checks back until it's done (Gemini can take minutes).
func (h *GroceriesHandler) UploadReceipt(c *gin.Context) error {
	file, err := c.FormFile("receipt")
	if err != nil {
		return badRequest("error.receipt_missing", err)
	}
	src, err := file.Open()
	if err != nil {
		return badRequest("error.receipt_unreadable", fmt.Errorf("failed to open file: %w", err))
	}
	defer src.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(src); err != nil {
		return badRequest("error.receipt_unreadable", fmt.Errorf("failed to read file: %w", err))
	}

	id := h.receipts.Start(currentUserID(c), buf.Bytes())
	job, _ := h.receipts.Get(id, currentUserID(c))
	return h.renderReceiptJob(c, job)
}

// GetReceiptJob is what the reading panel checks: still reading, the review
// form once done, or why it failed.
func (h *GroceriesHandler) GetReceiptJob(c *gin.Context) error {
	job, ok := h.receipts.Get(c.Param("id"), currentUserID(c))
	if !ok {
		c.HTML(http.StatusOK, "groceries-receipt-job", gin.H{"State": "expired", "Lang": currentLang(c)})
		return nil
	}
	return h.renderReceiptJob(c, job)
}

// renderReceiptJob shows a job's state; a finished job's result (or error)
// is shown once and then forgotten.
func (h *GroceriesHandler) renderReceiptJob(c *gin.Context, job services.ReceiptJob) error {
	lang := currentLang(c)
	switch job.Status {
	case services.JobReading:
		c.HTML(http.StatusOK, "groceries-receipt-job", gin.H{"State": "reading", "ID": job.ID, "Lang": lang})
		return nil
	case services.JobFailed:
		h.receipts.Forget(job.ID, job.UserID)
		message := utilities.T(lang, "error.receipt_not_analyzed")
		var httpErr *HTTPError
		if errors.As(receiptError(job.Err), &httpErr) {
			message = httpErr.message(lang)
		}
		c.HTML(http.StatusOK, "groceries-receipt-job", gin.H{"State": "failed", "Message": message, "Lang": lang})
		return nil
	}
	h.receipts.Forget(job.ID, job.UserID)
	return h.renderReceiptReview(c, job.Result)
}

// parseReceiptPurchaseTime reads the purchase date and time from the
// receipt review form; without one the items are dated now.
func parseReceiptPurchaseTime(form url.Values, now time.Time) (time.Time, error) {
	s := strings.TrimSpace(form.Get("purchased_at"))
	if s == "" {
		return now, nil
	}
	t, err := time.ParseInLocation(receiptTimeInput, s, time.Local)
	if err != nil {
		return time.Time{}, badRequest("error.invalid_date", err)
	}
	return t, nil
}

// receiptTimeInput is the value format of a datetime-local input.
const receiptTimeInput = "2006-01-02T15:04"

// renderReceiptReview shows what was read, to check before saving.
func (h *GroceriesHandler) renderReceiptReview(c *gin.Context, analysis *models.ReceiptAnalysis) error {
	categories, err := h.BaseHandler.DB.GetAllCategories()
	if err != nil {
		return err
	}
	subtotal, discount := analysis.Sums()
	purchasedAt, _ := analysis.PurchaseTime(time.Now())
	items := make([]models.ReceiptItem, len(analysis.Items))
	for i, item := range analysis.Items {
		item.Unit, item.UnitSize = receiptItemSize(item)
		items[i] = item
	}
	c.HTML(http.StatusOK, "groceries-receipt-edit-form", gin.H{
		"PurchasedAt": purchasedAt,
		"Store":       analysis.Store,
		"Items":       items,
		"Subtotal":    subtotal,
		"Discount":    discount,
		"Total":       subtotal - discount,
		"Categories":  categories,
		"Lang":        currentLang(c),
	})
	return nil
}

func (h *GroceriesHandler) ConfirmBatchReceipt(c *gin.Context) error {
	lang := currentLang(c)
	userID := currentUserID(c)
	if err := c.Request.ParseForm(); err != nil {
		return badRequest("error.invalid_date", err)
	}
	purchasedAt, err := parseReceiptPurchaseTime(c.Request.PostForm, time.Now())
	if err != nil {
		return err
	}

	var rows []gin.H
	for i := 0; ; i++ {
		product := c.PostForm(fmt.Sprintf("items[%d].product", i))
		if product == "" {
			break
		}
		categoryID, _ := strconv.Atoi(c.PostForm(fmt.Sprintf("items[%d].category_id", i)))
		if categoryID == 0 {
			categoryID = 7 // Default to 'Other' if not provided or empty
		}
		quantity, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].quantity", i)), 64)
		unitPrice, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].unit_price", i)), 64)
		discountPerUnit, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].discount_per_unit", i)), 64)
		totalPrice, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].total_price", i)), 64)
		totalDiscount, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].total_discount", i)), 64)
		supermarketName := c.PostForm(fmt.Sprintf("items[%d].supermarket_name", i))
		unit, unitSize := parseSizeFields(c.PostForm(fmt.Sprintf("items[%d].unit", i)), c.PostForm(fmt.Sprintf("items[%d].unit_size", i)))

		expense := &models.GroceriesExpense{
			UserID:          userID,
			CategoryID:      categoryID,
			Product:         product,
			Quantity:        quantity,
			UnitPrice:       unitPrice,
			DiscountPerUnit: discountPerUnit,
			TotalPrice:      totalPrice,
			TotalDiscount:   totalDiscount,
			SupermarketName: supermarketName,
			Unit:            unit,
			UnitSize:        unitSize,
			PurchaseDate:    purchasedAt,
		}

		if err := h.BaseHandler.DB.CreateGroceriesExpense(expense); err != nil {
			return fmt.Errorf("failed to save item %d: %w", i, err)
		}

		// Fetch the saved expense to get the CategoryName populated
		dbExpense, err := h.BaseHandler.DB.GetGroceriesExpenseByID(expense.ID, currentUserID(c))
		if err == nil {
			expense = dbExpense
		}
		rows = append(rows, groceryRowData(expense, lang))
	}

	expensesChanged(c)
	c.HTML(http.StatusCreated, "confirm-batch-groceries", gin.H{
		"Rows": rows,
		"Modal": &models.ModalContent{
			Title:   utilities.T(lang, "modal.success_title"),
			Message: utilities.T(lang, "modal.create_success"),
			Lang:    lang,
		},
		"Lang": lang,
	})
	return nil
}

func (h *GroceriesHandler) GetGroceriesHome(c *gin.Context) error {
	pageData, err := h.monthView(c)
	if err != nil {
		return err
	}

	renderPage(c, utilities.Templates.Pages.Groceries, pageData)
	return nil
}

// GetGroceriesForm opens the receipt upload panel. If a receipt is still
// being read, or finished while the panel was closed, that is shown instead
// (?new=1 starts a fresh upload).
func (h *GroceriesHandler) GetGroceriesForm(c *gin.Context) error {
	if c.Query("new") == "" {
		if job, ok := h.receipts.Latest(currentUserID(c)); ok {
			return h.renderReceiptJob(c, job)
		}
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.GroceriesReceiptForm, gin.H{
		"Lang": currentLang(c),
	})
	return nil
}

func (h *GroceriesHandler) GetCreateGroceriesForm(c *gin.Context) error {
	categories, err := h.BaseHandler.DB.GetAllCategories()
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.CreateGroceriesForm, gin.H{
		"Lang":       currentLang(c),
		"Categories": categories,
	})
	return nil
}

func (h *GroceriesHandler) PostCreateGroceriesExpense(c *gin.Context) error {
	expense := parseGroceriesExpenseForm(c)
	expense.UserID = currentUserID(c)

	if err := h.BaseHandler.DB.CreateGroceriesExpense(expense); err != nil {
		return err
	}

	// Fetch the saved expense to get the CategoryName populated
	saved, err := h.BaseHandler.DB.GetGroceriesExpenseByID(expense.ID, currentUserID(c))
	if err != nil {
		return err
	}

	return h.renderGroceryRows(c, http.StatusCreated, []gin.H{groceryRowData(saved, currentLang(c))})
}

func (h *GroceriesHandler) GetEditGroceriesForm(c *gin.Context) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}

	expense, err := h.BaseHandler.DB.GetGroceriesExpenseByID(id, currentUserID(c))
	if err != nil {
		return err
	}

	categories, err := h.BaseHandler.DB.GetAllCategories()
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditGroceriesForm, gin.H{
		"Expense":    expense,
		"Categories": categories,
		"Lang":       currentLang(c),
	})
	return nil
}

func (h *GroceriesHandler) EditGroceriesExpense(c *gin.Context) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}

	expense := parseGroceriesExpenseForm(c)
	expense.ID = id

	if err := h.BaseHandler.DB.EditGroceriesExpense(expense, currentUserID(c)); err != nil {
		return err
	}

	// Fetch updated expense to render the row
	updated, err := h.BaseHandler.DB.GetGroceriesExpenseByID(id, currentUserID(c))
	if err != nil {
		return err
	}

	return h.renderGroceryRows(c, http.StatusOK, []gin.H{groceryRowData(updated, currentLang(c))})
}

func (h *GroceriesHandler) DeleteGroceriesExpense(c *gin.Context) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}

	deleted, err := h.BaseHandler.DB.DeleteGroceriesExpense(id, currentUserID(c))
	if err != nil {
		return err
	}
	if !deleted {
		return notFound("error.expense_not_found")
	}

	// No rows: the deleted row's target is swapped out with nothing.
	return h.renderGroceryRows(c, http.StatusOK, nil)
}

func (h *GroceriesHandler) GetCurrentMonth(c *gin.Context) error {
	pageData, err := h.monthView(c)
	if err != nil {
		return err
	}

	renderView(c, utilities.Templates.Components.GroceriesCurrent, utilities.Templates.Pages.Groceries, pageData)
	return nil
}

// monthView builds the groceries page for the month, search and category
// filter in the request's query string (current month when missing).
func (h *GroceriesHandler) monthView(c *gin.Context) (*models.MonthView, error) {
	now := time.Now()
	ym := services.ParseYearMonth(c.Query("month"), now)
	return h.months.Groceries(currentUserID(c), ym, monthFilter(c), currentLang(c), now)
}
