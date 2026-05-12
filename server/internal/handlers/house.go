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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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

func (h *HouseHandler) getHousePageData(c *gin.Context) (*models.HouseData, bool, error) {
	dateNow := time.Now()
	month := dateNow.Month()
	year := dateNow.Year()

	userIDstr, exists := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	highestExpense, utilType, err := h.DB.GetHighestHouseExpenseForMonth(month, userID)
	if err != nil {
		return nil, exists, fmt.Errorf("error fetching highest house expense: %w", err)
	}

	monthlyExpense, err := h.DB.GetTotalHouseExpenseForMonth(dateNow, userID)
	if err != nil {
		return nil, exists, fmt.Errorf("error fetching total house expense: %w", err)
	}

	recentExpenses, err := h.DB.GetHouseExpensesForMonth(month, year, userID)
	if err != nil {
		return nil, exists, fmt.Errorf("error fetching recent house expenses: %w", err)
	}

	return &models.HouseData{
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

// INFO: CREATE

// GetCreateHouseForm renders the HTML form for users to input details
// for a new home expense.
// This handler serves the UI component for expense creation.
func (h *HouseHandler) GetCreateHouseForm(c *gin.Context) {
	lang := c.GetString("lang")
	expTypes, err := h.DB.GetHouseUtilityTypes()
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("500: %v", err),
			Lang:    lang,
		}

		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.CreateHouseExpForm, gin.H{
		"Types": expTypes,
		"Lang":  lang,
	})
}

