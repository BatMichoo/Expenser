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

type CarHandler struct {
	DB *database.DB
}

func NewCarHandler(db *database.DB) *CarHandler {
	return &CarHandler{
		DB: db,
	}
}

func (h *CarHandler) getCarPageData(c *gin.Context) (*models.CarData, bool, error) {
	dateNow := time.Now()
	month := dateNow.Month()
	year := dateNow.Year()

	userIDstr, exists := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	highestExpense, utilType, err := h.DB.GetHighestCarExpenseForMonth(month, userID)
	if err != nil {
		return nil, exists, fmt.Errorf("error fetching highest car expense: %w", err)
	}

	monthlyExpense, err := h.DB.GetTotalCarExpenseForMonth(month, userID)
	if err != nil {
		return nil, exists, fmt.Errorf("error fetching total car expense: %w", err)
	}

	recentExpenses, err := h.DB.GetCarExpensesForMonth(month, year, userID)
	if err != nil {
		return nil, exists, fmt.Errorf("error fetching recent car expenses: %w", err)
	}

	return &models.CarData{
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
	}, exists, nil
}

func (h *CarHandler) GetHome(c *gin.Context) {
	pageData, exists, err := h.getCarPageData(c)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("500: %v", err),
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Pages.Car, pageData)
		return
	} else {
		rl := &models.RootLayout{
			TemplateName:    utilities.Templates.Pages.Car,
			TemplateContent: pageData,
			HeaderOpts: &models.HeaderOptions{
				IsLoggedIn: exists,
			},
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

func (h *CarHandler) GetCurrentMonth(c *gin.Context) {
	pageData, exists, err := h.getCarPageData(c)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("500: %v", err),
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Components.CarCurrent, pageData)
		return
	} else {
		rl := &models.RootLayout{
			TemplateName:    utilities.Templates.Pages.Car,
			TemplateContent: pageData,
			HeaderOpts: &models.HeaderOptions{
				IsLoggedIn: exists,
			},
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

func (h *CarHandler) GetCreateCarForm(c *gin.Context) {
	expTypes, err := h.DB.GetCarExpenseTypes()
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("500: %v", err),
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.CreateCarExpForm, expTypes)
}

// CreateCarExpense handles the HTTP POST request to create a new home expense.
// It parses form data for expense type, date, amount, and notes,
// validates them, saves the new expense to the database,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *CarHandler) CreateCarExpense(c *gin.Context) {
	expTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	date, err := time.Parse("2006-01-02", c.Request.PostFormValue("date"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
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
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	timeNow := time.Now()

	highestExp, expType, err := h.DB.GetHighestCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	montlyTotal, err := h.DB.GetTotalCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	crExpResp := &models.CarExpResponse{
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
			Message: fmt.Sprintf("%s: %v BGN", newExpense.Type, newExpense.Amount),
		},
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateCarExp, crExpResp)
}

// TODO: Find a use for this
func (h *CarHandler) GetCarExpenseById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	exp, err := h.DB.GetCarExpenseByID(id)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	c.HTML(http.StatusOK, "expense", exp)
}

// INFO: UPDATE

// GetEditCarForm renders the HTML form pre-filled with existing expense data
// for editing a specific home expense.
// It expects the expense ID to be provided as a query parameter.
func (h *CarHandler) GetEditCarForm(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	exp, err := h.DB.GetCarExpenseByID(id)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	expTypes, err := h.DB.GetCarExpenseTypes()
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	formData := &models.EditCarFormData{
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
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	expTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	date, err := time.Parse("2006-01-02", c.Request.PostFormValue("date"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	notes := c.Request.PostFormValue("notes")
	editExpense := &models.CarExpense{
		ID:            id,
		Amount:        amount,
		ExpenseTypeID: expTypeID,
		Date:          date,
		Notes:         notes,
	}

	err = h.DB.EditCarExpense(editExpense)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	timeNow := time.Now()

	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	highestExp, expType, err := h.DB.GetHighestCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	montlyTotal, err := h.DB.GetTotalCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	edExpResp := &models.CarExpResponse{
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
			Title:   "Successful expense update.",
			Message: fmt.Sprintf("%s: %v BGN", editExpense.Type, editExpense.Amount),
		},
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateCarExp, edExpResp)
}

// INFO: DELETE

func (h *CarHandler) GetDeleteConfirm(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	content := &models.ModalConfirmContent{
		Title:    "Are you sure you want to delete this?",
		Method:   "DELETE",
		Endpoint: template.URL(fmt.Sprintf("/car/expenses/%v", id)),
		Target:   fmt.Sprintf("#exp-%v", id),
		Message:  fmt.Sprintf("Please confirm if you want to delete expense with ID: %v", id),
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.ModalConfirm, content)
}

// DeleteCarExp handles the HTTP DELETE request to remove a home expense by its ID.
// After successfully deleting the expense, it updates and returns
// the current month's total and highest expense summaries to reflect the change.
// It responds with 204 No Content if the expense was not found or not deleted,
// or 200 OK with updated summary data otherwise.
func (h *CarHandler) DeleteCarExp(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	res, err := h.DB.DeleteCarExpense(id)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("400: %v", err),
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
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
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("500: %v", err),
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	highestExpense, utilType, err := h.DB.GetHighestCarExpenseForMonth(month, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   "Something went wrong!",
			Message: fmt.Sprintf("500: %v", err),
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	pageData := &models.CarExpResponse{
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

	c.HTML(http.StatusOK, utilities.Templates.Responses.DeleteCarExp, pageData)
}
