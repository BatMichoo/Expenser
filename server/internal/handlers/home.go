package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/models"
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
}

type HomeData struct {
	MonthlyExpense float64
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

	c.HTML(http.StatusOK, "", gin.H{})
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
		MonthlyExpense: monthlyExpense,
		HighestExpense: &HighestExpense{
			Amount: highestExpense,
			Type:   utilType,
		},
		RecentExpenses: recentExpenses,
	}

	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, "home", pageData)
	} else {
		rl := &RootLayout{
			TemplateName:    "home",
			TemplateContent: pageData,
		}
		c.HTML(http.StatusOK, "index", rl)
	}

}

func (h *HomeHandler) GetCreateForm(c *gin.Context) {
	c.HTML(http.StatusOK, "new-exp", gin.H{})
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

	c.HTML(http.StatusCreated, "recent-exp-row", newExpense)
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

	c.HTML(http.StatusOK, "edit-exp", exp)
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

	c.HTML(http.StatusOK, "recent-exp-row", editExpense)
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
