package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"expenser/internal/groceryunits"
	"expenser/internal/models"
	"expenser/internal/services/gemini"
	"expenser/internal/utilities"

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
	ai    Suggester
}

// Suggester guesses sizes from product names.
type Suggester interface {
	SuggestSizes(ctx context.Context, products []string) ([]gemini.SizeSuggestion, error)
}

// suggestLimit is how many products one request asks about, so the answer
// fits and comes back in time.
const suggestLimit = 40

// suggestTimeout bounds one "suggest sizes" request, retries included.
const suggestTimeout = 2 * time.Minute // under nginx's 180s read timeout

func NewSizesHandler(store SizesStore, ai Suggester) *SizesHandler {
	return &SizesHandler{store: store, ai: ai}
}

var sizeErrors = []errorCase{
	badInput("error.invalid_size", models.ErrInvalidSize),
	{Status: http.StatusNotFound, Key: "error.expense_not_found", Errs: []error{sql.ErrNoRows}},
}

// sizeRows is the list with each row prefilled from what the name says.
func sizeRows(expenses []models.GroceriesExpense) []models.SizeRow {
	rows := make([]models.SizeRow, 0, len(expenses))
	for _, e := range expenses {
		row := models.SizeRow{Expense: e}
		if s, ok := groceryunits.Parse(e.Product, e.Quantity); ok {
			row.Unit, row.Size = string(s.Unit), s.Per
		}
		rows = append(rows, row)
	}
	return rows
}

func (h *SizesHandler) render(c *gin.Context, rows []models.SizeRow, suggestError, note string) {
	renderView(c, "grocery-sizes", "grocery-sizes-page", &models.SizesPage{
		Section:      string(models.SectionGroceries),
		Rows:         rows,
		Count:        len(rows),
		Lang:         currentLang(c),
		SuggestError: suggestError,
		SuggestNote:  note,
	})
}

// GetSizes renders the list: the fragment for HTMX requests, the full
// charts page wrapper otherwise.
func (h *SizesHandler) GetSizes(c *gin.Context) error {
	expenses, err := h.store.GetUnsizedGroceries(c.Request.Context(), currentUserID(c))
	if err != nil {
		return err
	}
	h.render(c, sizeRows(expenses), "", "")
	return nil
}

// Suggest asks the AI for the size of every listed product and shows the
// list with the guesses filled in. Nothing is saved: each row still needs
// its Save. A failed guess shows the list as it was, with the reason.
func (h *SizesHandler) Suggest(c *gin.Context) error {
	ctx, cancel := context.WithTimeout(c.Request.Context(), suggestTimeout)
	defer cancel()
	expenses, err := h.store.GetUnsizedGroceries(ctx, currentUserID(c))
	if err != nil {
		return err
	}
	rows := sizeRows(expenses)
	if len(rows) == 0 {
		h.render(c, rows, "", "")
		return nil
	}
	asked := min(len(rows), suggestLimit)
	names := make([]string, asked)
	for i, e := range expenses[:asked] {
		names[i] = e.Product
	}
	suggestions, err := h.ai.SuggestSizes(ctx, names)
	if err != nil {
		log.Printf("suggest sizes: %v", err)
		message := utilities.T(currentLang(c), "error.sizes_not_suggested")
		var httpErr *HTTPError
		if errors.As(sizesError(err), &httpErr) {
			message = httpErr.message(currentLang(c))
		}
		h.render(c, rows, message, "")
		return nil
	}
	for i, s := range suggestions {
		if i < len(rows) && s.Unit != "" {
			rows[i].Unit, rows[i].Size, rows[i].Suggested = s.Unit, s.UnitSize, true
		}
	}
	note := ""
	if len(rows) > asked {
		note = fmt.Sprintf(utilities.T(currentLang(c), "sizes.suggest_limited"), asked)
	}
	h.render(c, rows, "", note)
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
