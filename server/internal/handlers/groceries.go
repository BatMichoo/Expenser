package handlers

import (
	"bytes"
	"context"
	"errors"
	"expenser/internal/models"
	"expenser/internal/services/gemini"
	"expenser/internal/utilities"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GroceriesHandler struct {
	BaseHandler BaseHandler
	GeminiS     *gemini.GeminiService
}

func NewGroceriesHandler(rh *RootHandler, gs *gemini.GeminiService) *GroceriesHandler {
	return &GroceriesHandler{
		BaseHandler: rh.BaseHandler,
		GeminiS:     gs,
	}
}

func (h *GroceriesHandler) getGroceriesPageData(c *gin.Context) (*models.GroceriesData, error) {
	userID := currentUserID(c)

	highest, monthly, err := h.monthSummary(c.Request.Context(), userID)
	if err != nil {
		return nil, err
	}
	highest.IsOOB = false
	monthly.IsOOB = false

	now := time.Now()
	recentExpenses, err := h.BaseHandler.DB.GetGroceriesExpensesForMonth(c.Request.Context(), now.Month(), now.Year(), userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching groceries expenses for month: %w", err)
	}

	return &models.GroceriesData{
		Name:           "current",
		MonthlyExpense: monthly,
		HighestExpense: highest,
		RecentExpenses: &recentExpenses,
		Lang:           currentLang(c),
	}, nil
}

// monthSummary returns the current month's highest category and total
// expense as out-of-band fragments, used to refresh the summary cards.
func (h *GroceriesHandler) monthSummary(ctx context.Context, userID uuid.UUID) (*models.HighestExpense, *models.MonthlyExpense, error) {
	month := time.Now().Month()

	highestExp, category, err := h.BaseHandler.DB.GetHighestGroceriesCategoryForMonth(ctx, month, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching highest groceries category: %w", err)
	}

	monthlyTotal, err := h.BaseHandler.DB.GetTotalGroceriesExpenseForMonth(ctx, month, time.Now().Year(), userID)
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching total groceries expense: %w", err)
	}

	highest := &models.HighestExpense{
		Amount: highestExp,
		Type:   strings.TrimSpace(category),
		IsOOB:  true,
	}
	monthly := &models.MonthlyExpense{
		Amount: monthlyTotal,
		Month:  month.String(),
		IsOOB:  true,
	}
	return highest, monthly, nil
}

// renderGroceryRows renders the given rows plus out-of-band summary cards.
// modal, when not nil, is shown as a success popup.
func (h *GroceriesHandler) renderGroceryRows(c *gin.Context, status int, rows []gin.H, modal *models.ModalContent) error {
	highest, monthly, err := h.monthSummary(c.Request.Context(), currentUserID(c))
	if err != nil {
		return err
	}

	c.HTML(status, utilities.Templates.Responses.GroceryRows, gin.H{
		"Rows":           rows,
		"HighestExpense": highest,
		"MonthlyExpense": monthly,
		"Modal":          modal,
		"Lang":           currentLang(c),
	})
	return nil
}

// groceryRowData is the data the "grocery-row" template expects.
func groceryRowData(e *models.GroceriesExpense, lang string) gin.H {
	return gin.H{
		"ID":              e.ID,
		"CategoryName":    e.CategoryName,
		"PurchaseDate":    e.PurchaseDate,
		"Product":         e.Product,
		"Quantity":        e.Quantity,
		"UnitPrice":       e.UnitPrice,
		"DiscountPerUnit": e.DiscountPerUnit,
		"TotalPrice":      e.TotalPrice,
		"TotalDiscount":   e.TotalDiscount,
		"SupermarketName": e.SupermarketName,
		"Lang":            lang,
	}
}

// otherCategory is the grocery category used when a receipt item has none.
const otherCategory = "Other"

