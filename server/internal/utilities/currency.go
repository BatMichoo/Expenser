package utilities

var Currency = "EUR"

// CurrencySymbol is the short symbol shown next to amounts.
func CurrencySymbol() string {
	if Currency == "EUR" {
		return "€"
	}
	return Currency
}
