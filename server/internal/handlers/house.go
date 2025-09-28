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

// HouseData is a composite struct holding all the necessary data
// to render the main home page view, including monthly summaries and recent expenses.
type HouseData struct {
	Name           string
	MonthlyExpense *MonthlyExpense        // MonthlyExpense summarizes the total spending for the current month.
	HighestExpense *HighestExpense        // HighestExpense identifies the single largest expense in the current month.
	RecentExpenses *[]models.HouseExpense // RecentExpenses lists individual expenses for the current month.
}

// HouseHandler provides HTTP handlers for managing home-related expenses.
// It encapsulates database operations and renders HTML templates for a web interface.
type HouseHandler struct {
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

// NewHouseHandler creates and returns a new instance of HomeHandler.
// It requires a database connection pool to operate.
func NewHouseHandler(db *database.DB) *HouseHandler {
	return &HouseHandler{
		DB: db,
	}
}

// INFO: CREATE

// GetCreateHouseForm renders the HTML form for users to input details
// for a new home expense.
// This handler serves the UI component for expense creation.
func (h *HouseHandler) GetCreateHouseForm(c *gin.Context) {
	expTypes, err := h.DB.GetHouseUtilityTypes()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", "")
		return
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.CreateHouseExpForm, expTypes)
}

// CreateHouseExpResponse is the data structure returned to the client
// after a new expense has been successfully created.
// It includes details of the newly created expense and updated summary data.
type CreateHouseExpResponse struct {
	Expense        *models.HouseExpense // Expense is the newly created home expense record.
	MonthlyExpense *MonthlyExpense      // MonthlyExpense provides the updated total for the current month.
	HighestExpense *HighestExpense      // HighestExpense provides the updated highest expense for the current month.
}

// CreateHouseExpense handles the HTTP POST request to create a new home expense.
// It parses form data for expense type, date, amount, and notes,
// validates them, saves the new expense to the database,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *HouseHandler) CreateHouseExpense(c *gin.Context) {
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

	newExpense := &models.HouseExpense{
		CreatedBy:     userID,
		Amount:        amount,
		UtilityTypeID: utilTypeID,
		ExpenseDate:   date,
		Notes:         notes,
	}

	err = h.DB.CreateHouseExpense(newExpense)
	if err != nil {
		// TODO: Handle error page
		fmt.Printf("error creating: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	timeNow := time.Now()

	highestExp, expType, err := h.DB.GetHighestHouseExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page
		fmt.Printf("error fetching highest expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	montlyTotal, err := h.DB.GetTotalHouseExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page
		fmt.Printf("error fetching total expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	if newExpense.ExpenseDate.Month() != timeNow.Month() {
		c.HTML(http.StatusCreated, utilities.Templates.Components.Modal, gin.H{})
		return
	}

	crExpResp := &CreateHouseExpResponse{
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

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateHouseExp, crExpResp)
}

// INFO: READ

// GetHome renders the main home dashboard page.
// It fetches the highest expense, total monthly expense, and a list of
// recent expenses for the current month and year from the database.
// It intelligently renders either the full page layout or a partial HTML
// snippet based on whether the request is an HTMX request.
func (h *HouseHandler) GetHome(c *gin.Context) {
	dateNow := time.Now()
	month := dateNow.Month()
	year := dateNow.Year()

	userIDstr, exists := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	section := c.Query("section")

	if section == "chart" {
		types, _ := h.DB.GetHouseUtilityTypes()
		chartData := gin.H{
			"Type":  "house",
			"Year":  year,
			"Types": types,
		}
		c.HTML(http.StatusOK, utilities.Templates.Components.Chart, chartData)
		return
	}

	highestExpense, utilType, err := h.DB.GetHighestHouseExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	monthlyExpense, err := h.DB.GetTotalHouseExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	recentExpenses, err := h.DB.GetHouseExpensesForMonth(month, year, userID)
	if err != nil {
		fmt.Printf("Error fetching expenses %v", err)
		// TODO: Handle error page
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	pageData := &HouseData{
		Name: "house",
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
			c.HTML(http.StatusOK, utilities.Templates.Components.HouseSummary, pageData)
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

func (h *HouseHandler) GetHomeSection(c *gin.Context) {
	section := c.Query("section")

	c.HTML(http.StatusOK, fmt.Sprintf("house-%s", section), gin.H{})
}

// GetExpenseById retrieves a single home expense by its unique ID
// and renders its details.
// It expects the expense ID to be provided as a path parameter.
func (h *HouseHandler) GetExpenseById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: Handle error page: Invalid ID format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	exp, err := h.DB.GetHouseExpenseByID(id)

	if err != nil {
		// TODO: Handle error page: Expense not found or database error.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	c.HTML(http.StatusOK, "expense", exp)
}

// INFO: UPDATE

type EditFormData struct {
	Expense *models.HouseExpense
	Types   *[]models.HomeUtilityType
}

// GetEditHouseForm renders the HTML form pre-filled with existing expense data
// for editing a specific home expense.
// It expects the expense ID to be provided as a query parameter.
func (h *HouseHandler) GetEditHouseForm(c *gin.Context) {
	id, err := strconv.Atoi(c.Query("id"))
	if err != nil {
		// TODO: Handle error page: Invalid ID format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	exp, err := h.DB.GetHouseExpenseByID(id)
	if err != nil {
		// TODO: Handle error page: Expense not found or database error.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	expTypes, err := h.DB.GetHouseUtilityTypes()
	if err != nil {
		// TODO: Handle error page: Expense not found or database error.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	formData := &EditFormData{
		Expense: exp,
		Types:   expTypes,
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditHouseExpForm, formData)
}

// EditHouseExpenseById handles the HTTP PUT/POST request to update an existing home expense.
// It parses form data for updated expense type, date, amount, and notes,
// validates them, updates the expense in the database by its ID,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *HouseHandler) EditHouseExpenseById(c *gin.Context) {
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

	editExpense := &models.HouseExpense{
		ID:            id,
		Amount:        amount,
		UtilityTypeID: utilTypeID,
		ExpenseDate:   date,
		Notes:         notes,
	}

	err = h.DB.EditHouseExpense(editExpense)
	if err != nil {
		// TODO: Handle error page: Database update failed.
		fmt.Printf("error editing: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	timeNow := time.Now()
	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	highestExp, expType, err := h.DB.GetHighestHouseExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch highest expense after edit.
		fmt.Printf("error fetching highest expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	montlyTotal, err := h.DB.GetTotalHouseExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch monthly total after edit.
		fmt.Printf("error fetching total expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	edExpResp := &CreateHouseExpResponse{
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

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateHouseExp, edExpResp)
}

// INFO: DELETE

// DeleteHouseExp handles the HTTP DELETE request to remove a home expense by its ID.
// After successfully deleting the expense, it updates and returns
// the current month's total and highest expense summaries to reflect the change.
// It responds with 204 No Content if the expense was not found or not deleted,
// or 200 OK with updated summary data otherwise.
func (h *HouseHandler) DeleteHouseExp(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: Handle error page: Invalid ID format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	res, err := h.DB.DeleteHouseExpense(id)
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

	monthlyExpense, err := h.DB.GetTotalHouseExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch monthly total after delete.
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	highestExpense, utilType, err := h.DB.GetHighestHouseExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch highest expense after delete.
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	pageData := &HouseData{
		Name: "house",
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

	c.HTML(http.StatusOK, utilities.Templates.Responses.DeleteHouseExp, pageData)
}
