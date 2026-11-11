package models

// Section is a part of the app with its own expenses: house, car or
// groceries. House and car expenses have a type (utility or car expense
// type); bills belong to one of those two.
type Section string

const (
	SectionHouse     Section = "house"
	SectionCar       Section = "car"
	SectionGroceries Section = "groceries"
)

// Valid reports whether s is a known section.
func (s Section) Valid() bool {
	switch s {
	case SectionHouse, SectionCar, SectionGroceries:
		return true
	}
	return false
}
