package models

import (
	"math"
	"sort"
)

type ReceiptItem struct {
	Name            string  `json:"name"`
	Quantity        float64 `json:"quantity"`
	UnitPrice       float64 `json:"unit_price"`
	DiscountPerUnit float64 `json:"discount_per_unit"`
	TotalPrice      float64 `json:"total_price"`
	TotalDiscount   float64 `json:"total_discount"`
	Category        string  `json:"category"`
}

// Discount is a discount printed apart from its items (e.g. Kaufland's
// "XTRA отстъпка -3,73 / Позиции:19,15"). Positions are the 1-based numbers
// of the items it applies to, in printed order.
type Discount struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Positions   []int   `json:"positions"`
}

type Totals struct {
	TotalEUR     float64 `json:"total_eur"`
	TotalBGN     float64 `json:"total_bgn"`
	ExchangeRate float64 `json:"exchange_rate"`
}

type ReceiptAnalysis struct {
	Store     string        `json:"store"`
	Address   string        `json:"address"`
	VATNumber string        `json:"vat_number"`
	Items     []ReceiptItem `json:"items"`
	Discounts []Discount    `json:"discounts"`
	Totals    Totals        `json:"totals"`
}

// ApplyDiscounts puts each separately printed discount onto the items it
// names, so item totals add up to what was paid. One item gets the whole
// discount; several share it in proportion to their prices, in whole cents
// that add up exactly. A discount is left out (and returned) when its
// positions don't fit the items, when one of them already has a discount of
// its own (it was probably counted already), or when it is more than the
// items cost.
func (a *ReceiptAnalysis) ApplyDiscounts() (unassigned []Discount) {
	for _, d := range a.Discounts {
		if !a.applyDiscount(d) {
			unassigned = append(unassigned, d)
		}
	}
	return unassigned
}

func cents(v float64) int64 { return int64(math.Round(v * 100)) }

func (a *ReceiptAnalysis) applyDiscount(d Discount) bool {
	amount := cents(math.Abs(d.Amount))
	if amount == 0 || len(d.Positions) == 0 {
		return false
	}
	seen := map[int]bool{}
	weights := make([]int64, len(d.Positions))
	var total int64
	for i, pos := range d.Positions {
		if pos < 1 || pos > len(a.Items) || seen[pos] {
			return false
		}
		seen[pos] = true
		item := a.Items[pos-1]
		if item.TotalDiscount != 0 {
			return false
		}
		weights[i] = cents(item.TotalPrice)
		total += weights[i]
	}
	if total <= 0 || amount > total {
		return false
	}

	// Largest-remainder split: floor shares, then hand out the leftover
	// cents to the biggest remainders (first listed wins a tie).
	shares := make([]int64, len(weights))
	rems := make([]int64, len(weights))
	var given int64
	for i, w := range weights {
		shares[i] = amount * w / total
		rems[i] = amount * w % total
		given += shares[i]
	}
	order := make([]int, len(weights))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(x, y int) bool { return rems[order[x]] > rems[order[y]] })
	for i := int64(0); i < amount-given; i++ {
		shares[order[i]]++
	}

	for i, pos := range d.Positions {
		item := &a.Items[pos-1]
		item.TotalDiscount = float64(shares[i]) / 100
		if item.Quantity > 0 {
			item.DiscountPerUnit = math.Round(item.TotalDiscount/item.Quantity*10000) / 10000
		}
	}
	return true
}
