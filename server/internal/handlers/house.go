package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"fmt"
	"html/template"
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
	MonthlyExpense *models.MonthlyExpense // MonthlyExpense summarizes the total spending for the current month.
	HighestExpense *models.HighestExpense // HighestExpense identifies the single largest expense in the current month.
	RecentExpenses *[]models.HouseExpense // RecentExpenses lists individual expenses for the current month.
}

// HouseHandler provides HTTP handlers for managing home-related expenses.
// It encapsulates database operations and renders HTML templates for a web interface.
type HouseHandler struct {
	DB *database.DB // DB is the database client used for expense operations.
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
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "500: Internal Server Error :(",
		}

		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.CreateHouseExpForm, expTypes)
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
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request on type ID.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	date, err := time.Parse(utilities.DateFormats.Input, c.Request.PostFormValue("date"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request on date.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request on amount.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
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
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Error creating new house expense.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	timeNow := time.Now()

	highestExp, expType, err := h.DB.GetHighestHouseExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Error fetching highest house expense.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	montlyTotal, err := h.DB.GetTotalHouseExpenseForMonth(timeNow, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Error fetching highest house expense.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	if newExpense.ExpenseDate.Month() != timeNow.Month() {
		c.HTML(http.StatusCreated, utilities.Templates.Components.Dialog, gin.H{})
		return
	}

	expResp := &models.HouseExpResponse{
		Expense: newExpense,
		HighestExpense: &models.HighestExpense{
			Amount: highestExp,
			Type:   expType,
			IsOOB:  true,
		},
		MonthlyExpense: &models.MonthlyExpense{
			Amount: montlyTotal,
			Month:  timeNow.Month().String(),
			IsOOB:  true,
		},
		Modal: &models.ModalContent{
			Title:   "Successful expense creation.",
			Message: fmt.Sprintf("%s: %v BGN", newExpense.UtilityType, newExpense.Amount),
		},
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateHouseExp, expResp)
}

// INFO: READ

// GetHome renders the main home dashboard page.
// It fetches the highest expense, total monthly expense, and a list of
// recent expenses for the current month and year from the database.
// It intelligently renders either the full page layout or a partial HTML
// snippet based on whether the request is an HTMX request.
func (h *HouseHandler) GetCurrentMonth(c *gin.Context) {
	dateNow := time.Now()
	month := dateNow.Month()
	year := dateNow.Year()

	userIDstr, exists := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	highestExpense, utilType, err := h.DB.GetHighestHouseExpenseForMonth(month, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "500: Error fetching highest house expense.",
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	monthlyExpense, err := h.DB.GetTotalHouseExpenseForMonth(dateNow, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "500: Error fetching total house expense.",
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	recentExpenses, err := h.DB.GetHouseExpensesForMonth(month, year, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "500: Error fetching recent house expenses.",
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	pageData := &HouseData{
		Name: "current",
		MonthlyExpense: &models.MonthlyExpense{
			Amount: monthlyExpense,
			Month:  month.String(),
		},
		HighestExpense: &models.HighestExpense{
			Amount: highestExpense,
			Type:   utilType,
		},
		RecentExpenses: recentExpenses,
	}

	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Components.HouseCurrent, pageData)
		fmt.Println("HTMX request!")
		return
	} else {
		rl := &models.RootLayout{
			TemplateName:    utilities.Templates.Pages.House,
			TemplateContent: pageData,
			HeaderOpts: &models.HeaderOptions{
				IsLoggedIn: exists,
			},
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

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

	highestExpense, utilType, err := h.DB.GetHighestHouseExpenseForMonth(month, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "500: Error fetching highest house expense.",
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	monthlyExpense, err := h.DB.GetTotalHouseExpenseForMonth(dateNow, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "500: Error fetching total house expense.",
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	recentExpenses, err := h.DB.GetHouseExpensesForMonth(month, year, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "500: Error fetching recent house expenses.",
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	pageData := &HouseData{
		Name: "current",
		MonthlyExpense: &models.MonthlyExpense{
			Amount: monthlyExpense,
			Month:  month.String(),
		},
		HighestExpense: &models.HighestExpense{
			Amount: highestExpense,
			Type:   utilType,
		},
		RecentExpenses: recentExpenses,
	}

	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Pages.House, pageData)
		return
	} else {
		rl := &models.RootLayout{
			TemplateName:    utilities.Templates.Pages.House,
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
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request on ID.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	exp, err := h.DB.GetHouseExpenseByID(id)

	if err != nil {
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.Modal, err)
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
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request on ID.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	exp, err := h.DB.GetHouseExpenseByID(id)
	if err != nil {
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.Modal, err)
		return
	}

	expTypes, err := h.DB.GetHouseUtilityTypes()
	if err != nil {
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.Modal, err)
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
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.Modal, err)
		return
	}

	utilTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.Modal, err)
		return
	}
	date, err := time.Parse(utilities.DateFormats.Input, c.Request.PostFormValue("date"))
	if err != nil {
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.Modal, err)
		return
	}
	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil || amount <= 0 {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request. Amount invalid, must be a positive number",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
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
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request. Couldn't update expense",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	timeNow := time.Now()
	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	highestExp, expType, err := h.DB.GetHighestHouseExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.Modal, err)
		return
	}

	montlyTotal, err := h.DB.GetTotalHouseExpenseForMonth(timeNow, userID)
	if err != nil {
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.Modal, err)
		return
	}

	edExpResp := &models.HouseExpResponse{
		Expense: editExpense,
		HighestExpense: &models.HighestExpense{
			Amount: highestExp,
			Type:   expType,
			IsOOB:  true,
		},
		MonthlyExpense: &models.MonthlyExpense{
			Amount: montlyTotal,
			Month:  timeNow.Month().String(),
			IsOOB:  true,
		},
		Modal: &models.ModalContent{
			Title:   "Successfully edited expense!",
			Message: fmt.Sprintf("Expense with ID: %v updated!", editExpense.ID),
		},
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateHouseExp, edExpResp)
}

