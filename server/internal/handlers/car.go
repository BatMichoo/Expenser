package handlers

import (
	"context"
	"encoding/json"
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
	userID := currentUserID(c)

	highestExpense, utilType, err := h.BaseHandler.DB.GetHighestCarExpenseForMonth(c.Request.Context(), month, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching highest car expense: %w", err)
	}

	monthlyExpense, err := h.BaseHandler.DB.GetTotalCarExpenseForMonth(c.Request.Context(), month, year, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching total car expense: %w", err)
	}

	recentExpenses, err := h.BaseHandler.DB.GetCarExpensesForMonth(c.Request.Context(), month, year, userID)
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
		Lang:           currentLang(c),
	}, nil
}

// monthSummary returns the current month's highest and total expense as
// out-of-band fragments, used to refresh the summary cards after a change.
func (h *CarHandler) monthSummary(ctx context.Context, userID uuid.UUID) (*models.HighestExpense, *models.MonthlyExpense, error) {
	month := time.Now().Month()

	highestExp, expType, err := h.BaseHandler.DB.GetHighestCarExpenseForMonth(ctx, month, userID)
	if err != nil {
		return nil, nil, err
	}

	monthlyTotal, err := h.BaseHandler.DB.GetTotalCarExpenseForMonth(ctx, month, time.Now().Year(), userID)
	if err != nil {
		return nil, nil, err
	}

	highest := &models.HighestExpense{
		Amount: highestExp,
		Type:   strings.TrimSpace(expType),
		IsOOB:  true,
	}
	monthly := &models.MonthlyExpense{
		Amount: monthlyTotal,
		Month:  month.String(),
		IsOOB:  true,
	}
	return highest, monthly, nil
}

// parseCarExpenseForm reads the fields shared by the create and edit forms.
func (h *CarHandler) parseCarExpenseForm(c *gin.Context) (*models.CarExpense, error) {
	expTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		return nil, badRequest("error.invalid_type", err)
	}

	date, err := time.Parse(utilities.DateFormats.Input, c.Request.PostFormValue("date"))
	if err != nil {
		return nil, badRequest("error.invalid_date", err)
	}

	amount, err := strconv.ParseFloat(c.Request.PostFormValue("amount"), 64)
	if err != nil {
		return nil, badRequest("error.invalid_amount", err)
	}
	if amount <= 0 {
		return nil, badRequest("error.amount_not_positive", fmt.Errorf("amount %v", amount))
	}

	discountAmount, _ := strconv.ParseFloat(c.Request.PostFormValue("discount_amount"), 64)

	return &models.CarExpense{
		Amount:         amount,
		DiscountAmount: discountAmount,
		ExpenseTypeID:  expTypeID,
		Date:           date,
		Notes:          c.Request.PostFormValue("notes"),
		Metadata:       h.parseCarMetadata(c, expTypeID),
	}, nil
}

func (h *CarHandler) GetHome(c *gin.Context) error {
	pageData, err := h.getCarPageData(c)
	if err != nil {
		return err
	}

	renderPage(c, utilities.Templates.Pages.Car, pageData)
	return nil
}

// GetCurrentMonth renders the current month's summary and expenses.
// HTMX requests get only the summary fragment; full page loads get the
// whole car page.
func (h *CarHandler) GetCurrentMonth(c *gin.Context) error {
	pageData, err := h.getCarPageData(c)
	if err != nil {
		return err
	}

	renderView(c, utilities.Templates.Components.CarCurrent, utilities.Templates.Pages.Car, pageData)
	return nil
}

func (h *CarHandler) GetCreateCarForm(c *gin.Context) error {
	expTypes, err := h.BaseHandler.DB.GetCarExpenseTypes(c.Request.Context())
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.CreateCarExpForm, gin.H{
		"Types": expTypes,
		"Lang":  currentLang(c),
	})
	return nil
}

func (h *CarHandler) parseCarMetadata(c *gin.Context, typeID int) models.Metadata {
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
func (h *CarHandler) GetMetadataFields(c *gin.Context) error {
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
		return nil
	}

	c.HTML(http.StatusOK, templateName, gin.H{"Lang": currentLang(c)})
	return nil
}

