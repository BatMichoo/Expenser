package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type HomeHandler struct {
	DB *database.DB
}

type HighestExpense struct {
	Amount float64
	Type   string
	IsOOB  bool
}

type MonthlyExpense struct {
	Amount float64
	Month  string
	IsOOB  bool
}

type HomeData struct {
	MonthlyExpense *MonthlyExpense
	HighestExpense *HighestExpense
	RecentExpenses *[]models.HomeExpense
}

func NewHomeHandler(db *database.DB) *HomeHandler {
	return &HomeHandler{
		DB: db,
	}
}

func (h *HomeHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	res, err := h.DB.DeleteExpense(id)
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	if !res {
		c.HTML(http.StatusNoContent, "", gin.H{})
		return
	}
	timeNow := time.Now()
	month := timeNow.Month()

	monthlyExpense, err := h.DB.GetTotalExpenseForMonth(month)
	if err != nil {
		// TODO: Handle error page
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	highestExpense, utilType, err := h.DB.GetHighestExpenseForMonth(month)
	if err != nil {
		// TODO: Handle error page
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	pageData := &HomeData{
		MonthlyExpense: &MonthlyExpense{
			Amount: monthlyExpense,
			Month:  month.String(),
			IsOOB:  true,
		},
		HighestExpense: &HighestExpense{
			Amount: highestExpense,
			Type:   utilType,
			IsOOB:  true,
		},
	}

	c.HTML(http.StatusOK, utilities.Templates.Responses.DeleteHomeExp, pageData)
}

func (h *HomeHandler) GetHome(c *gin.Context) {
	dateNow := time.Now()
	month := dateNow.Month()
	year := dateNow.Year()

	highestExpense, utilType, err := h.DB.GetHighestExpenseForMonth(month)
	if err != nil {
		// TODO: Handle error page
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	monthlyExpense, err := h.DB.GetTotalExpenseForMonth(month)
	if err != nil {
		// TODO: Handle error page
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	recentExpenses, err := h.DB.GetExpensesForMonth(month, year)
	if err != nil {
		fmt.Printf("Error fetching expenses %v", err)
		// TODO: Handle error page
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	pageData := &HomeData{
		MonthlyExpense: &MonthlyExpense{
			Amount: monthlyExpense,
			Month:  month.String(),
		},
		HighestExpense: &HighestExpense{
			Amount: highestExpense,
			Type:   utilType,
		},
		RecentExpenses: recentExpenses,
	}

	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Pages.Home, pageData)
	} else {
		rl := &RootLayout{
			TemplateName:    utilities.Templates.Pages.Index,
			TemplateContent: pageData,
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}

}

func (h *HomeHandler) GetCreateForm(c *gin.Context) {
	c.HTML(http.StatusOK, utilities.Templates.Components.CreateExpForm, gin.H{})
}

type CreateExpResponse struct {
	ID             int
	Amount         float64
	UtilityType    string
	ExpenseDate    time.Time
	Notes          string
	MonthlyExpense *MonthlyExpense
	HighestExpense *HighestExpense
}

func (h *HomeHandler) CreateExpense(c *gin.Context) {
	utilType := c.Request.PostFormValue("type")
	date, err := time.Parse("2006-01-02", c.Request.PostFormValue("date"))
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}
	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}
	notes := c.Request.PostFormValue("notes")

	fmt.Printf("New Expense is : %s, %s, %v, %s", utilType, date, amount, notes)

	newExpense := &models.HomeExpense{
		Amount:      amount,
		UtilityType: utilType,
		ExpenseDate: date,
		Notes:       notes,
	}

	err = h.DB.CreateHomeExpense(newExpense)
	if err != nil {
		// TODO: Handle error page
		fmt.Printf("error creating: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	timeNow := time.Now()

	highestExp, expType, err := h.DB.GetHighestExpenseForMonth(timeNow.Month())
	montlyTotal, err := h.DB.GetTotalExpenseForMonth(timeNow.Month())

	crExpResp := &CreateExpResponse{
		ID:          newExpense.ID,
		Amount:      newExpense.Amount,
		UtilityType: newExpense.UtilityType,
		ExpenseDate: newExpense.ExpenseDate,
		Notes:       newExpense.Notes,
		HighestExpense: &HighestExpense{
			Amount: highestExp,
			Type:   expType,
			IsOOB:  true,
		},
		MonthlyExpense: &MonthlyExpense{
			Amount: montlyTotal,
			Month:  timeNow.Month().String(),
			IsOOB:  true,
		},
	}

	c.HTML(http.StatusCreated, utilities.Templates.Components.NewExp, crExpResp)
}

func (h *HomeHandler) GetEditForm(c *gin.Context) {
	id, err := strconv.Atoi(c.Query("id"))
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	exp, err := h.DB.GetHomeExpenseByID(id)
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditExpForm, exp)
}

func (h *HomeHandler) EditExpenseById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	utilType := c.Request.PostFormValue("type")
	date, err := time.Parse("2006-01-02", c.Request.PostFormValue("date"))
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}
	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}
	notes := c.Request.PostFormValue("notes")

	fmt.Printf("New Expense is : %s, %s, %v, %s", utilType, date, amount, notes)

	editExpense := &models.HomeExpense{
		ID:          id,
		Amount:      amount,
		UtilityType: utilType,
		ExpenseDate: date,
		Notes:       notes,
	}

	err = h.DB.EditHomeExpense(editExpense)
	if err != nil {
		// TODO: Handle error page
		fmt.Printf("error editing: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	timeNow := time.Now()

	highestExp, expType, err := h.DB.GetHighestExpenseForMonth(timeNow.Month())
	montlyTotal, err := h.DB.GetTotalExpenseForMonth(timeNow.Month())

	edExpResp := &CreateExpResponse{
		ID:          editExpense.ID,
		Amount:      editExpense.Amount,
		UtilityType: editExpense.UtilityType,
		ExpenseDate: editExpense.ExpenseDate,
		Notes:       editExpense.Notes,
		HighestExpense: &HighestExpense{
			Amount: highestExp,
			Type:   expType,
			IsOOB:  true,
		},
		MonthlyExpense: &MonthlyExpense{
			Amount: montlyTotal,
			Month:  timeNow.Month().String(),
			IsOOB:  true,
		},
	}

	c.HTML(http.StatusCreated, utilities.Templates.Components.NewExp, edExpResp)
}

func (h *HomeHandler) GetExpenseById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	exp, err := h.DB.GetHomeExpenseByID(id)

	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	c.HTML(http.StatusOK, "expense", exp)
}
