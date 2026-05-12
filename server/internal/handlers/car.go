package handlers

import (
	"encoding/json"
	database "expenser/internal/db"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
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
		Lang:           c.GetString("lang"),
	}, exists, nil
}

func (h *CarHandler) GetHome(c *gin.Context) {
	lang := c.GetString("lang")
	pageData, exists, err := h.getCarPageData(c)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("500: %v", err),
			Lang:    lang,
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
				Lang:       lang,
			},
			Lang: lang,
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

func (h *CarHandler) GetCurrentMonth(c *gin.Context) {
	lang := c.GetString("lang")
	pageData, exists, err := h.getCarPageData(c)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("500: %v", err),
			Lang:    lang,
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
				Lang:       lang,
			},
			Lang: lang,
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

func (h *CarHandler) GetCreateCarForm(c *gin.Context) {
	lang := c.GetString("lang")
	expTypes, err := h.DB.GetCarExpenseTypes()
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("500: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.CreateCarExpForm, gin.H{
		"Types": expTypes,
		"Lang":  lang,
	})
}

func (h *CarHandler) parseCarMetadata(c *gin.Context, typeID int) json.RawMessage {
	var metadata interface{}

	switch typeID {
	case 1: // Fuel
		liters, _ := strconv.ParseFloat(c.Request.PostFormValue("liters"), 64)
		price, _ := strconv.ParseFloat(c.Request.PostFormValue("pricePerLiter"), 64)
		metadata = models.FuelMetadata{
			Liters:        liters,
			PricePerLiter: price,
		}
	case 2: // Maintenance/Repair
		parts := c.Request.PostForm["parts"]
		otherParts := c.Request.PostFormValue("otherParts")
		if otherParts != "" {
			for _, p := range strings.Split(otherParts, ",") {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" {
					parts = append(parts, trimmed)
				}
			}
		}
		labor, _ := strconv.ParseFloat(c.Request.PostFormValue("laborCost"), 64)
		metadata = models.MaintenanceMetadata{
			Parts:     parts,
			LaborCost: labor,
		}
	case 3: // Insurance
		validUntil, _ := time.Parse("2006-01-02", c.Request.PostFormValue("validUntil"))
		metadata = models.InsuranceMetadata{
			Provider:     c.Request.PostFormValue("provider"),
			CoverageType: c.Request.PostFormValue("coverageType"),
			ValidUntil:   validUntil,
		}
	case 5: // Parking/Tolls
		metadata = models.ParkingTollsMetadata{
			Location: c.Request.PostFormValue("location"),
			Duration: c.Request.PostFormValue("duration"),
		}
	default:
		return nil
	}

	data, _ := json.Marshal(metadata)
	return data
}

// GetMetadataFields returns the HTML fragment for specific metadata fields based on typeID
func (h *CarHandler) GetMetadataFields(c *gin.Context) {
	typeID, _ := strconv.Atoi(c.Query("typeID"))

	var templateName string
	switch typeID {
	case 1:
		templateName = "car-metadata-fuel"
	case 2:
		templateName = "car-metadata-maintenance"
	case 3:
		templateName = "car-metadata-insurance"
	case 5:
		templateName = "car-metadata-parkingtolls"
	default:
		c.Status(http.StatusOK)
		return
	}

	c.HTML(http.StatusOK, templateName, gin.H{"Lang": c.GetString("lang")})
}

