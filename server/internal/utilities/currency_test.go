package utilities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMoney(t *testing.T) {
	assert.Equal(t, "12.50 €", Money(12.5))
	assert.Equal(t, "0.00 €", Money(0))
	assert.Equal(t, "0.00 €", Money(0.1+0.2-0.3), "float noise above zero")
	assert.Equal(t, "0.00 €", Money(0.3-0.1-0.2), "float noise below zero is not -0.00")
	assert.Equal(t, "-3.20 €", Money(-3.2))
	assert.Equal(t, "1.01 €", Money(1.005+1e-9), "rounds to the cent")
}
