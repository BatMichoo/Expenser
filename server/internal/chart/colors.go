package chart

// colorSlots is how many distinct type colours the stylesheet defines
// (--cat-1 .. --cat-8).
const colorSlots = 8

// TypeColor maps a type ID to its fixed colour slot, 1..8, cycling for
// larger IDs. An ID that is not positive (unknown type) gets slot 0, which
// the stylesheet draws as grey.
func TypeColor(typeID int) int {
	if typeID <= 0 {
		return 0
	}
	return (typeID-1)%colorSlots + 1
}
