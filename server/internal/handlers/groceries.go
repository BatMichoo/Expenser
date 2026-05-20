package handlers

import (
	"bytes"
	database "expenser/internal/db"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GroceriesHandler struct {
	DB      *database.DB
	GeminiS *services.GeminiService
}

func NewGroceriesHandler(db *database.DB, gs *services.GeminiService) *GroceriesHandler {
	return &GroceriesHandler{
		DB:      db,
		GeminiS: gs,
	}
}

func (h *GroceriesHandler) UploadReceipt(c *gin.Context) {
	lang := c.GetString("lang")
	file, err := c.FormFile("receipt")
	if err != nil {
		c.String(http.StatusBadRequest, "Failed to get file: %s", err.Error())
		return
	}

	src, err := file.Open()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to open file: %s", err.Error())
		return
	}
	defer src.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(src); err != nil {
		c.String(http.StatusInternalServerError, "Failed to read file: %s", err.Error())
		return
	}

	analysis, err := h.GeminiS.AnalyzeReceipt(c.Request.Context(), buf.Bytes())
	if err != nil {
		message := fmt.Sprintf("Failed to analyze receipt: %s", err.Error())
		status := http.StatusInternalServerError

		// Check for service unavailable or specific error indicating AI is down
		if err.Error() == "gemini api error: Service Unavailable" { // Adjust string based on actual API error output
			message = utilities.T(lang, "error.ai_unavailable")
			status = http.StatusServiceUnavailable
		}

		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: message,
			Lang:    lang,
		}
		c.HTML(status, utilities.Templates.Components.ModalError, content)
		return
	}

	categories, err := h.DB.GetAllCategories()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load categories")
		return
	}

	c.HTML(http.StatusOK, "groceries-receipt-edit-form", gin.H{
		"Store":      analysis.Store,
		"Items":      analysis.Items,
		"Categories": categories,
		"Lang":       lang,
	})
}

func (h *GroceriesHandler) ConfirmBatchReceipt(c *gin.Context) {
	lang := c.GetString("lang")
	userIDstr, _ := c.Get("user_id")
	userID := userIDstr.(uuid.UUID)

	i := 0
	var expenses []models.GroceriesExpense
	for {
		categoryID, _ := strconv.Atoi(c.PostForm(fmt.Sprintf("items[%d].category_id", i)))
		product := c.PostForm(fmt.Sprintf("items[%d].product", i))
		if product == "" {
			break
		}
		if categoryID == 0 {
			categoryID = 7 // Default to 'Other' if not provided or empty
		}
		quantity, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].quantity", i)), 64)
		unitPrice, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].unit_price", i)), 64)
		totalPrice, _ := strconv.ParseFloat(c.PostForm(fmt.Sprintf("items[%d].total_price", i)), 64)
		supermarketName := c.PostForm(fmt.Sprintf("items[%d].supermarket_name", i))

		expense := &models.GroceriesExpense{
			UserID:          userID,
			CategoryID:      categoryID,
			Product:         product,
			Quantity:        quantity,
			UnitPrice:       unitPrice,
			TotalPrice:      totalPrice,
			SupermarketName: supermarketName,
			PurchaseDate:    time.Now(),
		}

		if err := h.DB.CreateGroceriesExpense(expense); err != nil {
			c.String(http.StatusInternalServerError, "Failed to save item")
			return
		}

		// Fetch the saved expense to get the CategoryName populated
		dbExpense, err := h.DB.GetGroceriesExpenseByID(expense.ID)
		if err == nil {
			expenses = append(expenses, *dbExpense)
		} else {
			expenses = append(expenses, *expense)
		}
		i++
	}

	content := &models.ModalContent{
		Title:   utilities.T(lang, "modal.success_title"),
		Message: utilities.T(lang, "modal.create_success"),
		Lang:    lang,
	}

	c.HTML(http.StatusCreated, "confirm-batch-groceries", gin.H{
		"Expenses": expenses,
		"Modal":    content,
		"Lang":     lang,
	})
}

func (h *GroceriesHandler) ConfirmReceipt(c *gin.Context) {
	lang := c.GetString("lang")
	categoryID, _ := strconv.Atoi(c.PostForm("category_id"))
	product := c.PostForm("product")
	quantity, _ := strconv.ParseFloat(c.PostForm("quantity"), 64)
	unitPrice, _ := strconv.ParseFloat(c.PostForm("unit_price"), 64)
	totalPrice, _ := strconv.ParseFloat(c.PostForm("total_price"), 64)
	supermarketName := c.PostForm("supermarket_name")

	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	expense := &models.GroceriesExpense{
		UserID:          userID,
		CategoryID:      categoryID,
		Product:         product,
		Quantity:        quantity,
		UnitPrice:       unitPrice,
		TotalPrice:      totalPrice,
		SupermarketName: supermarketName,
		PurchaseDate:    time.Now(),
	}

	if err := h.DB.CreateGroceriesExpense(expense); err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: fmt.Sprintf("Failed to save expense: %v", err),
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	content := &models.ModalContent{
		Title:   utilities.T(lang, "modal.success_title"),
		Message: utilities.T(lang, "modal.create_success"),
		Lang:    lang,
	}
	c.HTML(http.StatusCreated, utilities.Templates.Components.ModalSuccess, content)
}

