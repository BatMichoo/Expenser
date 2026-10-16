package handlers

import (
	"encoding/json"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type CarHandler struct {
	BaseHandler BaseHandler
}

func NewCarHandler(rh *RootHandler) *CarHandler {
	return &CarHandler{
		BaseHandler: rh.BaseHandler,
	}
}

func (h *CarHandler) getCarPageData(c *gin.Context) (*models.CarData, error) {
	dateNow := time.Now()
	month := dateNow.Month()
	year := dateNow.Year()

	// WARN: Needs to be valid as well?
	claims, _, lang := GetBasicDataFromContext(c, &h.BaseHandler)
	userID := claims.UserID

	highestExpense, utilType, err := h.BaseHandler.DB.GetHighestCarExpenseForMonth(month, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching highest car expense: %w", err)
	}

	monthlyExpense, err := h.BaseHandler.DB.GetTotalCarExpenseForMonth(month, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching total car expense: %w", err)
	}

	recentExpenses, err := h.BaseHandler.DB.GetCarExpensesForMonth(month, year, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching recent car expenses: %w", err)
	}

	return &models.CarData{
		Name: "current",
		MonthlyExpense: &models.MonthlyExpense{
			Amount: monthlyExpense,
			Month:  month.String(),
		},
		HighestExpense: &models.HighestExpense{
			Amount: highestExpense,
			Type:   strings.TrimSpace(utilType),
		},
		RecentExpenses: recentExpenses,
		Lang:           lang,
	}, nil
}

func (h *CarHandler) GetHome(c *gin.Context) {
	isLoggedIn := services.ClaimsFromContext(c) != nil
	pageData, err := h.getCarPageData(c)
	if err != nil {
		RenderErrorModal(c, err, http.StatusInternalServerError)
		return
	}

	RenderPage(c, isLoggedIn, utilities.Templates.Pages.Car, pageData)
}

func (h *CarHandler) GetCurrentMonth(c *gin.Context) {
	pageData, err := h.getCarPageData(c)
	if err != nil {
		RenderErrorModal(c, err, http.StatusInternalServerError)
		return
	}

	isLoggedIn := services.ClaimsFromContext(c) != nil
	RenderPage(c, isLoggedIn, utilities.Templates.Components.CarCurrent, pageData)
}

func (h *CarHandler) GetCreateCarForm(c *gin.Context) {
	lang := GetLanguage(c)
	expTypes, err := h.BaseHandler.DB.GetCarExpenseTypes()
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
	var metadata any

	switch typeID {
	case 1: // Fuel
		liters, _ := strconv.ParseFloat(c.Request.PostFormValue("liters"), 64)
		price, _ := strconv.ParseFloat(c.Request.PostFormValue("pricePerLiter"), 64)
		metadata = models.FuelMetadata{
			Liters:        liters,
			PricePerLiter: price,
		}
	case 2: // Maintenance/Repair
		var parts []string
		selectedPart := c.Request.PostFormValue("parts")
		if selectedPart != "" {
			parts = append(parts, selectedPart)
		}
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
		validUntil, _ := time.Parse(utilities.DateFormats.Input, c.Request.PostFormValue("validUntil"))
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
	case 6: // Other
		metadata = models.OtherMetadata{
			CustomName: c.Request.PostFormValue("customName"),
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
	case 6:
		templateName = "car-metadata-other"
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
	claims, _, lang := GetBasicDataFromContext(c, &h.BaseHandler)
	userID := claims.UserID

	expTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	date, err := time.Parse(utilities.DateFormats.Input, c.Request.PostFormValue("date"))
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	discountAmount, _ := strconv.ParseFloat(c.Request.PostFormValue("discount_amount"), 64)

	notes := c.Request.PostFormValue("notes")
	metadata := h.parseCarMetadata(c, expTypeID)

	newExpense := &models.CarExpense{
		Amount:         amount,
		DiscountAmount: discountAmount,
		ExpenseTypeID:  expTypeID,
		Date:           date,
		Notes:          notes,
		Metadata:       metadata,
		CreatedBy:      userID,
	}

	err = h.BaseHandler.DB.CreateCarExpense(newExpense)
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	timeNow := time.Now()

	highestExp, expType, err := h.BaseHandler.DB.GetHighestCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	montlyTotal, err := h.BaseHandler.DB.GetTotalCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	modal := &models.ModalContent{
		Title:   utilities.T(lang, "modal.create_success"),
		Message: fmt.Sprintf("%s: %v %s", newExpense.Type, newExpense.Amount, utilities.Currency),
		Lang:    lang,
	}

	if newExpense.Date.Month() != timeNow.Month() {
		c.HTML(http.StatusCreated, utilities.Templates.Components.ModalSuccess, modal)
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
		Modal: modal,
		Lang:  lang,
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateCarExp, crExpResp)
}

// TODO: Find a use for this
func (h *CarHandler) GetCarExpenseById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	exp, err := h.BaseHandler.DB.GetCarExpenseByID(id)
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	c.HTML(http.StatusOK, "expense", exp)
}

// INFO: UPDATE

// GetEditCarForm renders the HTML form pre-filled with existing expense data
// for editing a specific home expense.
// It expects the expense ID to be provided as a query parameter.
func (h *CarHandler) GetEditCarForm(c *gin.Context) {
	lang := GetLanguage(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	exp, err := h.BaseHandler.DB.GetCarExpenseByID(id)
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	expTypes, err := h.BaseHandler.DB.GetCarExpenseTypes()
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
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
	claims, _, lang := GetBasicDataFromContext(c, &h.BaseHandler)
	userID := claims.UserID

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	expTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	date, err := time.Parse(utilities.DateFormats.Input, c.Request.PostFormValue("date"))
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	discountAmount, _ := strconv.ParseFloat(c.Request.PostFormValue("discount_amount"), 64)

	notes := c.Request.PostFormValue("notes")
	metadata := h.parseCarMetadata(c, expTypeID)

	editExpense := &models.CarExpense{
		ID:             id,
		Amount:         amount,
		DiscountAmount: discountAmount,
		ExpenseTypeID:  expTypeID,
		Date:           date,
		Notes:          notes,
		Metadata:       metadata,
	}

	err = h.BaseHandler.DB.EditCarExpense(editExpense)
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	timeNow := time.Now()

	highestExp, expType, err := h.BaseHandler.DB.GetHighestCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	montlyTotal, err := h.BaseHandler.DB.GetTotalCarExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
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
			Message: fmt.Sprintf("%s: %v %s", editExpense.Type, editExpense.Amount, utilities.Currency),
			Lang:    lang,
		},
		Lang: lang,
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateCarExp, edExpResp)
}

// INFO: DELETE

func (h *CarHandler) GetDeleteConfirm(c *gin.Context) {
	lang := GetLanguage(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
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
	claims, _, lang := GetBasicDataFromContext(c, &h.BaseHandler)
	userID := claims.UserID

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	res, err := h.BaseHandler.DB.DeleteCarExpense(id)
	if err != nil {
		RenderErrorModal(c, err, http.StatusBadRequest)
		return
	}

	if !res {
		// If res is false, it means the expense was not found or not deleted.
		c.HTML(http.StatusNoContent, "", gin.H{})
		return
	}

	timeNow := time.Now()
	month := timeNow.Month()

	monthlyExpense, err := h.BaseHandler.DB.GetTotalCarExpenseForMonth(month, userID)
	if err != nil {
		RenderErrorModal(c, err, http.StatusInternalServerError)
		return
	}

	highestExpense, utilType, err := h.BaseHandler.DB.GetHighestCarExpenseForMonth(month, userID)
	if err != nil {
		RenderErrorModal(c, err, http.StatusInternalServerError)
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
			Type:   strings.TrimSpace(utilType),
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
