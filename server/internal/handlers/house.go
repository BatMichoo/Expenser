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

type HouseHandler struct {
	BaseHandler BaseHandler
}

// NewHouseHandler creates and returns a new instance of HomeHandler.
// It requires a database connection pool to operate.
func NewHouseHandler(rh *RootHandler) *HouseHandler {
	return &HouseHandler{
		BaseHandler: rh.BaseHandler,
	}
}

func (h *HouseHandler) getHousePageData(c *gin.Context) (*models.HouseData, error) {
	dateNow := time.Now()
	month := dateNow.Month()
	year := dateNow.Year()
	userID := currentUserID(c)

	highestExpense, utilType, err := h.BaseHandler.DB.GetHighestHouseExpenseForMonth(c.Request.Context(), month, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching highest house expense: %w", err)
	}

	monthlyExpense, err := h.BaseHandler.DB.GetTotalHouseExpenseForMonth(c.Request.Context(), dateNow, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching total house expense: %w", err)
	}

	recentExpenses, err := h.BaseHandler.DB.GetHouseExpensesForMonth(c.Request.Context(), month, year, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching recent house expenses: %w", err)
	}

	return &models.HouseData{
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
func (h *HouseHandler) monthSummary(ctx context.Context, userID uuid.UUID) (*models.HighestExpense, *models.MonthlyExpense, error) {
	timeNow := time.Now()

	highestExp, expType, err := h.BaseHandler.DB.GetHighestHouseExpenseForMonth(ctx, timeNow.Month(), userID)
	if err != nil {
		return nil, nil, err
	}

	monthlyTotal, err := h.BaseHandler.DB.GetTotalHouseExpenseForMonth(ctx, timeNow, userID)
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
		Month:  timeNow.Month().String(),
		IsOOB:  true,
	}
	return highest, monthly, nil
}

// parseHouseExpenseForm reads the fields shared by the create and edit forms.
func (h *HouseHandler) parseHouseExpenseForm(c *gin.Context) (*models.HouseExpense, error) {
	utilTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
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

	return &models.HouseExpense{
		Amount:         amount,
		DiscountAmount: discountAmount,
		UtilityTypeID:  utilTypeID,
		ExpenseDate:    date,
		Notes:          c.Request.PostFormValue("notes"),
		Metadata:       h.parseHouseMetadata(c, utilTypeID),
	}, nil
}

// INFO: CREATE

// GetCreateHouseForm renders the HTML form for users to input details
// for a new home expense.
// This handler serves the UI component for expense creation.
func (h *HouseHandler) GetCreateHouseForm(c *gin.Context) error {
	expTypes, err := h.BaseHandler.DB.GetHouseUtilityTypes(c.Request.Context())
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.CreateHouseExpForm, gin.H{
		"Types": expTypes,
		"Lang":  currentLang(c),
		"Year":  time.Now().Year(),
		"Date":  time.Now().Format(utilities.DateFormats.HTML),
	})
	return nil
}

