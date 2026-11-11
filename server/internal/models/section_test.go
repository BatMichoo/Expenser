package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSectionValid(t *testing.T) {
	for _, s := range []Section{SectionHouse, SectionCar, SectionGroceries} {
		assert.True(t, s.Valid(), s)
	}
	assert.False(t, Section("boat").Valid())
}

func TestRecurringPaymentTypeID(t *testing.T) {
	house := RecurringPayment{Category: SectionHouse}
	house.SetTypeID(4)
	assert.Equal(t, 4, *house.TypeID())
	assert.Nil(t, house.CarExpenseTypeID)

	car := RecurringPayment{Category: SectionCar}
	car.SetTypeID(2)
	assert.Equal(t, 2, *car.TypeID())
	assert.Nil(t, car.UtilityTypeID)

	assert.Nil(t, (&RecurringPayment{Category: SectionHouse}).TypeID(), "no type yet")
}

func TestDateOnlyKeepsTheCalendarDay(t *testing.T) {
	sofia := time.FixedZone("EET", 2*60*60)
	assert.Equal(t, Date(2026, 11, 1), DateOnly(time.Date(2026, 11, 1, 0, 30, 0, 0, sofia)), "just after midnight local")
	assert.Equal(t, Date(2026, 11, 1), DateOnly(time.Date(2026, 11, 1, 23, 59, 0, 0, sofia)))
	assert.Equal(t, Date(2026, 2, 28), Date(2026, 3, 0), "day 0 is the last day of the month before")
}
