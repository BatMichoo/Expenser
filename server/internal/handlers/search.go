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

func (h *SearchHandler) GetSearch(c *gin.Context) error {
	c.HTML(http.StatusOK, utilities.Templates.Components.Search, gin.H{
		"CurrentMonth": time.Now().Format(utilities.DateFormats.MonthOnly),
		"IsCar":        strings.Contains(c.Request.URL.Path, "car"),
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

	expenses, err := h.DB.GetHouseExpensesForMonth(date.Month(), date.Year(), userID)
	if err != nil {
		return err
	}
	total, err := h.DB.GetTotalHouseExpenseForMonth(date, userID)
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

	expenses, err := h.DB.GetCarExpensesForMonth(date.Month(), date.Year(), userID)
	if err != nil {
		return err
	}
	total, err := h.DB.GetTotalCarExpenseForMonth(date.Month(), userID)
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
