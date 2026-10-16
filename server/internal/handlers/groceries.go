package handlers

import (
	"bytes"
	"expenser/internal/models"
	"expenser/internal/services/gemini"
	"expenser/internal/utilities"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type GroceriesHandler struct {
	BaseHandler BaseHandler
	GeminiS     *gemini.GeminiService
}

func NewGroceriesHandler(rh *RootHandler, gs *gemini.GeminiService) *GroceriesHandler {
	return &GroceriesHandler{
		BaseHandler: rh.BaseHandler,
		GeminiS:     gs,
	}
}

// groceryRowData is the data the "grocery-row" template expects.
func groceryRowData(e *models.GroceriesExpense, lang string) gin.H {
	return gin.H{
		"ID":              e.ID,
		"CategoryName":    e.CategoryName,
		"PurchaseDate":    e.PurchaseDate,
		"Product":         e.Product,
		"Quantity":        e.Quantity,
		"UnitPrice":       e.UnitPrice,
		"DiscountPerUnit": e.DiscountPerUnit,
		"TotalPrice":      e.TotalPrice,
		"TotalDiscount":   e.TotalDiscount,
		"SupermarketName": e.SupermarketName,
		"Lang":            lang,
	}
}

// parseGroceriesExpenseForm reads the fields shared by the create and edit forms.
func parseGroceriesExpenseForm(c *gin.Context) *models.GroceriesExpense {
	categoryID, _ := strconv.Atoi(c.PostForm("category_id"))
	quantity, _ := strconv.ParseFloat(c.PostForm("quantity"), 64)
	unitPrice, _ := strconv.ParseFloat(c.PostForm("unit_price"), 64)
	discountPerUnit, _ := strconv.ParseFloat(c.PostForm("discount_per_unit"), 64)
	totalPrice, _ := strconv.ParseFloat(c.PostForm("total_price"), 64)
	totalDiscount, _ := strconv.ParseFloat(c.PostForm("total_discount"), 64)

	return &models.GroceriesExpense{
		CategoryID:      categoryID,
		Product:         c.PostForm("product"),
		Quantity:        quantity,
		UnitPrice:       unitPrice,
		DiscountPerUnit: discountPerUnit,
		TotalPrice:      totalPrice,
		TotalDiscount:   totalDiscount,
		SupermarketName: c.PostForm("supermarket_name"),
		PurchaseDate:    time.Now(),
	}
}

func (h *GroceriesHandler) UploadReceipt(c *gin.Context) error {
	lang := currentLang(c)
	file, err := c.FormFile("receipt")
	if err != nil {
		return badRequest("error.receipt_missing", err)
	}

	src, err := file.Open()
	if err != nil {
		return badRequest("error.receipt_unreadable", fmt.Errorf("failed to open file: %w", err))
	}
	defer src.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(src); err != nil {
		return badRequest("error.receipt_unreadable", fmt.Errorf("failed to read file: %w", err))
	}

	analysis, err := h.GeminiS.AnalyzeReceipt(c.Request.Context(), buf.Bytes())
	if err != nil {
		// Check for service unavailable or specific error indicating AI is down
		if err.Error() == "gemini api error: Service Unavailable" { // Adjust string based on actual API error output
			return &HTTPError{Status: http.StatusServiceUnavailable, MessageKey: "error.ai_unavailable", Err: err}
		}
		return &HTTPError{Status: http.StatusBadGateway, MessageKey: "error.receipt_not_analyzed", Err: err}
	}

	categories, err := h.BaseHandler.DB.GetAllCategories()
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, "groceries-receipt-edit-form", gin.H{
		"Store":      analysis.Store,
		"Items":      analysis.Items,
		"Categories": categories,
		"Lang":       lang,
	})
	return nil
}

