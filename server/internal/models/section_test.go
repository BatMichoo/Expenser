package models

import (
	"testing"

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
