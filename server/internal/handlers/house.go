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
	"time"

	"github.com/gin-gonic/gin"
)

// HouseHandler serves the house section: month view and expense forms.
type HouseHandler struct {
	store  HouseStore
	months *services.MonthViewService
}

// parseHouseExpenseForm reads the fields shared by the create and edit forms.
func (h *HouseHandler) parseHouseExpenseForm(c *gin.Context) (*models.HouseExpense, error) {
	if err := c.Request.ParseForm(); err != nil {
		return nil, badRequest("error.invalid_form", err)
	}
	f, err := parseExpenseFields(c.Request.PostForm)
	if err != nil {
		return nil, err
	}
	return &models.HouseExpense{
		Amount:         f.Amount,
		DiscountAmount: f.Discount,
		UtilityTypeID:  f.TypeID,
		ExpenseDate:    f.Date,
		Notes:          f.Notes,
		Metadata:       h.parseHouseMetadata(c, f.TypeID),
	}, nil
}

// INFO: CREATE

// GetCreateHouseForm renders the HTML form for users to input details
// for a new home expense.
// This handler serves the UI component for expense creation.
func (h *HouseHandler) GetCreateHouseForm(c *gin.Context) error {
	expTypes, err := h.store.GetHouseUtilityTypes(c.Request.Context())
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.CreateHouseExpForm, gin.H{
		"Types": expTypes,
		"Lang":  currentLang(c),
		"Year":  time.Now().Year(),
		"Date":  time.Now().Format(utilities.DateFormats.HTML),
		"Month": time.Now().Format(utilities.DateFormats.MonthOnly), // latest month a backfill may reach
	})
	return nil
}

func (h *HouseHandler) parseHouseMetadata(c *gin.Context, typeID int) models.Metadata {
	var metadata any

	switch typeID {
	case models.UtilityElectricity:
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
	case models.UtilityWater:
		qty, _ := strconv.ParseFloat(c.Request.PostFormValue("quantity"), 64)
		price, _ := strconv.ParseFloat(c.Request.PostFormValue("pricePerUnit"), 64)
		metadata = models.WaterMetadata{
			Quantity:     qty,
			PricePerUnit: price,
		}
	case models.UtilityGas:
		qty, _ := strconv.ParseFloat(c.Request.PostFormValue("quantity"), 64)
		price, _ := strconv.ParseFloat(c.Request.PostFormValue("pricePerUnit"), 64)
		metadata = models.GasMetadata{
			Quantity:     qty,
			PricePerUnit: price,
		}
	case models.UtilityInternet, models.UtilityTV:
		metadata = models.InternetTVMetadata{
			Provider: c.Request.PostFormValue("provider"),
			Plan:     c.Request.PostFormValue("plan"),
		}
	case models.UtilityOther:
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
	return renderMetadataFields(c, houseMetadataTemplates[typeID])
}

// houseMetadataTemplates are the extra form fields of each utility type
// that records metadata.
var houseMetadataTemplates = map[int]string{
	models.UtilityElectricity: "house-metadata-electricity",
	models.UtilityWater:       "house-metadata-water",
	models.UtilityGas:         "house-metadata-gas",
	models.UtilityInternet:    "house-metadata-internettv",
	models.UtilityTV:          "house-metadata-internettv",
	models.UtilityOther:       "house-metadata-other",
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

	if err := h.store.CreateHouseExpense(c.Request.Context(), newExpense); err != nil {
		return err
	}

	title := utilities.T(lang, "modal.create_success")
	message := fmt.Sprintf("%s: %v %s", newExpense.UtilityType, newExpense.Amount, utilities.Currency)

	expensesChanged(c)
	renderSuccess(c, http.StatusCreated, title, message)
	return nil
}

// INFO: READ

// GetCurrentMonth renders the current month's summary and expenses.
// HTMX requests get only the summary fragment; full page loads get the
// whole house page.
func (h *HouseHandler) GetCurrentMonth(c *gin.Context) error {
	pageData, err := h.monthView(c)
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
	pageData, err := h.monthView(c)
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
	id, err := paramID(c)
	if err != nil {
		return err
	}

	exp, err := h.store.GetHouseExpenseByID(c.Request.Context(), id, currentUserID(c))
	if err != nil {
		return err
	}
	if exp == nil {
		return notFound("error.expense_not_found")
	}

	expTypes, err := h.store.GetHouseUtilityTypes(c.Request.Context())
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

	id, err := paramID(c)
	if err != nil {
		return err
	}

	editExpense, err := h.parseHouseExpenseForm(c)
	if err != nil {
		return err
	}
	editExpense.ID = id

	if err := h.store.EditHouseExpense(c.Request.Context(), editExpense, currentUserID(c)); err != nil {
		return err
	}

	expensesChanged(c)
	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateHouseExp, &models.HouseExpResponse{
		Expense: editExpense,
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
	id, err := paramID(c)
	if err != nil {
		return err
	}

	exp, err := h.store.GetHouseExpenseByID(c.Request.Context(), id, currentUserID(c))
	if err != nil {
		return err
	}
	if exp == nil {
		return notFound("error.expense_not_found")
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.ModalConfirm, &models.ModalConfirmContent{
		Title:    utilities.T(lang, "modal.delete_confirm"),
		Method:   "DELETE",
		Endpoint: template.URL(fmt.Sprintf("/house/expenses/%v", id)),
		Target:   fmt.Sprintf("#exp-%v", id),
		Message: fmt.Sprintf(utilities.T(lang, "modal.delete_expense_message"),
			utilities.T(lang, "type."+exp.UtilityType), exp.Amount, utilities.CurrencySymbol(), exp.ExpenseDate.Format("02.01.2006")),
		ConfirmLabel: utilities.T(lang, "common.delete"),
		Lang:         lang,
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

	id, err := paramID(c)
	if err != nil {
		return err
	}

	deleted, err := h.store.DeleteHouseExpense(c.Request.Context(), id, currentUserID(c))
	if err != nil {
		return err
	}
	if !deleted {
		c.Status(http.StatusNoContent)
		return nil
	}

	expensesChanged(c)
	c.HTML(http.StatusOK, utilities.Templates.Responses.DeleteHouseExp, &models.HouseExpResponse{
		Modal: &models.ModalContent{
			Title:   utilities.T(lang, "modal.delete_success"),
			Message: fmt.Sprintf(utilities.T(lang, "modal.delete_success_message"), id),
			Lang:    lang,
		},
		Lang: lang,
	})
	return nil
}

// monthView builds the page for the month, search and type filter in the
// request's query string (current month when missing).
func (h *HouseHandler) monthView(c *gin.Context) (*models.MonthView, error) {
	now := time.Now()
	ym := services.ParseYearMonth(c.Query("month"), now)
	return h.months.House(c.Request.Context(), currentUserID(c), ym, monthFilter(c), currentLang(c), now)
}
