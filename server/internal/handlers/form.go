package handlers

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"expenser/internal/utilities"

	"github.com/gin-gonic/gin"
)

// expenseFields are the fields every house and car expense form has
// (create, edit and backfill).
type expenseFields struct {
	TypeID   int
	Date     time.Time
	Amount   float64
	Discount float64 // optional, 0 when empty
	Notes    string
}

// parseExpenseFields reads and checks the shared expense fields, with a
// message naming what to fix.
func parseExpenseFields(v url.Values) (expenseFields, error) {
	var f expenseFields
	var err error
	if f.TypeID, err = strconv.Atoi(v.Get("typeID")); err != nil {
		return f, badRequest("error.invalid_type", err)
	}
	if f.Date, err = time.Parse(utilities.DateFormats.Input, v.Get("date")); err != nil {
		return f, badRequest("error.invalid_date", err)
	}
	if f.Amount, err = formMoney(v, "amount", true); err != nil {
		return f, err
	}
	if f.Discount, err = formMoney(v, "discount_amount", false); err != nil {
		return f, err
	}
	f.Notes = strings.TrimSpace(v.Get("notes"))
	return f, nil
}

// formMoney reads a non-negative amount; an empty optional field is 0.
func formMoney(v url.Values, key string, required bool) (float64, error) {
	return parseMoney(v.Get(key), key, required)
}

// parseMoney is formMoney for a value already read; key names the field in
// the log.
func parseMoney(s, key string, required bool) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" && !required {
		return 0, nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err == nil && (math.IsNaN(n) || math.IsInf(n, 0)) {
		err = fmt.Errorf("%q is not a finite number", s) // ParseFloat accepts "NaN" and "Inf"
	}
	if err != nil {
		return 0, badRequest("error.invalid_amount", fmt.Errorf("%s: %w", key, err))
	}
	if n < 0 {
		return 0, badRequest("error.amount_negative", fmt.Errorf("%s %v", key, n))
	}
	return n, nil
}

// paramID reads the numeric :id route parameter.
func paramID(c *gin.Context) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return 0, badRequest("error.invalid_id", err)
	}
	return id, nil
}
