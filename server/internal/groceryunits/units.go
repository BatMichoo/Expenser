// Package groceryunits reads pack sizes (kg / l / pc) from product names.
package groceryunits

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Unit is a base unit for grocery quantities.
type Unit string

const (
	Kg    Unit = "kg"
	Liter Unit = "l"
	Piece Unit = "pc"
)

// Valid reports whether u is one of the known base units.
func (u Unit) Valid() bool {
	return u == Kg || u == Liter || u == Piece
}

// Size is the amount of base units in one counted quantity.
type Size struct {
	Unit Unit
	Per  float64
}

// Go's \b is ASCII-only, so token boundaries are spelled out explicitly.
const (
	numPat   = `(\d+(?:\.\d+)?)`
	unitPat  = `(кг|kg|мл|ml|бр|pcs|pc|гр|г|g|л|l)`
	leftPat  = `(?:^|[^\p{L}\d.])`
	rightPat = `\.?(?:$|[^\p{L}])`
)

var (
	multiRe  = regexp.MustCompile(leftPat + numPat + `\s*[xх*]\s*` + numPat + `\s*` + unitPat + rightPat)
	singleRe = regexp.MustCompile(leftPat + numPat + `\s*` + unitPat + rightPat)
	commaRe  = regexp.MustCompile(`(\d),(\d)`)
	// A number glued to another by / - or en dash before a size is a fraction
	// or range ("1/2 кг", "250-300г"), not a size.
	rangeRe = regexp.MustCompile(`\d\s*[/\-–]\s*` + numPat + `\s*` + unitPat + rightPat)
	// A multiplier after a size ("150г x6") makes the pack size ambiguous.
	trailMulRe = regexp.MustCompile(unitPat + `\.?\s*[xх*]\s*\d`)
)

// normalise converts a parsed amount and unit token to a base unit.
func normalise(amount float64, token string) Size {
	switch token {
	case "кг", "kg":
		return Size{Kg, amount}
	case "г", "гр", "g":
		return Size{Kg, amount / 1000}
	case "мл", "ml":
		return Size{Liter, amount / 1000}
	case "л", "l":
		return Size{Liter, amount}
	default:
		return Size{Piece, amount}
	}
}

// Parse reads the pack size from a product name. It returns false unless the
// size is certain. A name without a size but with a fractional quantity is
// treated as sold by weight (one kg per counted unit).
func Parse(product string, quantity float64) (Size, bool) {
	s := commaRe.ReplaceAllString(strings.ToLower(product), "$1.$2")

	if rangeRe.MatchString(s) {
		return Size{}, false
	}

	var found []Size

	// Multipacks first, removed so their parts are not read again as singles.
	for {
		m := multiRe.FindStringSubmatchIndex(s)
		if m == nil {
			break
		}
		n, err1 := strconv.ParseFloat(s[m[2]:m[3]], 64)
		v, err2 := strconv.ParseFloat(s[m[4]:m[5]], 64)
		if err1 != nil || err2 != nil {
			return Size{}, false
		}
		sz := normalise(v, s[m[6]:m[7]])
		sz.Per *= n
		found = append(found, sz)
		s = s[:m[0]] + " " + s[m[1]:]
	}

	if trailMulRe.MatchString(s) {
		return Size{}, false
	}

	// Singles. Resume at the end of each unit token so a shared delimiter can
	// start the next match.
	for pos := 0; pos < len(s); {
		m := singleRe.FindStringSubmatchIndex(s[pos:])
		if m == nil {
			break
		}
		v, err := strconv.ParseFloat(s[pos+m[2]:pos+m[3]], 64)
		if err != nil {
			return Size{}, false
		}
		found = append(found, normalise(v, s[pos+m[4]:pos+m[5]]))
		pos += m[5]
	}

	if len(found) == 0 {
		if quantity > 0 && math.Abs(quantity-math.Round(quantity)) > 1e-9 {
			return Size{Kg, 1}, true
		}
		return Size{}, false
	}
	first := found[0]
	for _, f := range found[1:] {
		if f.Unit != first.Unit || math.Abs(f.Per-first.Per) > 1e-9 {
			return Size{}, false
		}
	}
	if first.Per <= 0 {
		return Size{}, false
	}
	return first, true
}

// Resolve reconciles the rule-based parse with an AI-suggested size. A rule
// result wins when the AI is silent or agrees (same unit, Per within 1%) and
// is rejected when the AI disagrees. Without a rule result, a valid AI size
// is accepted.
func Resolve(product string, quantity float64, aiUnit string, aiPer float64) (Size, bool) {
	rule, ok := Parse(product, quantity)
	ai := Unit(strings.ToLower(strings.TrimSpace(aiUnit)))

	if ok {
		if aiUnit == "" && aiPer == 0 {
			return rule, true
		}
		if ai == rule.Unit && math.Abs(aiPer-rule.Per) <= 0.01*rule.Per {
			return rule, true
		}
		return Size{}, false
	}
	if ai.Valid() && aiPer > 0 {
		return Size{ai, aiPer}, true
	}
	return Size{}, false
}

// Bounds of a pack size that fits the unit_size NUMERIC(10,4) column and its
// CHECK (unit_size > 0) without rounding to zero or overflowing.
const (
	MinPer = 0.0001
	MaxPer = 999999.0
)

// Storable reports whether s has a valid unit and a size the database can hold.
func (s Size) Storable() bool {
	return s.Unit.Valid() && s.Per >= MinPer && s.Per <= MaxPer
}
