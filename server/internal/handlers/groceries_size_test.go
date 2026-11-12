package handlers

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"expenser/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
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
	e := parseGroceriesExpenseForm(formContext(url.Values{"product": {"Milk"}, "unit": {"l"}, "unit_size": {"1"}}))
	assert.Equal(t, "l", e.Unit)
	assert.Equal(t, 1.0, e.UnitSize)

	e = parseGroceriesExpenseForm(formContext(url.Values{"product": {"Milk"}, "unit": {"box"}, "unit_size": {"1"}}))
	assert.Equal(t, "", e.Unit, "only known units are accepted")
	assert.Equal(t, 0.0, e.UnitSize)

	e = parseGroceriesExpenseForm(formContext(url.Values{"product": {"Milk"}}))
	assert.Equal(t, "", e.Unit)
}

func TestSizeGuess(t *testing.T) {
	unit, size := sizeGuess("Мляко 1л", "1", "", "")
	assert.Equal(t, "l", unit)
	assert.Equal(t, 1.0, size)

	unit, size = sizeGuess("Мляко 1л", "1", "pc", "6")
	assert.Equal(t, "pc", unit, "posted values are kept")
	assert.Equal(t, 6.0, size)

	unit, size = sizeGuess("Нещо", "1", "", "")
	assert.Equal(t, "", unit)
	assert.Equal(t, 0.0, size)
}
