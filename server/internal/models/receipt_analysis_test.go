package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kauflandItems() []ReceiptItem {
	items := make([]ReceiptItem, 50)
	for i := range items {
		items[i] = ReceiptItem{Name: "x", Quantity: 1, UnitPrice: 1, TotalPrice: 1}
	}
	set := func(pos int, name string, qty, total float64) {
		items[pos-1] = ReceiptItem{Name: name, Quantity: qty, UnitPrice: total / qty, TotalPrice: total}
	}
	set(5, "спагети 500г", 2, 3.06)
	set(11, "La Molisana", 1, 2.14)
	set(15, "Always 24бр", 1, 6.21)
	set(18, "Nutella", 1, 8.69)
	set(19, "Always дам.", 1, 6.21)
	set(36, "Захар", 2, 2.64)
	return items
}

func TestApplyDiscountsByPosition(t *testing.T) {
	a := ReceiptAnalysis{Items: kauflandItems(), Discounts: []Discount{
		{Description: "XTRA отстъпка", Amount: -3.73, Positions: []int{19, 15}},
		{Description: "XTRA отстъпка", Amount: 1.30, Positions: []int{5, 11}},
		{Description: "XTRA отстъпка", Amount: 2.40, Positions: []int{18}},
		{Description: "XTRA отстъпка", Amount: 1.14, Positions: []int{36}},
	}}
	left := a.ApplyDiscounts()
	assert.Empty(t, left)

	it := func(pos int) ReceiptItem { return a.Items[pos-1] }
	assert.InDelta(t, 2.40, it(18).TotalDiscount, 1e-9, "one item gets it all")
	assert.InDelta(t, 2.40, it(18).DiscountPerUnit, 1e-9)
	assert.InDelta(t, 1.14, it(36).TotalDiscount, 1e-9)
	assert.InDelta(t, 0.57, it(36).DiscountPerUnit, 1e-9, "per unit for 2 units")

	assert.InDelta(t, 0.77, it(5).TotalDiscount, 1e-9, "split by price: 3.06 of 5.20")
	assert.InDelta(t, 0.53, it(11).TotalDiscount, 1e-9)
	assert.InDelta(t, 3.73, it(15).TotalDiscount+it(19).TotalDiscount, 1e-9, "the parts add up to the cent")
	assert.InDelta(t, 1.86, it(15).TotalDiscount, 0.011)
	assert.InDelta(t, 1.86, it(19).TotalDiscount, 0.011)
	assert.Zero(t, it(1).TotalDiscount, "untouched items")
}

func TestApplyDiscountsLeavesDoubtfulOnesUnassigned(t *testing.T) {
	items := func() []ReceiptItem {
		return []ReceiptItem{{Name: "a", Quantity: 1, TotalPrice: 2}, {Name: "b", Quantity: 1, TotalPrice: 3, TotalDiscount: 0.5}}
	}
	for name, d := range map[string]Discount{
		"no positions":            {Amount: 1},
		"position past the end":   {Amount: 1, Positions: []int{3}},
		"position zero":           {Amount: 1, Positions: []int{0}},
		"repeated position":       {Amount: 1, Positions: []int{1, 1}},
		"already discounted item": {Amount: 0.5, Positions: []int{2}},
		"more than the items":     {Amount: 2.5, Positions: []int{1}},
	} {
		t.Run(name, func(t *testing.T) {
			a := ReceiptAnalysis{Items: items(), Discounts: []Discount{d}}
			left := a.ApplyDiscounts()
			require.Len(t, left, 1)
			assert.Zero(t, a.Items[0].TotalDiscount)
			assert.Equal(t, 0.5, a.Items[1].TotalDiscount, "an item's own discount is kept, not doubled")
		})
	}
}

func TestReceiptPurchaseTime(t *testing.T) {
	sofia, err := time.LoadLocation("Europe/Sofia")
	require.NoError(t, err)
	now := time.Date(2026, 11, 12, 18, 30, 0, 0, sofia)

	for name, tc := range map[string]struct {
		printed string
		want    time.Time
		found   bool
	}{
		"date and time":     {"2026-11-10T14:05", time.Date(2026, 11, 10, 14, 5, 0, 0, sofia), true},
		"with seconds":      {"2026-11-10T14:05:31", time.Date(2026, 11, 10, 14, 5, 31, 0, sofia), true},
		"space separated":   {"2026-11-10 14:05", time.Date(2026, 11, 10, 14, 5, 0, 0, sofia), true},
		"date only: noon":   {"2026-11-10", time.Date(2026, 11, 10, 12, 0, 0, 0, sofia), true},
		"later today":       {"2026-11-12T20:00", time.Date(2026, 11, 12, 20, 0, 0, 0, sofia), true},
		"not printed":       {"", now, false},
		"unreadable":        {"10.11.2026", now, false},
		"future: misread":   {"2026-12-10T14:05", now, false},
		"over a year: typo": {"2025-11-10T14:05", now, false},
	} {
		got, found := (&ReceiptAnalysis{PurchasedAt: tc.printed}).PurchaseTime(now)
		assert.Equal(t, tc.found, found, name)
		assert.True(t, tc.want.Equal(got), "%s: got %v", name, got)
	}
}

func TestReceiptSums(t *testing.T) {
	a := &ReceiptAnalysis{Items: []ReceiptItem{
		{TotalPrice: 7.92, TotalDiscount: 1.2},
		{TotalPrice: 2.9},
		{TotalPrice: 0.1, TotalDiscount: 0.1},
	}}
	subtotal, discount := a.Sums()
	assert.InDelta(t, 10.92, subtotal, 1e-9, "before discounts")
	assert.InDelta(t, 1.3, discount, 1e-9)
}