// CreateCarExpense handles the HTTP POST request to create a new home expense.
// It parses form data for expense type, date, amount, and notes,
// validates them, saves the new expense to the database,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *CarHandler) CreateCarExpense(c *gin.Context) {
	lang := c.GetString("lang")
	expTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	date, err := time.Parse("2006-01-02", c.Request.PostFormValue("date"))
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	notes := c.Request.PostFormValue("notes")
	metadata := h.parseCarMetadata(c, expTypeID)

	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	newExpense := &models.CarExpense{
		Amount:        amount,
		ExpenseTypeID: expTypeID,
		Date:          date,
		Notes:         notes,
		Metadata:      metadata,
		CreatedBy:     userID,
	}

	err = h.DB.CreateCarExpense(newExpense)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	timeNow := time.Now()

	highestExp, expType, err := h.DB.GetHighestCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	montlyTotal, err := h.DB.GetTotalCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
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
			Title:   utilities.T(lang, "modal.create_success"),
			Message: fmt.Sprintf("%s: %v BGN", newExpense.Type, newExpense.Amount),
			Lang:    lang,
		},
		Lang: lang,
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateCarExp, crExpResp)
}

// TODO: Find a use for this
func (h *CarHandler) GetCarExpenseById(c *gin.Context) {
	lang := c.GetString("lang")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	exp, err := h.DB.GetCarExpenseByID(id)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
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
	lang := c.GetString("lang")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	exp, err := h.DB.GetCarExpenseByID(id)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	expTypes, err := h.DB.GetCarExpenseTypes()
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	formData := &models.EditCarFormData{
		Expense:  exp,
		Types:    expTypes,
		Metadata: exp.UnmarshalMetadata(),
		Lang:     lang,
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditCarExpForm, formData)
}

// EditCarExpenseById handles the HTTP PUT/POST request to update an existing home expense.
// It parses form data for updated expense type, date, amount, and notes,
// validates them, updates the expense in the database by its ID,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *CarHandler) EditCarExpenseById(c *gin.Context) {
	lang := c.GetString("lang")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	expTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	date, err := time.Parse("2006-01-02", c.Request.PostFormValue("date"))
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	notes := c.Request.PostFormValue("notes")
	metadata := h.parseCarMetadata(c, expTypeID)

	editExpense := &models.CarExpense{
		ID:            id,
		Amount:        amount,
		ExpenseTypeID: expTypeID,
		Date:          date,
		Notes:         notes,
		Metadata:      metadata,
	}

	err = h.DB.EditCarExpense(editExpense)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
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
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	montlyTotal, err := h.DB.GetTotalCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
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
			Title:   utilities.T(lang, "modal.update_success"),
			Message: fmt.Sprintf("%s: %v BGN", editExpense.Type, editExpense.Amount),
			Lang:    lang,
		},
		Lang: lang,
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateCarExp, edExpResp)
}

// INFO: DELETE

func (h *CarHandler) GetDeleteConfirm(c *gin.Context) {
	lang := c.GetString("lang")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	content := &models.ModalConfirmContent{
		Title:    utilities.T(lang, "modal.delete_confirm"),
		Method:   "DELETE",
		Endpoint: template.URL(fmt.Sprintf("/car/expenses/%v", id)),
		Target:   fmt.Sprintf("#exp-%v", id),
		Message:  fmt.Sprintf(utilities.T(lang, "modal.delete_message"), id),
		Lang:     lang,
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.ModalConfirm, content)
}

// DeleteCarExp handles the HTTP DELETE request to remove a home expense by its ID.
// After successfully deleting the expense, it updates and returns
// the current month's total and highest expense summaries to reflect the change.
// It responds with 204 No Content if the expense was not found or not deleted,
// or 200 OK with updated summary data otherwise.
func (h *CarHandler) DeleteCarExp(c *gin.Context) {
	lang := c.GetString("lang")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	res, err := h.DB.DeleteCarExpense(id)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
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
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("500: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	highestExpense, utilType, err := h.DB.GetHighestCarExpenseForMonth(month, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("500: %v", err),
			Lang:    lang,
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
			Title:   utilities.T(lang, "modal.delete_success"),
			Message: fmt.Sprintf(utilities.T(lang, "modal.delete_success_message"), id),
			Lang:    lang,
		},
		Lang: lang,
	}

	c.HTML(http.StatusOK, utilities.Templates.Responses.DeleteCarExp, pageData)
}