func (h *HouseHandler) parseHouseMetadata(c *gin.Context, typeID int) models.Metadata {
	var metadata any

	switch typeID {
	case 1: // Electricity
		kWhDay, _ := strconv.ParseFloat(c.Request.PostFormValue("kWhDay"), 64)
		kWhNight, _ := strconv.ParseFloat(c.Request.PostFormValue("kWhNight"), 64)
		priceDay, _ := strconv.ParseFloat(c.Request.PostFormValue("priceDay"), 64)
		priceNight, _ := strconv.ParseFloat(c.Request.PostFormValue("priceNight"), 64)
		metadata = models.ElectricityMetadata{
			KWhDay:     kWhDay,
			KWhNight:   kWhNight,
			PriceDay:   priceDay,
			PriceNight: priceNight,
		}
	case 2: // Water
		qty, _ := strconv.ParseFloat(c.Request.PostFormValue("quantity"), 64)
		price, _ := strconv.ParseFloat(c.Request.PostFormValue("pricePerUnit"), 64)
		metadata = models.WaterMetadata{
			Quantity:     qty,
			PricePerUnit: price,
		}
	case 3: // Gas
		qty, _ := strconv.ParseFloat(c.Request.PostFormValue("quantity"), 64)
		price, _ := strconv.ParseFloat(c.Request.PostFormValue("pricePerUnit"), 64)
		metadata = models.GasMetadata{
			Quantity:     qty,
			PricePerUnit: price,
		}
	case 4, 5: // Internet, TV
		metadata = models.InternetTVMetadata{
			Provider: c.Request.PostFormValue("provider"),
			Plan:     c.Request.PostFormValue("plan"),
		}
	case 7: // Other
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
func (h *HouseHandler) GetMetadataFields(c *gin.Context) error {
	typeID, _ := strconv.Atoi(c.Query("typeID"))

	// We'll map type IDs to template names
	var templateName string
	switch typeID {
	case 1:
		templateName = "house-metadata-electricity"
	case 2:
		templateName = "house-metadata-water"
	case 3:
		templateName = "house-metadata-gas"
	case 4, 5:
		templateName = "house-metadata-internettv"
	case 7:
		templateName = "house-metadata-other"
	default:
		c.Status(http.StatusOK)
		return nil
	}

	c.HTML(http.StatusOK, templateName, gin.H{"Lang": currentLang(c)})
	return nil
}

// CreateHouseExpense handles the HTTP POST request to create a new home expense.
// It parses form data for expense type, date, amount, and notes,
// validates them, saves the new expense to the database,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *HouseHandler) CreateHouseExpense(c *gin.Context) error {
	lang := currentLang(c)
	userID := currentUserID(c)

	newExpense, err := h.parseHouseExpenseForm(c)
	if err != nil {
		return err
	}
	newExpense.CreatedBy = userID

	if err := h.BaseHandler.DB.CreateHouseExpense(c.Request.Context(), newExpense); err != nil {
		return err
	}

	title := utilities.T(lang, "modal.create_success")
	message := fmt.Sprintf("%s: %v %s", newExpense.UtilityType, newExpense.Amount, utilities.Currency)

	if newExpense.ExpenseDate.Month() != time.Now().Month() {
		renderSuccess(c, http.StatusCreated, title, message)
		return nil
	}

	highest, monthly, err := h.monthSummary(c.Request.Context(), userID)
	if err != nil {
		return err
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateHouseExp, &models.HouseExpResponse{
		Expense:        newExpense,
		HighestExpense: highest,
		MonthlyExpense: monthly,
		Modal:          &models.ModalContent{Title: title, Message: message, Lang: lang},
		Lang:           lang,
	})
	return nil
}

// INFO: READ

// GetCurrentMonth renders the current month's summary and expenses.
// HTMX requests get only the summary fragment; full page loads get the
// whole house page.
func (h *HouseHandler) GetCurrentMonth(c *gin.Context) error {
	pageData, err := h.getHousePageData(c)
	if err != nil {
		return err
	}

	renderView(c, utilities.Templates.Components.HouseCurrent, utilities.Templates.Pages.House, pageData)
	return nil
}

// GetHome renders the main home dashboard page.
// It fetches the highest expense, total monthly expense, and a list of
// recent expenses for the current month and year from the database.
func (h *HouseHandler) GetHome(c *gin.Context) error {
	pageData, err := h.getHousePageData(c)
	if err != nil {
		return err
	}

	renderPage(c, utilities.Templates.Pages.House, pageData)
	return nil
}

// INFO: UPDATE

// GetEditHouseForm renders the HTML form pre-filled with existing expense data
// for editing a specific home expense.
// It expects the expense ID to be provided as a path parameter.
func (h *HouseHandler) GetEditHouseForm(c *gin.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	exp, err := h.BaseHandler.DB.GetHouseExpenseByID(c.Request.Context(), id, currentUserID(c))
	if err != nil {
		return err
	}
	if exp == nil {
		return notFound("error.expense_not_found")
	}

	expTypes, err := h.BaseHandler.DB.GetHouseUtilityTypes(c.Request.Context())
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditHouseExpForm, &models.EditHouseFormData{
		Expense:  exp,
		Types:    expTypes,
		Metadata: exp.UnmarshalMetadata(),
		Lang:     currentLang(c),
	})
	return nil
}

// EditHouseExpenseById handles the HTTP PUT/POST request to update an existing home expense.
// It parses form data for updated expense type, date, amount, and notes,
// validates them, updates the expense in the database by its ID,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *HouseHandler) EditHouseExpenseById(c *gin.Context) error {
	lang := currentLang(c)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	editExpense, err := h.parseHouseExpenseForm(c)
	if err != nil {
		return err
	}
	editExpense.ID = id

	if err := h.BaseHandler.DB.EditHouseExpense(c.Request.Context(), editExpense, currentUserID(c)); err != nil {
		return err
	}

	highest, monthly, err := h.monthSummary(c.Request.Context(), currentUserID(c))
	if err != nil {
		return err
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateHouseExp, &models.HouseExpResponse{
		Expense:        editExpense,
		HighestExpense: highest,
		MonthlyExpense: monthly,
		Modal: &models.ModalContent{
			Title:   utilities.T(lang, "modal.update_success"),
			Message: fmt.Sprintf(utilities.T(lang, "modal.update_success_message"), editExpense.ID),
			Lang:    lang,
		},
		Lang: lang,
	})
	return nil
}

// INFO: DELETE

func (h *HouseHandler) GetDeleteConfirm(c *gin.Context) error {
	lang := currentLang(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.ModalConfirm, &models.ModalConfirmContent{
		Title:    utilities.T(lang, "modal.delete_confirm"),
		Method:   "DELETE",
		Endpoint: template.URL(fmt.Sprintf("/house/expenses/%v", id)),
		Target:   fmt.Sprintf("#exp-%v", id),
		Message:  fmt.Sprintf(utilities.T(lang, "modal.delete_message"), id),
		Lang:     lang,
	})
	return nil
}

// DeleteHouseExp handles the HTTP DELETE request to remove a home expense by its ID.
// After successfully deleting the expense, it updates and returns
// the current month's total and highest expense summaries to reflect the change.
// It responds with 204 No Content if the expense was not found or not deleted,
// or 200 OK with updated summary data otherwise.
func (h *HouseHandler) DeleteHouseExp(c *gin.Context) error {
	lang := currentLang(c)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	deleted, err := h.BaseHandler.DB.DeleteHouseExpense(c.Request.Context(), id, currentUserID(c))
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

	c.HTML(http.StatusOK, utilities.Templates.Responses.DeleteHouseExp, &models.HouseExpResponse{
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