// INFO: DELETE

func (h *HouseHandler) GetDeleteConfirm(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request. Couldn't get ID.",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	content := &models.ModalConfirmContent{
		Title:    "Are you sure you want to delete this?",
		Method:   "DELETE",
		Endpoint: template.URL(fmt.Sprintf("./house/expenses/%v", id)),
		Target:   fmt.Sprintf("#exp-%v", id),
		Message:  fmt.Sprintf("Please confirm if you want to delete expense with ID: %v", id),
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.ModalConfirm, content)
	return
}

// DeleteHouseExp handles the HTTP DELETE request to remove a home expense by its ID.
// After successfully deleting the expense, it updates and returns
// the current month's total and highest expense summaries to reflect the change.
// It responds with 204 No Content if the expense was not found or not deleted,
// or 200 OK with updated summary data otherwise.
func (h *HouseHandler) DeleteHouseExp(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request. Couldn't delete expense",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	res, err := h.DB.DeleteHouseExpense(id)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "400: Bad Request. Couldn't delete expense",
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	if !res {
		// If res is false, it means the expense was not found or not deleted.
		c.HTML(http.StatusNoContent, "", gin.H{})

		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: "204: No Content. Couldn't find expense or it doesn't exist.",
		}
		c.HTML(http.StatusNoContent, utilities.Templates.Components.ModalError, content)
		return
	}

	timeNow := time.Now()
	month := timeNow.Month()
	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	monthlyExpense, err := h.DB.GetTotalHouseExpenseForMonth(timeNow, userID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.Modal, err)
		return
	}

	highestExpense, utilType, err := h.DB.GetHighestHouseExpenseForMonth(month, userID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.Modal, err)
		return
	}

	pageData := &models.HouseExpResponse{
		MonthlyExpense: &models.MonthlyExpense{
			Amount: monthlyExpense,
			Month:  month.String(),
			IsOOB:  true,
		},
		HighestExpense: &models.HighestExpense{
			Amount: highestExpense,
			Type:   utilType,
			IsOOB:  true,
		},
		Modal: &models.ModalContent{
			Title:   "Successfully deleted expense!",
			Message: fmt.Sprintf("Expense with ID: %v deleted!", id),
		},
	}

	c.HTML(http.StatusOK, utilities.Templates.Responses.DeleteHouseExp, pageData)
}
