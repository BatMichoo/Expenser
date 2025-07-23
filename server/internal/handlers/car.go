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

type CarHandler struct {
	DB *database.DB
}

func NewCarHandler(db *database.DB) *CarHandler {
	return &CarHandler{
		DB: db,
	}
}

func (h *CarHandler) GetHome(c *gin.Context) {
	dateNow := time.Now()
	month := dateNow.Month()
	year := dateNow.Year()

	userIDstr, exists := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	highestExpense, utilType, err := h.DB.GetHighestCarExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	monthlyExpense, err := h.DB.GetTotalCarExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	recentExpenses, err := h.DB.GetCarExpensesForMonth(month, year, userID)
	if err != nil {
		fmt.Printf("Error fetching expenses %v", err)
		// TODO: Handle error page
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	pageData := &CarData{
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
		c.HTML(http.StatusOK, utilities.Templates.Pages.Car, pageData)
	} else {
		// RootLayout is assumed to be a struct that wraps the template name and content
		// for a full page render, typically including common layout elements.
		// It's used when the request is not an HTMX partial request.
		rl := &RootLayout{
			TemplateName:    utilities.Templates.Pages.Car,
			TemplateContent: pageData,
			HeaderOpts: &HeaderOptions{
				IsLoggedIn: exists,
			},
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

func (h *CarHandler) GetCreateCarForm(c *gin.Context) {
	expTypes, err := h.DB.GetCarExpenseTypes()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", "")
		return
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.CreateCarExpForm, expTypes)
}

// CreateExpResponse is the data structure returned to the client
// after a new expense has been successfully created.
// It includes details of the newly created expense and updated summary data.
type CreateCarExpResponse struct {
	Expense        *models.CarExpense // Expense is the newly created car expense record.
	MonthlyExpense *MonthlyExpense    // MonthlyExpense provides the updated total for the current month.
	HighestExpense *HighestExpense    // HighestExpense provides the updated highest expense for the current month.
}

// CreateCarExpense handles the HTTP POST request to create a new home expense.
// It parses form data for expense type, date, amount, and notes,
// validates them, saves the new expense to the database,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *CarHandler) CreateCarExpense(c *gin.Context) {
	expTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
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

		c.HTML(http.StatusBadRequest, "error", err)
		return
	}
	notes := c.Request.PostFormValue("notes")

	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	newExpense := &models.CarExpense{
		Amount:        amount,
		ExpenseTypeID: expTypeID,
		Date:          date,
		Notes:         notes,
		CreatedBy:     userID,
	}

	err = h.DB.CreateCarExpense(newExpense)
	if err != nil {
		// TODO: Handle error page
		fmt.Printf("error creating: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	timeNow := time.Now()

	highestExp, expType, err := h.DB.GetHighestCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page
		fmt.Printf("error fetching highest expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	montlyTotal, err := h.DB.GetTotalCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page
		fmt.Printf("error fetching total expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	crExpResp := &CreateCarExpResponse{
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

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateCarExp, crExpResp)
}

func (h *CarHandler) GetCarExpenseById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: Handle error page: Invalid ID format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	exp, err := h.DB.GetCarExpenseByID(id)

	if err != nil {
		// TODO: Handle error page: Expense not found or database error.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	c.HTML(http.StatusOK, "expense", exp)
}

// INFO: UPDATE

type EditCarFormData struct {
	Expense *models.CarExpense
	Types   *[]models.CarExpenseType
}

// GetEditCarForm renders the HTML form pre-filled with existing expense data
// for editing a specific home expense.
// It expects the expense ID to be provided as a query parameter.
func (h *CarHandler) GetEditCarForm(c *gin.Context) {
	id, err := strconv.Atoi(c.Query("id"))
	if err != nil {
		// TODO: Handle error page: Invalid ID format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	exp, err := h.DB.GetCarExpenseByID(id)
	if err != nil {
		// TODO: Handle error page: Expense not found or database error.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	expTypes, err := h.DB.GetCarExpenseTypes()
	if err != nil {
		// TODO: Handle error page: Expense not found or database error.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	formData := &EditCarFormData{
		Expense: exp,
		Types:   expTypes,
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditCarExpForm, formData)
}

// EditCarExpenseById handles the HTTP PUT/POST request to update an existing home expense.
// It parses form data for updated expense type, date, amount, and notes,
// validates them, updates the expense in the database by its ID,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *CarHandler) EditCarExpenseById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: Handle error page: Invalid ID format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	expTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		// TODO: Handle error page: Invalid date format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}
	date, err := time.Parse("2006-01-02", c.Request.PostFormValue("date"))
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

	fmt.Printf("New Expense is : %v, %s, %v, %s\n", expTypeID, date, amount, notes)

	editExpense := &models.CarExpense{
		ID:            id,
		Amount:        amount,
		ExpenseTypeID: expTypeID,
		Date:          date,
		Notes:         notes,
	}

	err = h.DB.EditCarExpense(editExpense)
	if err != nil {
		// TODO: Handle error page: Database update failed.
		fmt.Printf("error editing: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	timeNow := time.Now()

	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	highestExp, expType, err := h.DB.GetHighestCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch highest expense after edit.
		fmt.Printf("error fetching highest expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	montlyTotal, err := h.DB.GetTotalCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch monthly total after edit.
		fmt.Printf("error fetching total expense: %v", err)
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	edExpResp := &CreateCarExpResponse{
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

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateCarExp, edExpResp)
}

// INFO: DELETE

// DeleteCarExp handles the HTTP DELETE request to remove a home expense by its ID.
// After successfully deleting the expense, it updates and returns
// the current month's total and highest expense summaries to reflect the change.
// It responds with 204 No Content if the expense was not found or not deleted,
// or 200 OK with updated summary data otherwise.
func (h *CarHandler) DeleteCarExp(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		// TODO: Handle error page: Invalid ID format.
		c.HTML(http.StatusBadRequest, "error", err)
		return
	}

	res, err := h.DB.DeleteCarExpense(id)
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

	monthlyExpense, err := h.DB.GetTotalCarExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch monthly total after delete.
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	highestExpense, utilType, err := h.DB.GetHighestCarExpenseForMonth(month, userID)
	if err != nil {
		// TODO: Handle error page: Failed to fetch highest expense after delete.
		// c.HTML(http.StatusInternalServerError, "error", map[string]any{})
		c.HTML(http.StatusInternalServerError, "error", err)
		return
	}

	pageData := &CarData{
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

	c.HTML(http.StatusOK, utilities.Templates.Responses.DeleteCarExp, pageData)
}