func (h *GroceriesHandler) ConfirmBatchReceipt(c *gin.Context) error {
	lang := currentLang(c)
	userID := currentUserID(c)

	var expenses []models.GroceriesExpense
	for i := 0; ; i++ {
		product := c.PostForm(fmt.Sprintf("items[%d].product", i))
		if product == "" {
			break
		}
		categoryID, _ := strconv.Atoi(c.PostForm(fmt.Sprintf("items[%d].category_id", i)))
		if categoryID == 0 {
			categoryID = 7 // Default to 'Other' if not provided or empty
		}
		quantity, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].quantity", i)), 64)
		unitPrice, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].unit_price", i)), 64)
		discountPerUnit, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].discount_per_unit", i)), 64)
		totalPrice, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].total_price", i)), 64)
		totalDiscount, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].total_discount", i)), 64)
		supermarketName := c.PostForm(fmt.Sprintf("items[%d].supermarket_name", i))

		expense := &models.GroceriesExpense{
			UserID:          userID,
			CategoryID:      categoryID,
			Product:         product,
			Quantity:        quantity,
			UnitPrice:       unitPrice,
			DiscountPerUnit: discountPerUnit,
			TotalPrice:      totalPrice,
			TotalDiscount:   totalDiscount,
			SupermarketName: supermarketName,
			PurchaseDate:    time.Now(),
		}

		if err := h.BaseHandler.DB.CreateGroceriesExpense(expense); err != nil {
			return fmt.Errorf("failed to save item %d: %w", i, err)
		}

		// Fetch the saved expense to get the CategoryName populated
		dbExpense, err := h.BaseHandler.DB.GetGroceriesExpenseByID(expense.ID, currentUserID(c))
		if err == nil {
			expenses = append(expenses, *dbExpense)
		} else {
			expenses = append(expenses, *expense)
		}
	}

	c.HTML(http.StatusCreated, "confirm-batch-groceries", gin.H{
		"Expenses": expenses,
		"Modal": &models.ModalContent{
			Title:   utilities.T(lang, "modal.success_title"),
			Message: utilities.T(lang, "modal.create_success"),
			Lang:    lang,
		},
		"Lang": lang,
	})
	return nil
}

func (h *GroceriesHandler) GetGroceriesHome(c *gin.Context) error {
	expenses, err := h.BaseHandler.DB.GetRecentGroceriesExpenses(currentUserID(c))
	if err != nil {
		return err
	}

	renderPage(c, utilities.Templates.Pages.Groceries, gin.H{
		"Groceries": expenses,
		"Lang":      currentLang(c),
	})
	return nil
}

func (h *GroceriesHandler) GetGroceriesForm(c *gin.Context) error {
	c.HTML(http.StatusOK, utilities.Templates.Components.GroceriesReceiptForm, nil)
	return nil
}

func (h *GroceriesHandler) GetCreateGroceriesForm(c *gin.Context) error {
	categories, err := h.BaseHandler.DB.GetAllCategories()
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.CreateGroceriesForm, gin.H{
		"Lang":       currentLang(c),
		"Categories": categories,
	})
	return nil
}

func (h *GroceriesHandler) PostCreateGroceriesExpense(c *gin.Context) error {
	expense := parseGroceriesExpenseForm(c)
	expense.UserID = currentUserID(c)

	if err := h.BaseHandler.DB.CreateGroceriesExpense(expense); err != nil {
		return err
	}

	// Fetch the saved expense to get the CategoryName populated
	saved, err := h.BaseHandler.DB.GetGroceriesExpenseByID(expense.ID, currentUserID(c))
	if err != nil {
		return err
	}

	c.HTML(http.StatusCreated, "grocery-row", groceryRowData(saved, currentLang(c)))
	return nil
}

func (h *GroceriesHandler) GetEditGroceriesForm(c *gin.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	expense, err := h.BaseHandler.DB.GetGroceriesExpenseByID(id, currentUserID(c))
	if err != nil {
		return err
	}

	categories, err := h.BaseHandler.DB.GetAllCategories()
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditGroceriesForm, gin.H{
		"Expense":    expense,
		"Categories": categories,
		"Lang":       currentLang(c),
	})
	return nil
}

func (h *GroceriesHandler) EditGroceriesExpense(c *gin.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	expense := parseGroceriesExpenseForm(c)
	expense.ID = id

	if err := h.BaseHandler.DB.EditGroceriesExpense(expense, currentUserID(c)); err != nil {
		return err
	}

	// Fetch updated expense to render the row
	updated, err := h.BaseHandler.DB.GetGroceriesExpenseByID(id, currentUserID(c))
	if err != nil {
		return err
	}

	c.HTML(http.StatusOK, "grocery-row", groceryRowData(updated, currentLang(c)))
	return nil
}

func (h *GroceriesHandler) DeleteGroceriesExpense(c *gin.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}

	deleted, err := h.BaseHandler.DB.DeleteGroceriesExpense(id, currentUserID(c))
	if err != nil {
		return err
	}
	if !deleted {
		return notFound("error.expense_not_found")
	}

	c.Status(http.StatusOK)
	return nil
}

func (h *GroceriesHandler) GetCurrentMonth(c *gin.Context) error {
	recentExpenses, err := h.BaseHandler.DB.GetRecentGroceriesExpenses(currentUserID(c))
	if err != nil {
		return err
	}

	pageData := &models.GroceriesData{
		RecentExpenses: &recentExpenses,
		Lang:           currentLang(c),
	}
	renderView(c, utilities.Templates.Components.GroceriesCurrent, utilities.Templates.Pages.Groceries, pageData)
	return nil
}
