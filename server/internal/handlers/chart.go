package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/models"
	"net/http"
	"strconv"

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

func (ch *ChartHandler) Search(c *gin.Context) {
	typeStr := c.Query("type")
	yearStr := c.Query("year")

	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	typeId, _ := strconv.Atoi(typeStr)
	year, _ := strconv.Atoi(yearStr)
	var exp *[]models.HomeExpense
	var err error

	if typeStr != "" {
		exp, err = ch.DB.GetExpenseTypeForYear(typeId, year, userID)
	} else {
		exp, err = ch.DB.GetHomeExpensesForYear(year, userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, exp)
}
