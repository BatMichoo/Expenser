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

// Go's \b is ASCII-only, so token boundaries are spelled out explicitly. The
// left boundary is the start, any non-digit non-dot character (letters too:
// "Домати400гр"), or an abbreviation dot after a letter ("б.ф.200г"). A dot
// after a digit or at the start (".5л") is not a boundary.
const (
	numPat   = `(\d+(?:\.\d+)?)`
	unitPat  = `(кг|kg|мл|ml|бр|pcs|pc|гр|г|g|л|l)`
	leftPat  = `(?:^|[^\d.]|\p{L}\.)`
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

// Round4 rounds a pack size to 4 decimals, the precision of the unit_size
// NUMERIC(10,4) column.
func Round4(v float64) float64 {
	return math.Round(v*1e4) / 1e4
}

// normalise converts a parsed amount and unit token to a base unit, rounded
// to 4 decimals. Grams and millilitres are rounded before dividing by 1000 so
// a half step (43.75 g) rounds up as written instead of by float noise.
func normalise(amount float64, token string) Size {
	switch token {
	case "кг", "kg":
		return Size{Kg, Round4(amount)}
	case "г", "гр", "g":
		return Size{Kg, math.Round(amount*10) / 1e4}
	case "мл", "ml":
		return Size{Liter, math.Round(amount*10) / 1e4}
	case "л", "l":
		return Size{Liter, Round4(amount)}
	default:
		return Size{Piece, Round4(amount)}
	}
}

// yearRe matches a four-digit number; with the unit "г" and a dot, space or
// end after it, a number in 1900..2100 is a year ("Вино 2019г."), not grams.
var yearRe = regexp.MustCompile(`^\d{4}$`)

func isYear(number, token, after string) bool {
	if token != "г" || !yearRe.MatchString(number) {
		return false
	}
	if after != "" && after[0] != '.' && after[0] != ' ' && after[0] != '\t' {
		return false
	}
	y, err := strconv.Atoi(number)
	return err == nil && y >= 1900 && y <= 2100
}

// Parse reads the pack size from a product name. It returns false unless the
// size is certain: exactly one size in the name (a repeated size, equal or
// not, is ambiguous), and for a fractional (weighed) quantity only a size of
// exactly 1 kg or 1 l. A name without a size but with a fractional quantity
// is treated as sold by weight (one kg per counted unit).
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
		found = append(found, normalise(n*v, s[m[6]:m[7]]))
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
		number, token := s[pos+m[2]:pos+m[3]], s[pos+m[4]:pos+m[5]]
		end := pos + m[5]
		pos = end
		if isYear(number, token, s[end:]) {
			continue
		}
		v, err := strconv.ParseFloat(number, 64)
		if err != nil {
			return Size{}, false
		}
		found = append(found, normalise(v, token))
	}

	if len(found) == 0 {
		if quantity > 0 && fractional(quantity) {
			return Size{Kg, 1}, true
		}
		return Size{}, false
	}
	if len(found) > 1 {
		return Size{}, false
	}
	size := found[0]
	if size.Per <= 0 {
		return Size{}, false
	}
	if fractional(quantity) && !(size.Per == 1 && (size.Unit == Kg || size.Unit == Liter)) {
		return Size{}, false
	}
	return size, true
}

// fractional reports whether a quantity is not a whole number, i.e. weighed.
func fractional(quantity float64) bool {
	return math.Abs(quantity-math.Round(quantity)) > 1e-9
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
	if per := Round4(aiPer); ai.Valid() && per > 0 {
		return Size{ai, per}, true
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
