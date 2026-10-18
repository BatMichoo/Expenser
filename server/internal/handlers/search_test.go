package handlers

import (
	"context"
	"expenser/internal/models"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchFormPerSection(t *testing.T) {
	env := newTestEnv(t)

	tests := []struct {
		path, action, typeHeader string
	}{
		{"/car/search", "./car/search", "Expense Type"},
		{"/house/search", "./house/search", "Utility Type"},
		{"/groceries/search", "./groceries/search", "Supermarket"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := env.do(http.MethodGet, tt.path, nil)
			require.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), `hx-post="`+tt.action+`"`)
			assert.Contains(t, w.Body.String(), "<th>"+tt.typeHeader+"</th>")
		})
	}
}

func TestCarSearchTotalUsesSearchedYear(t *testing.T) {
	env := newTestEnv(t)
	thisYear := time.Now().Year()

	for _, e := range []struct {
		year   int
		amount float64
	}{{thisYear - 1, 30}, {thisYear, 50}} {
		require.NoError(t, env.db.CreateCarExpense(context.Background(), &models.CarExpense{
			CreatedBy:     env.user.ID,
			ExpenseTypeID: 4,
			Amount:        e.amount,
			Date:          time.Date(e.year, time.March, 10, 0, 0, 0, 0, time.UTC),
		}))
	}

	w := env.do(http.MethodPost, "/car/search", url.Values{"date": {fmt.Sprintf("%d-03", thisYear-1)}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `<span id="total" hx-swap-oob="true">30.00</span>`)
}