func (h *HouseHandler) parseHouseMetadata(c *gin.Context, typeID int) json.RawMessage {
	var metadata interface{}

	switch typeID {
	case 1: // Electricity
		kWhDay, _ := strconv.ParseFloat(c.Request.PostFormValue("kWhDay"), 64)
		kWhNight, _ := strconv.ParseFloat(c.Request.PostFormValue("kWhNight"), 64)
		priceDay, _ := strconv.ParseFloat(c.Request.PostFormValue("priceDay"), 64)
		priceNight, _ := strconv.ParseFloat(c.Request.PostFormValue("priceNight"), 64)
		metadata = models.ElectricityMetadata{
			KWhDay:     kWhDay,
			KWhNight:    kWhNight,
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
func (h *HouseHandler) GetMetadataFields(c *gin.Context) {
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
		return
	}

	c.HTML(http.StatusOK, templateName, gin.H{"Lang": c.GetString("lang")})
}

// CreateHouseExpense handles the HTTP POST request to create a new home expense.
// It parses form data for expense type, date, amount, and notes,
// validates them, saves the new expense to the database,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *HouseHandler) CreateHouseExpense(c *gin.Context) {
	lang := c.GetString("lang")
	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	utilTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	date, err := time.Parse(utilities.DateFormats.Input, c.Request.PostFormValue("date"))
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
	metadata := h.parseHouseMetadata(c, utilTypeID)

	newExpense := &models.HouseExpense{
		CreatedBy:     userID,
		Amount:        amount,
		UtilityTypeID: utilTypeID,
		ExpenseDate:   date,
		Notes:         notes,
		Metadata:      metadata,
	}

	err = h.DB.CreateHouseExpense(newExpense)
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

	highestExp, expType, err := h.DB.GetHighestHouseExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	montlyTotal, err := h.DB.GetTotalHouseExpenseForMonth(timeNow, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
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
			Title:   utilities.T(lang, "modal.create_success"),
			Message: fmt.Sprintf("%s: %v BGN", newExpense.UtilityType, newExpense.Amount),
			Lang:    lang,
		},
		Lang: lang,
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
	lang := c.GetString("lang")
	pageData, exists, err := h.getHousePageData(c)
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
		c.HTML(http.StatusOK, utilities.Templates.Components.HouseCurrent, pageData)
		return
	} else {
		rl := &models.RootLayout{
			TemplateName:    utilities.Templates.Pages.House,
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

// GetHome renders the main home dashboard page.
// It fetches the highest expense, total monthly expense, and a list of
// recent expenses for the current month and year from the database.
// It intelligently renders either the full page layout or a partial HTML
// snippet based on whether the request is an HTMX request.
func (h *HouseHandler) GetHome(c *gin.Context) {
	lang := c.GetString("lang")
	pageData, exists, err := h.getHousePageData(c)
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
		c.HTML(http.StatusOK, utilities.Templates.Pages.House, pageData)
		return
	} else {
		rl := &models.RootLayout{
			TemplateName:    utilities.Templates.Pages.House,
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

func (h *HouseHandler) GetHomeSection(c *gin.Context) {
	section := c.Query("section")

	c.HTML(http.StatusOK, fmt.Sprintf("house-%s", section), gin.H{
		"Lang": c.GetString("lang"),
	})
}

// GetExpenseById retrieves a single home expense by its unique ID
// and renders its details.
// It expects the expense ID to be provided as a path parameter.
func (h *HouseHandler) GetExpenseById(c *gin.Context) {
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

	exp, err := h.DB.GetHouseExpenseByID(id)

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

// GetEditHouseForm renders the HTML form pre-filled with existing expense data
// for editing a specific home expense.
// It expects the expense ID to be provided as a query parameter.
func (h *HouseHandler) GetEditHouseForm(c *gin.Context) {
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

	exp, err := h.DB.GetHouseExpenseByID(id)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	expTypes, err := h.DB.GetHouseUtilityTypes()
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	formData := &models.EditHouseFormData{
		Expense:  exp,
		Types:    expTypes,
		Metadata: exp.UnmarshalMetadata(),
		Lang:     lang,
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditHouseExpForm, formData)
}

// EditHouseExpenseById handles the HTTP PUT/POST request to update an existing home expense.
// It parses form data for updated expense type, date, amount, and notes,
// validates them, updates the expense in the database by its ID,
// and then returns updated summary data (highest and monthly total)
// to refresh the UI.
func (h *HouseHandler) EditHouseExpenseById(c *gin.Context) {
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

	utilTypeID, err := strconv.Atoi(c.Request.PostFormValue("typeID"))
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}
	date, err := time.Parse(utilities.DateFormats.Input, c.Request.PostFormValue("date"))
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
	if err != nil || amount <= 0 {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "400: Bad Request. Amount invalid, must be a positive number",
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}
	notes := c.Request.PostFormValue("notes")
	metadata := h.parseHouseMetadata(c, utilTypeID)

	editExpense := &models.HouseExpense{
		ID:            id,
		Amount:        amount,
		UtilityTypeID: utilTypeID,
		ExpenseDate:   date,
		Notes:         notes,
		Metadata:      metadata,
	}

	err = h.DB.EditHouseExpense(editExpense)
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

	highestExp, expType, err := h.DB.GetHighestHouseExpenseForMonth(timeNow.Month(), userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	montlyTotal, err := h.DB.GetTotalHouseExpenseForMonth(timeNow, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("400: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
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
			Title:   utilities.T(lang, "modal.update_success"),
			Message: fmt.Sprintf(utilities.T(lang, "modal.update_success_message"), editExpense.ID),
			Lang:    lang,
		},
		Lang: lang,
	}

	c.HTML(http.StatusCreated, utilities.Templates.Responses.CreateHouseExp, edExpResp)
}

// INFO: DELETE

func (h *HouseHandler) GetDeleteConfirm(c *gin.Context) {
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
		Endpoint: template.URL(fmt.Sprintf("/house/expenses/%v", id)),
		Target:   fmt.Sprintf("#exp-%v", id),
		Message:  fmt.Sprintf(utilities.T(lang, "modal.delete_message"), id),
		Lang:     lang,
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.ModalConfirm, content)
}

// DeleteHouseExp handles the HTTP DELETE request to remove a home expense by its ID.
// After successfully deleting the expense, it updates and returns
// the current month's total and highest expense summaries to reflect the change.
// It responds with 204 No Content if the expense was not found or not deleted,
// or 200 OK with updated summary data otherwise.
func (h *HouseHandler) DeleteHouseExp(c *gin.Context) {
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

	res, err := h.DB.DeleteHouseExpense(id)
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

	monthlyExpense, err := h.DB.GetTotalHouseExpenseForMonth(timeNow, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("500: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	highestExpense, utilType, err := h.DB.GetHighestHouseExpenseForMonth(month, userID)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("500: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
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
			Title:   utilities.T(lang, "modal.delete_success"),
			Message: fmt.Sprintf(utilities.T(lang, "modal.delete_success_message"), id),
			Lang:    lang,
		},
		Lang: lang,
	}

	c.HTML(http.StatusOK, utilities.Templates.Responses.DeleteHouseExp, pageData)
}
