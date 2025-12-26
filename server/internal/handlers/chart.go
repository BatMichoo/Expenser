package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/models"
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

func (ch *ChartHandler) HouseRoot(c *gin.Context) {
	dateNow := time.Now()
	year := dateNow.Year()

	types, _ := ch.DB.GetHouseUtilityTypes()
	chartData := gin.H{
		"Type":  "house",
		"Year":  year,
		"Types": types,
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.Chart, chartData)
}

func (ch *ChartHandler) HouseSearch(c *gin.Context) {
	typeStr := c.Query("type")
	yearStr := c.Query("year")

	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	typeId, _ := strconv.Atoi(typeStr)
	year, _ := strconv.Atoi(yearStr)
	var exp *[]models.HouseExpense
	var err error

	if typeStr != "" {
		exp, err = ch.DB.GetHouseExpenseTypeForYear(typeId, year, userID)
	} else {
		exp, err = ch.DB.GetHouseExpensesForYear(year, userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, exp)
}

func (ch *ChartHandler) CarRoot(c *gin.Context) {
	dateNow := time.Now()
	year := dateNow.Year()

	types, _ := ch.DB.GetCarExpenseTypes()
	chartData := gin.H{
		"Type":  "car",
		"Year":  year,
		"Types": types,
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.Chart, chartData)
}

func (ch *ChartHandler) CarSearch(c *gin.Context) {
	typeStr := c.Query("type")
	yearStr := c.Query("year")

	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	typeId, _ := strconv.Atoi(typeStr)
	year, _ := strconv.Atoi(yearStr)
	var exp *[]models.CarExpense
	var err error

	if typeStr != "" {
		exp, err = ch.DB.GetCarExpenseTypeForYear(typeId, year, userID)
	} else {
		exp, err = ch.DB.GetCarExpensesForYear(year, userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, exp)
}
