package groceryunits

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	cases := []struct {
		product string
		qty     float64
		unit    Unit
		per     float64
		ok      bool
	}{
		{"Fresh milk 1L", 2, Liter, 1, true},
		{"Прясно мляко 3% 1л", 1, Liter, 1, true},
		{"Кисело мляко 3,6% 400г", 1, Kg, 0.4, true},
		{"Yogurt 400g", 2, Kg, 0.4, true},
		{"Potatoes 2kg", 1, Kg, 2, true},
		{"Сирене 0,400 кг", 1, Kg, 0.4, true},
		{"Beer 0.5L", 4, Liter, 0.5, true},
		{"Вода 6x0,5л", 1, Liter, 3, true},
		{"Кока-Кола 2 х 1,5 л", 1, Liter, 3, true},
		{"Олио 500 мл", 1, Liter, 0.5, true},
		{"Яйца М 10бр", 1, Piece, 10, true},
		{"Eggs 10 pcs", 1, Piece, 10, true},
		{"Tomatoes", 1.2, Kg, 1, true},
		{"Банани", 0.855, Kg, 1, true},
		{"White bread", 1, "", 0, false},
		{"Бира Загорка кен", 4, "", 0, false},
		{"Шоколад 100г 2бр", 1, "", 0, false},
		{"Мляко 1л 2л", 1, "", 0, false},
		{"Йогурт 150г x6", 1, "", 0, false},
		{"Мляко 1л x6", 1, "", 0, false},
		{"Вода 1.5л*6", 1, "", 0, false},
		{"Бира 330мл х 6", 1, "", 0, false},
		{"Бира 330мл x 6бр", 1, "", 0, false},
		{"Сирене 1/2 кг", 1, "", 0, false},
		{"Мляко 1/2л", 1, "", 0, false},
		{"Ябълки 0,5-1 кг", 1, "", 0, false},
		{"Шунка 250-300г", 1, "", 0, false},
		{"Фъст.сарн.б.ф.200г", 1, Kg, 0.2, true},
		{"KLC.кекс.600г", 1, Kg, 0.6, true},
		{"K.Домати400гр", 1, Kg, 0.4, true},
		{"KLC.дом.по.бос.690г", 1, Kg, 0.69, true},
		{"Pomsticks к.пръч.85г", 1, Kg, 0.085, true},
		// A size glued to a letter is accepted: "Vit2g" reads as 2 g.
		{"Vit2g", 1, Kg, 0.002, true},
		// A leading dot is not a boundary, and a decimal dot stays in the number.
		{"Мляко .5л", 1, "", 0, false},
		{"Мляко 1.5л", 1, Liter, 1.5, true},
		// Repeated sizes are ambiguous even when equal (two packs? one?).
		{"Кафе 500г+500г", 1, "", 0, false},
		{"Мляко 1л 1л", 1, "", 0, false},
		// A weighed (fractional) quantity with a pack size other than exactly
		// 1 kg or 1 l cannot be priced per unit.
		{"Кашкавал 400г", 0.855, "", 0, false},
		{"Мляко 2л", 1.5, "", 0, false},
		{"Банани 1кг", 0.855, Kg, 1, true},
		{"Мляко 1л", 1.5, Liter, 1, true},
		// A four-digit year before "г" is not a size.
		{"Вино 2019г.", 1, "", 0, false},
		{"Вино 2019г", 1, "", 0, false},
		{"Вино 2019г 0,75л", 1, Liter, 0.75, true},
		{"Брашно 1900гр", 1, Kg, 1.9, true},
		{"Ориз 2500г", 1, Kg, 2.5, true},
	}
	for _, c := range cases {
		t.Run(c.product, func(t *testing.T) {
			got, ok := Parse(c.product, c.qty)
			assert.Equal(t, c.ok, ok)
			if c.ok {
				assert.Equal(t, c.unit, got.Unit)
				assert.InDelta(t, c.per, got.Per, 1e-9)
			}
		})
	}
}

// Sizes are rounded to 4 decimals, like the NUMERIC(10,4) column, so a
// multipack never carries float noise into forms or the database.
func TestParseRoundsToFourDecimals(t *testing.T) {
	got, ok := Parse("Сок 3x0,2л", 1)
	assert.True(t, ok)
	assert.Equal(t, Liter, got.Unit)
	assert.Equal(t, 0.6, got.Per)

	got, ok = Parse("Дъвки 25x1,75г", 1)
	assert.True(t, ok)
	assert.Equal(t, Kg, got.Unit)
	assert.Equal(t, 0.0438, got.Per)

	got, ok = Resolve("White bread", 1, "kg", 0.33333)
	assert.True(t, ok)
	assert.Equal(t, 0.3333, got.Per)
}

func TestParseZeroQuantity(t *testing.T) {
	_, ok := Parse("Tomatoes", 0)
	assert.False(t, ok)

	got, ok := Parse("Fresh milk 1L", 0)
	assert.True(t, ok)
	assert.Equal(t, Liter, got.Unit)
	assert.InDelta(t, 1, got.Per, 1e-9)
}

func TestResolve(t *testing.T) {
	got, ok := Resolve("Fresh milk 1L", 1, "", 0)
	assert.True(t, ok)
	assert.Equal(t, Size{Liter, 1}, got)

	got, ok = Resolve("Fresh milk 1L", 1, "l", 1.005)
	assert.True(t, ok)
	assert.Equal(t, Size{Liter, 1}, got)

	_, ok = Resolve("Fresh milk 1L", 1, "l", 2)
	assert.False(t, ok)
	_, ok = Resolve("Fresh milk 1L", 1, "kg", 1)
	assert.False(t, ok)

	got, ok = Resolve("White bread", 1, "kg", 0.25)
	assert.True(t, ok)
	assert.Equal(t, Size{Kg, 0.25}, got)

	_, ok = Resolve("White bread", 1, "box", 1)
	assert.False(t, ok)
	_, ok = Resolve("White bread", 1, "l", 0)
	assert.False(t, ok)
}

func TestUnitValid(t *testing.T) {
	assert.True(t, Kg.Valid())
	assert.True(t, Liter.Valid())
	assert.True(t, Piece.Valid())
	assert.False(t, Unit("box").Valid())
}
