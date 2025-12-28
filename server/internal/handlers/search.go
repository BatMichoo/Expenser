package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func (h *SearchHandler) GetSearch(c *gin.Context) {
	path := c.Request.URL.Path
	isCar := strings.Contains(path, "car")

	c.HTML(http.StatusOK, utilities.Templates.Components.Search, gin.H{
		"CurrentMonth": time.Now().Format("2006-01"),
		"IsCar":        isCar,
	})
}

func (h *SearchHandler) GetResultsHouse(c *gin.Context) {
	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	date, err := time.Parse(utilities.DateFormats.MonthOnly, c.Request.PostFormValue("date"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request on date.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	expenses, err := h.DB.GetHouseExpensesForMonth(date.Month(), date.Year(), userID)
	total, err := h.DB.GetTotalHouseExpenseForMonth(date, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	results := gin.H{
		"Expenses": expenses,
		"Total":    total,
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.SearchResultsHouse, results)
}

func (h *SearchHandler) GetResultsCar(c *gin.Context) {
	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	date, err := time.Parse(utilities.DateFormats.MonthOnly, c.Request.PostFormValue("date"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request on date.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	expenses, err := h.DB.GetCarExpensesForMonth(date.Month(), date.Year(), userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request on date.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}
	total, err := h.DB.GetTotalCarExpenseForMonth(date.Month(), userID)

	results := gin.H{
		"Expenses": expenses,
		"Total":    total,
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.SearchResultsCar, results)
}
