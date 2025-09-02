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
	"github.com/google/uuid"
)

// HomeHandler provides HTTP handlers for managing home-related expenses.
// It encapsulates database operations and renders HTML templates for a web interface.
type HomeHandler struct {
	DB *database.DB // DB is the database client used for expense operations.
}

// HighestExpense represents the expense with the highest amount for a given period.
// It includes the amount, the type of utility (e.g., "Electricity", "Water"),
// and an 'IsOOB' flag indicating if HTMX should update it out of bounds.
type HighestExpense struct {
	Amount float64
	Type   string
	IsOOB  bool
}

// MonthlyExpense summarizes the total expense for a specific month.
// It includes the aggregated amount, the name of the month,
// and an 'IsOOB' flag.
type MonthlyExpense struct {
	Amount float64
	Month  string
	IsOOB  bool
}

// HomeData is a composite struct holding all the necessary data
// to render the main home page view, including monthly summaries and recent expenses.
type HomeData struct {
	MonthlyExpense *MonthlyExpense       // MonthlyExpense summarizes the total spending for the current month.
	HighestExpense *HighestExpense       // HighestExpense identifies the single largest expense in the current month.
	RecentExpenses *[]models.HomeExpense // RecentExpenses lists individual expenses for the current month.
}

type CarData struct {
	MonthlyExpense *MonthlyExpense      // MonthlyExpense summarizes the total spending for the current month.
	HighestExpense *HighestExpense      // HighestExpense identifies the single largest expense in the current month.
	RecentExpenses *[]models.CarExpense // RecentExpenses lists individual expenses for the current month.
}

// NewHomeHandler creates and returns a new instance of HomeHandler.
// It requires a database connection pool to operate.
func NewHomeHandler(db *database.DB) *HomeHandler {
	return &HomeHandler{
		DB: db,
	}
}

// INFO: CREATE

// GetCreateHomeForm renders the HTML form for users to input details
// for a new home expense.
// This handler serves the UI component for expense creation.
func (h *HomeHandler) GetCreateHomeForm(c *gin.Context) {
	expTypes, err := h.DB.GetHomeUtilityTypes()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", "")
		return
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.CreateHomeExpForm, expTypes)
}

// CreateHomeExpResponse is the data structure returned to the client
// after a new expense has been successfully created.
// It includes details of the newly created expense and updated summary data.
type CreateHomeExpResponse struct {
	Expense        *models.HomeExpense // Expense is the newly created home expense record.
	MonthlyExpense *MonthlyExpense     // MonthlyExpense provides the updated total for the current month.
	HighestExpense *HighestExpense     // HighestExpense provides the updated highest expense for the current month.
}