// optionalFloat parses s as a float, treating an empty string as 0.
func optionalFloat(s string) (float64, error) {
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

// parseGroceriesItem reads one grocery item from form fields named
// prefix+"product", prefix+"quantity" and so on. A missing category gives
// CategoryID 0; the caller decides whether that is allowed.
func parseGroceriesItem(c *gin.Context, prefix string) (*models.GroceriesExpense, error) {
	field := func(name string) string { return strings.TrimSpace(c.PostForm(prefix + name)) }

	product := field("product")
	if product == "" {
		return nil, badRequest("error.product_missing", fmt.Errorf("%sproduct is empty", prefix))
	}

	var categoryID int
	if v := field("category_id"); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil || id <= 0 {
			return nil, badRequest("error.invalid_category", fmt.Errorf("%scategory_id %q", prefix, v))
		}
		categoryID = id
	}

	quantity, err := strconv.ParseFloat(field("quantity"), 64)
	if err != nil || quantity <= 0 {
		return nil, badRequest("error.invalid_quantity", fmt.Errorf("%squantity %q: %v", prefix, field("quantity"), err))
	}

	prices := map[string]float64{}
	for _, name := range []string{"unit_price", "total_price", "discount_per_unit", "total_discount"} {
		v, err := optionalFloat(field(name))
		if err != nil || v < 0 {
			return nil, badRequest("error.invalid_price", fmt.Errorf("%s%s %q: %v", prefix, name, field(name), err))
		}
		prices[name] = v
	}

	return &models.GroceriesExpense{
		CategoryID:      categoryID,
		Product:         product,
		Quantity:        quantity,
		UnitPrice:       prices["unit_price"],
		DiscountPerUnit: prices["discount_per_unit"],
		TotalPrice:      prices["total_price"],
		TotalDiscount:   prices["total_discount"],
		SupermarketName: field("supermarket_name"),
	}, nil
}

func parsePurchaseDate(c *gin.Context) (time.Time, error) {
	date, err := time.Parse(utilities.DateFormats.Input, c.PostForm("purchase_date"))
	if err != nil {
		return time.Time{}, badRequest("error.invalid_date", err)
	}
	return date, nil
}

// parseGroceriesExpenseForm reads the fields shared by the create and edit forms.
func parseGroceriesExpenseForm(c *gin.Context) (*models.GroceriesExpense, error) {
	expense, err := parseGroceriesItem(c, "")
	if err != nil {
		return nil, err
	}
	if expense.CategoryID == 0 {
		return nil, badRequest("error.invalid_category", fmt.Errorf("category_id is empty"))
	}

	expense.PurchaseDate, err = parsePurchaseDate(c)
	if err != nil {
		return nil, err
	}
	return expense, nil
}

// isCurrentMonth reports whether t falls in the current month.
func isCurrentMonth(t time.Time) bool {
	now := time.Now()
	return t.Year() == now.Year() && t.Month() == now.Month()
}

func (h *GroceriesHandler) UploadReceipt(c *gin.Context) error {
	lang := currentLang(c)
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

	analysis, err := h.GeminiS.AnalyzeReceipt(c.Request.Context(), buf.Bytes())
	if err != nil {
		if errors.Is(err, gemini.ErrUnavailable) {
			return &HTTPError{Status: http.StatusServiceUnavailable, MessageKey: "error.ai_unavailable", Err: err}
		}
		return &HTTPError{Status: http.StatusBadGateway, MessageKey: "error.receipt_not_analyzed", Err: err}
	}

	categories, err := h.BaseHandler.DB.GetAllCategories(c.Request.Context())
	if err != nil {
		return err
	}

	// Items with a missing or unknown category default to Other.
	known := make(map[string]bool, len(categories))
	for _, cat := range categories {
		known[cat.Name] = true
	}
	for i := range analysis.Items {
		if !known[analysis.Items[i].Category] {
			analysis.Items[i].Category = otherCategory
		}
	}

	// Fall back to today when the receipt date is missing or unreadable.
	purchaseDate := time.Now()
	if d, err := time.Parse(utilities.DateFormats.Input, analysis.Date); err == nil {
		purchaseDate = d
	}

	c.HTML(http.StatusOK, utilities.Templates.Responses.GroceriesReceiptEdit, gin.H{
		"Store":        analysis.Store,
		"PurchaseDate": purchaseDate,
		"Items":        analysis.Items,
		"Categories":   categories,
		"Lang":         lang,
	})
	return nil
}

func (h *GroceriesHandler) ConfirmBatchReceipt(c *gin.Context) error {
	ctx := c.Request.Context()
	lang := currentLang(c)
	userID := currentUserID(c)

	purchaseDate, err := parsePurchaseDate(c)
	if err != nil {
		return err
	}

	var expenses []*models.GroceriesExpense
	for i := 0; ; i++ {
		prefix := fmt.Sprintf("items[%d].", i)
		if _, ok := c.GetPostForm(prefix + "product"); !ok {
			break
		}

		expense, err := parseGroceriesItem(c, prefix)
		if err != nil {
			var httpErr *HTTPError
			if errors.As(err, &httpErr) {
				return &HTTPError{Status: http.StatusBadRequest, MessageKey: "error.receipt_item_invalid", Args: []any{i + 1}, Err: err}
			}
			return err
		}
		if expense.CategoryID == 0 {
			expense.CategoryID, err = h.BaseHandler.DB.GetCategoryIDByName(ctx, otherCategory)
			if err != nil {
				return err
			}
		}
		expense.UserID = userID
		expense.PurchaseDate = purchaseDate
		expenses = append(expenses, expense)
	}
	if len(expenses) == 0 {
		return badRequest("error.receipt_empty", fmt.Errorf("no items submitted"))
	}

	if err := h.BaseHandler.DB.CreateGroceriesExpenses(ctx, expenses); err != nil {
		return err
	}

	var rows []gin.H
	if isCurrentMonth(purchaseDate) {
		for _, expense := range expenses {
			// Fetch the saved expense to get the CategoryName populated
			saved, err := h.BaseHandler.DB.GetGroceriesExpenseByID(ctx, expense.ID, userID)
			if err != nil {
				return err
			}
			rows = append(rows, groceryRowData(saved, lang))
		}
	}

	return h.renderGroceryRows(c, http.StatusCreated, rows, &models.ModalContent{
		Title:   utilities.T(lang, "modal.success_title"),
		Message: utilities.T(lang, "modal.create_success"),
		Lang:    lang,
	})
}

