package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"expenser/internal/groceryunits"
	"expenser/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SizesStore is the data the "needs a size" list uses.
type SizesStore interface {
	GetUnsizedGroceries(ctx context.Context, userID uuid.UUID) ([]models.GroceriesExpense, error)
	CountUnsizedGroceries(ctx context.Context, userID uuid.UUID) (int, error)
	SetGrocerySize(ctx context.Context, userID uuid.UUID, id int, unit string, size float64) error
	SkipGrocerySize(ctx context.Context, userID uuid.UUID, id int) error
}

// SizesHandler serves the list of grocery purchases that still need a size.
type SizesHandler struct {
	store SizesStore
}

func NewSizesHandler(store SizesStore) *SizesHandler {
	return &SizesHandler{store: store}
}

var sizeErrors = []errorCase{
	badInput("error.invalid_size", models.ErrInvalidSize),
	{Status: http.StatusNotFound, Key: "error.expense_not_found", Errs: []error{sql.ErrNoRows}},
}

// GetSizes renders the list: the fragment for HTMX requests, the full
// charts page wrapper otherwise.
func (h *SizesHandler) GetSizes(c *gin.Context) error {
	expenses, err := h.store.GetUnsizedGroceries(c.Request.Context(), currentUserID(c))
	if err != nil {
		return err
	}
	rows := make([]models.SizeRow, 0, len(expenses))
	for _, e := range expenses {
		row := models.SizeRow{Expense: e}
		if s, ok := groceryunits.Parse(e.Product, e.Quantity); ok {
			row.Unit, row.Size = string(s.Unit), s.Per
		}
		rows = append(rows, row)
	}
	renderView(c, "grocery-sizes", "grocery-sizes-page", &models.SizesPage{
		Section: string(models.SectionGroceries),
		Rows:    rows,
		Count:   len(rows),
		Lang:    currentLang(c),
	})
	return nil
}

// SaveSize stores the unit and size typed for one purchase.
func (h *SizesHandler) SaveSize(c *gin.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}
	unit := c.PostForm("unit")
	size, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(c.PostForm("unit_size")), ",", ".", 1), 64)
	if err != nil || size <= 0 || !groceryunits.Unit(unit).Valid() {
		return badRequest("error.invalid_size", err)
	}
	if err := h.store.SetGrocerySize(c.Request.Context(), currentUserID(c), id, unit, size); err != nil {
		return mapError(err, sizeErrors)
	}
	return h.rowDone(c)
}

// SkipSize marks one purchase as not worth sizing.
func (h *SizesHandler) SkipSize(c *gin.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return badRequest("error.invalid_id", err)
	}
	if err := h.store.SkipGrocerySize(c.Request.Context(), currentUserID(c), id); err != nil {
		return mapError(err, sizeErrors)
	}
	return h.rowDone(c)
}

// rowDone answers a save or skip: the row's swap gets no content (so it is
// removed) and the count is updated out of band.
func (h *SizesHandler) rowDone(c *gin.Context) error {
	n, err := h.store.CountUnsizedGroceries(c.Request.Context(), currentUserID(c))
	if err != nil {
		return err
	}
	c.Header("HX-Trigger", "sizes-changed")
	c.HTML(http.StatusOK, "unsized-count-oob", gin.H{"Count": n, "Lang": currentLang(c)})
	return nil
}
