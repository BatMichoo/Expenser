package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/utilities"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type SearchHandler struct {
	DB *database.DB // DB is the database client used for expense operations.
}

// It requires a database connection pool to operate.
func NewSearchHandler(db *database.DB) *SearchHandler {
	return &SearchHandler{
		DB: db,
	}
}

// searchColumns holds the i18n keys of each section's result table headers.
var searchColumns = map[string][]string{
	"car":       {"common.date", "form.expense_type", "common.amount", "common.discount", "common.details", "common.notes", "common.actions"},
	"house":     {"common.date", "form.utility_type", "common.amount", "common.discount", "common.details", "common.notes", "common.actions"},
	"groceries": {"common.date", "groceries.category", "groceries.product", "groceries.quantity", "groceries.unit_price", "groceries.discount_per_unit", "groceries.total_price", "groceries.total_discount", "groceries.supermarket", "common.actions"},
}

func (h *SearchHandler) GetSearch(c *gin.Context) error {
	// The route is /<section>/search.
	section := strings.Split(strings.Trim(c.FullPath(), "/"), "/")[0]
	columns, ok := searchColumns[section]
	if !ok {
		return notFound("error.page_not_found")
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.Search, gin.H{
		"CurrentMonth": time.Now().Format(utilities.DateFormats.MonthOnly),
		"Section":      section,
		"Columns":      columns,
		"Lang":         currentLang(c),
	})
	return nil
}

func parseSearchMonth(c *gin.Context) (time.Time, error) {
	date, err := time.Parse(utilities.DateFormats.MonthOnly, c.Request.PostFormValue("date"))
	if err != nil {
		return time.Time{}, badRequest("error.invalid_month", err)
	}
	return date, nil
}

func (h *SearchHandler) GetResultsHouse(c *gin.Context) error {
	userID := currentUserID(c)
	date, err := parseSearchMonth(c)
	if err != nil {
		return err
	}

	expenses, err := h.DB.GetHouseExpensesForMonth(c.Request.Context(), date.Month(), date.Year(), userID)
	if err != nil {
		return err
	}
	total, err := h.DB.GetTotalHouseExpenseForMonth(c.Request.Context(), date, userID)
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.SearchResultsHouse, gin.H{
		"Expenses": expenses,
		"Total":    total,
		"Lang":     currentLang(c),
	})
	return nil
}

func (h *SearchHandler) GetResultsCar(c *gin.Context) error {
	userID := currentUserID(c)
	date, err := parseSearchMonth(c)
	if err != nil {
		return err
	}

	expenses, err := h.DB.GetCarExpensesForMonth(c.Request.Context(), date.Month(), date.Year(), userID)
	if err != nil {
		return err
	}
	total, err := h.DB.GetTotalCarExpenseForMonth(c.Request.Context(), date.Month(), date.Year(), userID)
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.SearchResultsCar, gin.H{
		"Expenses": expenses,
		"Total":    total,
		"Lang":     currentLang(c),
	})
	return nil
}

func (h *SearchHandler) GetResultsGroceries(c *gin.Context) error {
	userID := currentUserID(c)
	date, err := parseSearchMonth(c)
	if err != nil {
		return err
	}

	expenses, err := h.DB.GetGroceriesExpensesForMonth(c.Request.Context(), date.Month(), date.Year(), userID)
	if err != nil {
		return err
	}
	total, err := h.DB.GetTotalGroceriesExpenseForMonth(c.Request.Context(), date.Month(), date.Year(), userID)
	if err != nil {
		return err
	}

	lang := currentLang(c)
	rows := make([]gin.H, 0, len(expenses))
	for i := range expenses {
		rows = append(rows, groceryRowData(&expenses[i], lang))
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.SearchResultsGroceries, gin.H{
		"Rows":  rows,
		"Total": total,
		"Lang":  lang,
	})
	return nil
}
