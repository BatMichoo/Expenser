package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/utilities"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ChartHandler struct {
	DB *database.DB
}

func NewChartHandler(db *database.DB) *ChartHandler {
	return &ChartHandler{
		DB: db,
	}
}

func (ch *ChartHandler) FetchChartData(c *gin.Context) {
	monthStr := c.Query("month")
	yearStr := c.Query("year")
	month, _ := strconv.Atoi(monthStr)
	year, _ := strconv.Atoi(yearStr)

	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	exp, _ := ch.DB.GetHomeExpensesForMonth(time.Month(month), year, userID)

	c.JSON(http.StatusOK, exp)
}

func (ch *ChartHandler) Search(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	fromDate, _ := time.Parse(utilities.DateFormats.Input, fromStr)
	toDate, _ := time.Parse(utilities.DateFormats.Input, toStr)

	exp, err := ch.DB.GetExpensesByDates(fromDate, toDate, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, exp)
}