func (h *GroceriesHandler) GetGroceriesHome(c *gin.Context) {
	lang := c.GetString("lang")
	userIDstr, exists := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	expenses, err := h.DB.GetRecentGroceriesExpenses(userID)
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
	pageData := gin.H{
		"Groceries": expenses,
		"Lang":      lang,
	}

	if isHtmxRequest {
		c.HTML(http.StatusOK, "groceries-page", pageData)
	} else {
		rl := &models.RootLayout{
			TemplateName:    "groceries-page",
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

func (h *GroceriesHandler) GetGroceriesForm(c *gin.Context) {
	c.HTML(http.StatusOK, utilities.Templates.Components.GroceriesReceiptForm, nil)
}

func (h *GroceriesHandler) GetCreateGroceriesForm(c *gin.Context) {
	lang := c.GetString("lang")
	categories, err := h.DB.GetAllCategories()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load categories")
		return
	}
	c.HTML(http.StatusOK, utilities.Templates.Components.CreateGroceriesForm, gin.H{
		"Lang":       lang,
		"Categories": categories,
	})
}

func (h *GroceriesHandler) PostCreateGroceriesExpense(c *gin.Context) {
	lang := c.GetString("lang")
	userIDstr, _ := c.Get("user_id")
	userID := userIDstr.(uuid.UUID)

	categoryID, _ := strconv.Atoi(c.PostForm("category_id"))
	product := c.PostForm("product")
	quantity, _ := strconv.ParseFloat(c.PostForm("quantity"), 64)
	unitPrice, _ := strconv.ParseFloat(c.PostForm("unit_price"), 64)
	totalPrice, _ := strconv.ParseFloat(c.PostForm("total_price"), 64)
	supermarketName := c.PostForm("supermarket_name")

	expense := &models.GroceriesExpense{
		UserID:          userID,
		CategoryID:      categoryID,
		Product:         product,
		Quantity:        quantity,
		UnitPrice:       unitPrice,
		TotalPrice:      totalPrice,
		SupermarketName: supermarketName,
		PurchaseDate:    time.Now(),
	}

	if err := h.DB.CreateGroceriesExpense(expense); err != nil {
		c.String(http.StatusInternalServerError, "Failed to save expense")
		return
	}

	expense, _ = h.DB.GetGroceriesExpenseByID(expense.ID)

	c.HTML(http.StatusCreated, "grocery-row", gin.H{
		"ID":              expense.ID,
		"CategoryName":    expense.CategoryName,
		"PurchaseDate":    expense.PurchaseDate,
		"Product":         expense.Product,
		"Quantity":        expense.Quantity,
		"UnitPrice":       expense.UnitPrice,
		"TotalPrice":      expense.TotalPrice,
		"SupermarketName": expense.SupermarketName,
		"Lang":            lang,
	})

}

func (h *GroceriesHandler) GetEditGroceriesForm(c *gin.Context) {
	lang := c.GetString("lang")
	id, _ := strconv.Atoi(c.Param("id"))

	expense, err := h.DB.GetGroceriesExpenseByID(id)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load expense")
		return
	}

	categories, err := h.DB.GetAllCategories()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load categories")
		return
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditGroceriesForm, gin.H{
		"Expense":    expense,
		"Categories": categories,
		"Lang":       lang,
	})
}

func (h *GroceriesHandler) EditGroceriesExpense(c *gin.Context) {
	lang := c.GetString("lang")
	id, _ := strconv.Atoi(c.Param("id"))

	categoryID, _ := strconv.Atoi(c.PostForm("category_id"))
	product := c.PostForm("product")
	quantity, _ := strconv.ParseFloat(c.PostForm("quantity"), 64)
	unitPrice, _ := strconv.ParseFloat(c.PostForm("unit_price"), 64)
	totalPrice, _ := strconv.ParseFloat(c.PostForm("total_price"), 64)
	supermarketName := c.PostForm("supermarket_name")

	expense := &models.GroceriesExpense{
		ID:              id,
		CategoryID:      categoryID,
		Product:         product,
		Quantity:        quantity,
		UnitPrice:       unitPrice,
		TotalPrice:      totalPrice,
		SupermarketName: supermarketName,
		PurchaseDate:    time.Now(),
	}

	if err := h.DB.EditGroceriesExpense(expense); err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "Failed to update expense",
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	// Fetch updated expense to render the row
	updatedExpense, err := h.DB.GetGroceriesExpenseByID(id)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch updated expense")
		return
	}

	c.HTML(http.StatusOK, "grocery-row", gin.H{
		"ID":              updatedExpense.ID,
		"CategoryName":    updatedExpense.CategoryName,
		"PurchaseDate":    updatedExpense.PurchaseDate,
		"Product":         updatedExpense.Product,
		"Quantity":        updatedExpense.Quantity,
		"UnitPrice":       updatedExpense.UnitPrice,
		"TotalPrice":      updatedExpense.TotalPrice,
		"SupermarketName": updatedExpense.SupermarketName,
		"Lang":            lang,
	})
}

func (h *GroceriesHandler) DeleteGroceriesExpense(c *gin.Context) {
	lang := c.GetString("lang")
	id, _ := strconv.Atoi(c.Param("id"))

	if _, err := h.DB.DeleteGroceriesExpense(id); err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "Failed to delete expense",
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	c.Status(http.StatusOK)
}
