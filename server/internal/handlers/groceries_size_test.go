package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"expenser/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReceiptItemSize(t *testing.T) {
	cases := map[string]struct {
		item models.ReceiptItem
		unit string
		size float64
	}{
		"name only":    {models.ReceiptItem{Name: "Мляко 1л", Quantity: 1}, "l", 1},
		"ai only":      {models.ReceiptItem{Name: "Сирене", Quantity: 1, Unit: "kg", UnitSize: 0.25}, "kg", 0.25},
		"disagreement": {models.ReceiptItem{Name: "Мляко 1л", Quantity: 1, Unit: "kg", UnitSize: 5}, "", 0},
		"nothing":      {models.ReceiptItem{Name: "Нещо", Quantity: 1}, "", 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			unit, size := receiptItemSize(tc.item)
			assert.Equal(t, tc.unit, unit)
			assert.InDelta(t, tc.size, size, 1e-9)
		})
	}
}

func formContext(values url.Values) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c
}

func TestParseGroceriesFormUnits(t *testing.T) {
	e, err := parseGroceriesExpenseForm(formContext(url.Values{"product": {"Milk"}, "unit": {"l"}, "unit_size": {"1"}}))
	require.NoError(t, err)
	assert.Equal(t, "l", e.Unit)
	assert.Equal(t, 1.0, e.UnitSize)

	e, err = parseGroceriesExpenseForm(formContext(url.Values{"product": {"Milk"}, "unit": {"box"}, "unit_size": {"1"}}))
	require.NoError(t, err)
	assert.Equal(t, "", e.Unit, "only known units are accepted")
	assert.Equal(t, 0.0, e.UnitSize)

	e, err = parseGroceriesExpenseForm(formContext(url.Values{"product": {"Milk"}}))
	require.NoError(t, err)
	assert.Equal(t, "", e.Unit)
}

func TestParseGroceriesFormRejectsBadNumbers(t *testing.T) {
	for _, v := range []url.Values{
		{"product": {"Milk"}, "total_price": {"abc"}},
		{"product": {"Milk"}, "total_price": {"-1"}},
		{"product": {"Milk"}, "quantity": {"NaN"}},
		{"product": {"Milk"}, "unit_price": {"Inf"}},
		{"product": {"  "}, "total_price": {"2"}},
	} {
		_, err := parseGroceriesExpenseForm(formContext(v))
		var httpErr *HTTPError
		require.ErrorAs(t, err, &httpErr, "%v", v)
		assert.Equal(t, http.StatusBadRequest, httpErr.Status, "%v", v)
	}
}

func TestParseReceiptItems(t *testing.T) {
	at := time.Date(2026, 11, 1, 18, 30, 0, 0, time.UTC)
	user := uuid.New()
	items, err := parseReceiptItems(url.Values{
		"items[0].product":     {"Milk"},
		"items[0].total_price": {"2.40"},
		"items[0].category_id": {"10"},
		"items[1].product":     {"Bread"},
		"items[1].total_price": {"1.10"},
		"items[3].product":     {"after a gap: not read"},
	}, user, at)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, 10, items[0].CategoryID)
	assert.Equal(t, models.GroceryCategoryOther, items[1].CategoryID, "no category: Other")
	assert.Equal(t, 1.10, items[1].TotalPrice)
	assert.Equal(t, user, items[1].UserID)
	assert.Equal(t, at, items[1].PurchaseDate)

	_, err = parseReceiptItems(url.Values{"items[0].product": {"Milk"}, "items[0].total_price": {"x"}}, user, at)
	assert.Error(t, err, "one bad item refuses the receipt")
}

func TestSizeGuess(t *testing.T) {
	unit, size, guessed := sizeGuess("Мляко 1л", "1", "", "", "")
	assert.Equal(t, "l", unit)
	assert.Equal(t, 1.0, size)
	assert.True(t, guessed)

	unit, size, guessed = sizeGuess("Мляко 500мл", "1", "l", "1", "1")
	assert.Equal(t, "l", unit)
	assert.Equal(t, 0.5, size, "an earlier guess is replaced by the new one")
	assert.True(t, guessed)

	unit, size, guessed = sizeGuess("Нещо", "1", "l", "1", "1")
	assert.Equal(t, "", unit, "a stale guess is cleared")
	assert.Equal(t, 0.0, size)
	assert.False(t, guessed)

	unit, size, guessed = sizeGuess("Мляко 1л", "1", "pc", "6", "")
	assert.Equal(t, "pc", unit, "a size the user chose is kept")
	assert.Equal(t, 6.0, size)
	assert.False(t, guessed)

	unit, size, _ = sizeGuess("Нещо", "1", "", "", "")
	assert.Equal(t, "", unit)
	assert.Equal(t, 0.0, size)
}

func TestParseFormIgnoresSizeGuessed(t *testing.T) {
	e, err := parseGroceriesExpenseForm(formContext(url.Values{"product": {"Milk"}, "unit": {"l"}, "unit_size": {"1"}, "size_guessed": {"1"}}))
	require.NoError(t, err)
	assert.Equal(t, "l", e.Unit)
	assert.Equal(t, 1.0, e.UnitSize)
}
