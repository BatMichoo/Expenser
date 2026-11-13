package utilities

import (
	"fmt"
	"math"
)

var Currency = "EUR"

// CurrencySymbol is the short symbol shown next to amounts.
func CurrencySymbol() string {
	if Currency == "EUR" {
		return "€"
	}
	return Currency
}

// MoneyValue is an amount with exactly two decimals and no symbol, as a
// form input's value ("12.50", never "12.5" or "2.4000000000000004").
// Amounts are float64 sums of cent values, so they can be a hair off (a
// zero total as -0.0000000001): round to the cent first, half away from
// zero, so that never shows as "-0.00".
func MoneyValue(v float64) string {
	cents := math.Round(v * 100)
	if cents == 0 {
		cents = 0 // drops the sign of -0
	}
	return fmt.Sprintf("%.2f", cents/100)
}

// Money formats an amount with two decimals and the currency symbol.
func Money(v float64) string {
	return MoneyValue(v) + " " + CurrencySymbol()
}