func (h *GroceriesHandler) GetGroceriesHome(c *gin.Context) error {
	pageData, err := h.getGroceriesPageData(c)
	if err != nil {
		return err
	}

	renderPage(c, utilities.Templates.Pages.Groceries, pageData)
	return nil
}

func (h *GroceriesHandler) GetGroceriesForm(c *gin.Context) error {
	c.HTML(http.StatusOK, utilities.Templates.Components.GroceriesReceiptForm, gin.H{
		"Lang": currentLang(c),
	})
	return nil
}

func (h *GroceriesHandler) GetCreateGroceriesForm(c *gin.Context) error {
	categories, err := h.BaseHandler.DB.GetAllCategories(c.Request.Context())
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.CreateGroceriesForm, gin.H{
		"Lang":       currentLang(c),
		"Categories": categories,
		"Today":      time.Now().Format(utilities.DateFormats.Input),
	})
	return nil
}

func (h *GroceriesHandler) PostCreateGroceriesExpense(c *gin.Context) error {
	lang := currentLang(c)
	expense, err := parseGroceriesExpenseForm(c)
	if err != nil {
		return err
	}
	expense.UserID = currentUserID(c)

	if err := h.BaseHandler.DB.CreateGroceriesExpense(c.Request.Context(), expense); err != nil {
		return err
	}

	// Fetch the saved expense to get the CategoryName populated
	saved, err := h.BaseHandler.DB.GetGroceriesExpenseByID(c.Request.Context(), expense.ID, currentUserID(c))
	if err != nil {
		return err
	}

	// Only the current month is listed, so other months get just the popup.
	var rows []gin.H
	if isCurrentMonth(saved.PurchaseDate) {
		rows = append(rows, groceryRowData(saved, lang))
	}

	return h.renderGroceryRows(c, http.StatusCreated, rows, &models.ModalContent{
		Title:   utilities.T(lang, "modal.success_title"),
		Message: utilities.T(lang, "modal.create_success"),
		Lang:    lang,
	})
}

func (h *GroceriesHandler) GetEditGroceriesForm(c *gin.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	expense, err := h.BaseHandler.DB.GetGroceriesExpenseByID(c.Request.Context(), id, currentUserID(c))
	if err != nil {
		return err
	}

	categories, err := h.BaseHandler.DB.GetAllCategories(c.Request.Context())
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
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	expense, err := parseGroceriesExpenseForm(c)
	if err != nil {
		return err
	}
	expense.ID = id

	if err := h.BaseHandler.DB.EditGroceriesExpense(c.Request.Context(), expense, currentUserID(c)); err != nil {
		return err
	}

	// Fetch updated expense to render the row
	updated, err := h.BaseHandler.DB.GetGroceriesExpenseByID(c.Request.Context(), id, currentUserID(c))
	if err != nil {
		return err
	}

	return h.renderGroceryRows(c, http.StatusOK, []gin.H{groceryRowData(updated, currentLang(c))}, nil)
}

func (h *GroceriesHandler) DeleteGroceriesExpense(c *gin.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	deleted, err := h.BaseHandler.DB.DeleteGroceriesExpense(c.Request.Context(), id, currentUserID(c))
	if err != nil {
		return err
	}
	if !deleted {
		return notFound("error.expense_not_found")
	}

	// No rows: the deleted row's target is swapped out with nothing.
	return h.renderGroceryRows(c, http.StatusOK, nil, nil)
}

func (h *GroceriesHandler) GetCurrentMonth(c *gin.Context) error {
	pageData, err := h.getGroceriesPageData(c)
	if err != nil {
		return err
	}

	renderView(c, utilities.Templates.Components.GroceriesCurrent, utilities.Templates.Pages.Groceries, pageData)
	return nil
}