// CreateHomeExpense handles the HTTP POST request to create a new home expense.
// It parses form data for expense type, date, amount, and notes,
// validates them, saves the new expense to the database,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *HomeHandler) CreateHomeExpense(c *gin.Context) {
	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	utilTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}
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

	newExpense := &models.HomeExpense{
		CreatedBy:     userID,
		Amount:        amount,
		UtilityTypeID: utilTypeID,
		ExpenseDate:   date,
		Notes:         notes,
	}

	err = h.DB.CreateHomeExpense(newExpense)
	if err != nil {
		// TODO: Handle error page
		fmt.Printf("error creating: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	timeNow := time.Now()

	highestExp, expType, err := h.DB.GetHighestHomeExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page
		fmt.Printf("error fetching highest expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	montlyTotal, err := h.DB.GetTotalHomeExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page
		fmt.Printf("error fetching total expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	crExpResp := &CreateHomeExpResponse{
		Expense: newExpense,
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

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateHomeExp, crExpResp)
}

// INFO: READ

// GetHome renders the main home dashboard page.
// It fetches the highest expense, total monthly expense, and a list of
// recent expenses for the current month and year from the database.
// It intelligently renders either the full page layout or a partial HTML
// snippet based on whether the request is an HTMX request.
func (h *HomeHandler) GetHome(c *gin.Context) {
	dateNow := time.Now()
	month := dateNow.Month()
	year := dateNow.Year()

	userIDstr, exists := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	section := c.Query("section")

	if section == "chart" {
		types, _ := h.DB.GetHomeUtilityTypes()
		dates := gin.H{
			"Year":  year,
			"Types": types,
		}
		c.HTML(http.StatusOK, utilities.Templates.Components.HomeChart, dates)
		return
	}

	highestExpense, utilType, err := h.DB.GetHighestHomeExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	monthlyExpense, err := h.DB.GetTotalHomeExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	recentExpenses, err := h.DB.GetHomeExpensesForMonth(month, year, userID)
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
		if section == "summary" {
			c.HTML(http.StatusOK, utilities.Templates.Components.HomeSummary, pageData)
			return
		}

		c.HTML(http.StatusOK, utilities.Templates.Pages.Home, pageData)
	} else {
		rl := &RootLayout{
			TemplateName:    utilities.Templates.Pages.Home,
			TemplateContent: pageData,
			HeaderOpts: &models.HeaderOptions{
				IsLoggedIn: exists,
			},
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

func (h *HomeHandler) GetHomeSection(c *gin.Context) {
	section := c.Query("section")

	c.HTML(http.StatusOK, fmt.Sprintf("home-%s", section), gin.H{})
}

// GetExpenseById retrieves a single home expense by its unique ID
// and renders its details.
// It expects the expense ID to be provided as a path parameter.
func (h *HomeHandler) GetExpenseById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: Handle error page: Invalid ID format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	exp, err := h.DB.GetHomeExpenseByID(id)

	if err != nil {
		// TODO: Handle error page: Expense not found or database error.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	c.HTML(http.StatusOK, "expense", exp)
}

// INFO: UPDATE

type EditFormData struct {
	Expense *models.HomeExpense
	Types   *[]models.HomeUtilityType
}

// GetEditHomeForm renders the HTML form pre-filled with existing expense data
// for editing a specific home expense.
// It expects the expense ID to be provided as a query parameter.
func (h *HomeHandler) GetEditHomeForm(c *gin.Context) {
	id, err := strconv.Atoi(c.Query("id"))
	if err != nil {
		// TODO: Handle error page: Invalid ID format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	exp, err := h.DB.GetHomeExpenseByID(id)
	if err != nil {
		// TODO: Handle error page: Expense not found or database error.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	expTypes, err := h.DB.GetHomeUtilityTypes()
	if err != nil {
		// TODO: Handle error page: Expense not found or database error.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	formData := &EditFormData{
		Expense: exp,
		Types:   expTypes,
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditHomeExpForm, formData)
}

// EditHomeExpenseById handles the HTTP PUT/POST request to update an existing home expense.
// It parses form data for updated expense type, date, amount, and notes,
// validates them, updates the expense in the database by its ID,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *HomeHandler) EditHomeExpenseById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: Handle error page: Invalid ID format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	utilTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		// TODO: Handle error page: Invalid date format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}
	date, err := time.Parse(utilities.DateFormats.Input, c.Request.PostFormValue("date"))
	if err != nil {
		// TODO: Handle error page: Invalid date format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}
	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil {
		// TODO: Handle error page: Invalid amount format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}
	notes := c.Request.PostFormValue("notes")

	editExpense := &models.HomeExpense{
		ID:            id,
		Amount:        amount,
		UtilityTypeID: utilTypeID,
		ExpenseDate:   date,
		Notes:         notes,
	}

	err = h.DB.EditHomeExpense(editExpense)
	if err != nil {
		// TODO: Handle error page: Database update failed.
		fmt.Printf("error editing: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	timeNow := time.Now()
	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	highestExp, expType, err := h.DB.GetHighestHomeExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch highest expense after edit.
		fmt.Printf("error fetching highest expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	montlyTotal, err := h.DB.GetTotalHomeExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch monthly total after edit.
		fmt.Printf("error fetching total expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	edExpResp := &CreateHomeExpResponse{
		Expense: editExpense,
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

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateHomeExp, edExpResp)
}

// INFO: DELETE

// DeleteHomeExp handles the HTTP DELETE request to remove a home expense by its ID.
// After successfully deleting the expense, it updates and returns
// the current month's total and highest expense summaries to reflect the change.
// It responds with 204 No Content if the expense was not found or not deleted,
// or 200 OK with updated summary data otherwise.
func (h *HomeHandler) DeleteHomeExp(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: Handle error page: Invalid ID format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	res, err := h.DB.DeleteHomeExpense(id)
	if err != nil {
		// TODO: Handle error page: Database deletion failed.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	if !res {
		// If res is false, it means the expense was not found or not deleted.
		c.HTML(http.StatusNoContent, "", gin.H{})
		return
	}
	timeNow := time.Now()
	month := timeNow.Month()
	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	monthlyExpense, err := h.DB.GetTotalHomeExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch monthly total after delete.
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	highestExpense, utilType, err := h.DB.GetHighestHomeExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch highest expense after delete.
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