// CreateCarExpense handles the HTTP POST request to create a new car expense.
// It parses form data for expense type, date, amount, and notes,
// validates them, saves the new expense to the database,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *CarHandler) CreateCarExpense(c *gin.Context) error {
	lang := currentLang(c)
	userID := currentUserID(c)

	newExpense, err := h.parseCarExpenseForm(c)
	if err != nil {
		return err
	}
	newExpense.CreatedBy = userID

	if err := h.BaseHandler.DB.CreateCarExpense(c.Request.Context(), newExpense); err != nil {
		return err
	}

	modal := &models.ModalContent{
		Title:   utilities.T(lang, "modal.create_success"),
		Message: fmt.Sprintf("%s: %v %s", newExpense.Type, newExpense.Amount, utilities.Currency),
		Lang:    lang,
	}

	if newExpense.Date.Month() != time.Now().Month() {
		renderSuccess(c, http.StatusCreated, modal.Title, modal.Message)
		return nil
	}

	highest, monthly, err := h.monthSummary(c.Request.Context(), userID)
	if err != nil {
		return err
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateCarExp, &models.CarExpResponse{
		Expense:        newExpense,
		HighestExpense: highest,
		MonthlyExpense: monthly,
		Modal:          modal,
		Lang:           lang,
	})
	return nil
}

// INFO: UPDATE

// GetEditCarForm renders the HTML form pre-filled with existing expense data
// for editing a specific car expense.
// It expects the expense ID to be provided as a path parameter.
func (h *CarHandler) GetEditCarForm(c *gin.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	exp, err := h.BaseHandler.DB.GetCarExpenseByID(c.Request.Context(), id, currentUserID(c))
	if err != nil {
		return err
	}
	if exp == nil {
		return notFound("error.expense_not_found")
	}

	expTypes, err := h.BaseHandler.DB.GetCarExpenseTypes(c.Request.Context())
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditCarExpForm, &models.EditCarFormData{
		Expense:  exp,
		Types:    expTypes,
		Metadata: exp.UnmarshalMetadata(),
		Lang:     currentLang(c),
	})
	return nil
}

// EditCarExpenseById handles the HTTP PUT/POST request to update an existing car expense.
// It parses form data for updated expense type, date, amount, and notes,
// validates them, updates the expense in the database by its ID,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *CarHandler) EditCarExpenseById(c *gin.Context) error {
	lang := currentLang(c)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	editExpense, err := h.parseCarExpenseForm(c)
	if err != nil {
		return err
	}
	editExpense.ID = id

	if err := h.BaseHandler.DB.EditCarExpense(c.Request.Context(), editExpense, currentUserID(c)); err != nil {
		return err
	}

	highest, monthly, err := h.monthSummary(c.Request.Context(), currentUserID(c))
	if err != nil {
		return err
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateCarExp, &models.CarExpResponse{
		Expense:        editExpense,
		HighestExpense: highest,
		MonthlyExpense: monthly,
		Modal: &models.ModalContent{
			Title:   utilities.T(lang, "modal.update_success"),
			Message: fmt.Sprintf("%s: %v %s", editExpense.Type, editExpense.Amount, utilities.Currency),
			Lang:    lang,
		},
		Lang: lang,
	})
	return nil
}

// INFO: DELETE

func (h *CarHandler) GetDeleteConfirm(c *gin.Context) error {
	lang := currentLang(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.ModalConfirm, &models.ModalConfirmContent{
		Title:    utilities.T(lang, "modal.delete_confirm"),
		Method:   "DELETE",
		Endpoint: template.URL(fmt.Sprintf("/car/expenses/%v", id)),
		Target:   fmt.Sprintf("#exp-%v", id),
		Message:  fmt.Sprintf(utilities.T(lang, "modal.delete_message"), id),
		Lang:     lang,
	})
	return nil
}

// DeleteCarExp handles the HTTP DELETE request to remove a car expense by its ID.
// After successfully deleting the expense, it updates and returns
// the current month's total and highest expense summaries to reflect the change.
// It responds with 204 No Content if the expense was not found or not deleted,
// or 200 OK with updated summary data otherwise.
func (h *CarHandler) DeleteCarExp(c *gin.Context) error {
	lang := currentLang(c)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	deleted, err := h.BaseHandler.DB.DeleteCarExpense(c.Request.Context(), id, currentUserID(c))
	if err != nil {
		return err
	}
	if !deleted {
		c.Status(http.StatusNoContent)
		return nil
	}

	highest, monthly, err := h.monthSummary(c.Request.Context(), currentUserID(c))
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Responses.DeleteCarExp, &models.CarExpResponse{
		HighestExpense: highest,
		MonthlyExpense: monthly,
		Modal: &models.ModalContent{
			Title:   utilities.T(lang, "modal.delete_success"),
			Message: fmt.Sprintf(utilities.T(lang, "modal.delete_success_message"), id),
			Lang:    lang,
		},
		Lang: lang,
	})
	return nil
}
