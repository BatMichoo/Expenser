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
		c.String(http.StatusInternalServerError, "Failed to analyze receipt: %s", err.Error())
		return
	}

	c.HTML(http.StatusOK, utilities.Templates.Responses.GroceriesReceiptResult, gin.H{
		"Product":         analysis.Product,
		"Quantity":        analysis.Quantity,
		"Price":           analysis.Price,
		"SupermarketName": analysis.SupermarketName,
		"Lang":            lang,
	})
}

func (h *GroceriesHandler) ConfirmReceipt(c *gin.Context) {
	lang := c.GetString("lang")
	product := c.PostForm("product")
	quantity, _ := strconv.ParseFloat(c.PostForm("quantity"), 64)
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	supermarketName := c.PostForm("supermarket_name")

	userIDstr, _ := c.Get("user_id")
	userID, _ := userIDstr.(uuid.UUID)

	expense := &models.GroceriesExpense{
		UserID:          userID,
		Product:         product,
		Quantity:        quantity,
		Price:           int64(price * 100),
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

func (h *GroceriesHandler) GetEditGroceriesForm(c *gin.Context) {
	lang := c.GetString("lang")
	id, _ := strconv.Atoi(c.Param("id"))

	expense, err := h.DB.GetGroceriesExpenseByID(id)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load expense")
		return
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.EditGroceriesForm, gin.H{
		"Expense": expense,
		"Lang":    lang,
	})
}

func (h *GroceriesHandler) EditGroceriesExpense(c *gin.Context) {
	lang := c.GetString("lang")
	id, _ := strconv.Atoi(c.Param("id"))

	product := c.PostForm("product")
	quantity, _ := strconv.ParseFloat(c.PostForm("quantity"), 64)
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	supermarketName := c.PostForm("supermarket_name")

	expense := &models.GroceriesExpense{
		ID:              id,
		Product:         product,
		Quantity:        quantity,
		Price:           int64(price * 100),
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

	c.Status(http.StatusOK)
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
