package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSizesStore struct {
	owner   uuid.UUID
	rows    []models.GroceriesExpense
	setID   int
	setUnit string
	setSize float64
	skipped int
}

func (f *fakeSizesStore) GetUnsizedGroceries(context.Context, uuid.UUID) ([]models.GroceriesExpense, error) {
	return f.rows, nil
}

func (f *fakeSizesStore) CountUnsizedGroceries(context.Context, uuid.UUID) (int, error) {
	return len(f.rows), nil
}

func (f *fakeSizesStore) SetGrocerySize(_ context.Context, userID uuid.UUID, id int, unit string, size float64) error {
	if userID != f.owner {
		return fmt.Errorf("set: %w", sql.ErrNoRows)
	}
	f.setID, f.setUnit, f.setSize = id, unit, size
	return nil
}

func (f *fakeSizesStore) SkipGrocerySize(_ context.Context, userID uuid.UUID, id int) error {
	if userID != f.owner {
		return fmt.Errorf("skip: %w", sql.ErrNoRows)
	}
	f.skipped = id
	return nil
}

// sizesCall runs a sizes route as user, returning the recorder.
func sizesCall(t *testing.T, store *fakeSizesStore, user uuid.UUID, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	require.NoError(t, utilities.InitI18n())
	tmpl, err := utilities.ParseTemplates("../templates/*/*.html")
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.SetHTMLTemplate(tmpl)
	r.Use(func(c *gin.Context) {
		c.Set("lang", "en")
		services.SetClaims(c, &services.JWTClaims{UserID: user})
	})
	h := NewSizesHandler(store)
	r.GET("/groceries/sizes", wrap(h.GetSizes))
	r.POST("/groceries/sizes/:id", wrap(h.SaveSize))
	r.POST("/groceries/sizes/:id/skip", wrap(h.SkipSize))

	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSaveSizeInvalid(t *testing.T) {
	user := uuid.New()
	for name, form := range map[string]url.Values{
		"bad unit":  {"unit": {"box"}, "unit_size": {"1"}},
		"zero size": {"unit": {"l"}, "unit_size": {"0"}},
		"no size":   {"unit": {"l"}},
		"not a num": {"unit": {"l"}, "unit_size": {"abc"}},
	} {
		t.Run(name, func(t *testing.T) {
			store := &fakeSizesStore{owner: user}
			w := sizesCall(t, store, user, "POST", "/groceries/sizes/5", form)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Zero(t, store.setID)
		})
	}
}

func TestSaveSizeNotOwned(t *testing.T) {
	store := &fakeSizesStore{owner: uuid.New()}
	w := sizesCall(t, store, uuid.New(), "POST", "/groceries/sizes/5", url.Values{"unit": {"l"}, "unit_size": {"1"}})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSaveSizeOK(t *testing.T) {
	user := uuid.New()
	store := &fakeSizesStore{owner: user, rows: make([]models.GroceriesExpense, 2)}
	w := sizesCall(t, store, user, "POST", "/groceries/sizes/5", url.Values{"unit": {"l"}, "unit_size": {"1"}})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 5, store.setID)
	assert.Equal(t, "l", store.setUnit)
	assert.Equal(t, 1.0, store.setSize)
	assert.Equal(t, "sizes-changed", w.Header().Get("HX-Trigger"))
	assert.Contains(t, w.Body.String(), `id="unsized-count"`)
	assert.Contains(t, w.Body.String(), `hx-swap-oob="true"`)
	assert.Contains(t, w.Body.String(), "2")
}

func TestSkipSize(t *testing.T) {
	user := uuid.New()
	store := &fakeSizesStore{owner: user}
	w := sizesCall(t, store, user, "POST", "/groceries/sizes/7/skip", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 7, store.skipped)
	assert.Equal(t, "sizes-changed", w.Header().Get("HX-Trigger"))
	assert.Contains(t, w.Body.String(), `id="unsized-count"`)

	w = sizesCall(t, store, uuid.New(), "POST", "/groceries/sizes/7/skip", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetSizes(t *testing.T) {
	user := uuid.New()
	store := &fakeSizesStore{owner: user, rows: []models.GroceriesExpense{{ID: 3, Product: "Мляко", Quantity: 1, CategoryName: "Dairy"}}}
	w := sizesCall(t, store, user, "GET", "/groceries/sizes", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Мляко")
	assert.Contains(t, w.Body.String(), `hx-post="/groceries/sizes/3"`)
}
