package handlers

import (
	"context"
	"expenser/internal/config"
	database "expenser/internal/db"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEnv is a router wired like production, backed by the test database,
// with a logged-in user. These tests share the test database with the db
// package tests, so run packages one at a time: go test -p 1 ./...
type testEnv struct {
	t      *testing.T
	router *gin.Engine
	db     *database.DB
	user   *models.User
	cookie *http.Cookie
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	require.NoError(t, utilities.InitI18n())

	db := database.InitTestDB(cfg)
	t.Cleanup(func() { db.Close() })

	user := &models.User{
		Username:     "handler-test-" + uuid.NewString()[:8],
		PasswordHash: database.TestUserRegisterModel.PasswordHash,
	}
	require.NoError(t, db.CreateUser(context.Background(), user))

	as := services.NewAuthService(cfg.JWT.SecretKey, cfg.JWT.TokenExpiration, cfg.LanDomain)
	token, err := as.GenerateToken(user)
	require.NoError(t, err)

	router := gin.New()
	router.SetHTMLTemplate(template.Must(template.New("").Funcs(utilities.TemplateFuncs()).ParseGlob("../templates/*/*.html")))
	RegisterRoutes(router, db, cfg)

	return &testEnv{
		t:      t,
		router: router,
		db:     db,
		user:   user,
		cookie: &http.Cookie{Name: "auth_token", Value: token.Value},
	}
}

// do sends an HTMX request with form values and returns the recorder.
func (e *testEnv) do(method, path string, form url.Values) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(e.cookie)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func (e *testEnv) categoryID(name string) int {
	e.t.Helper()
	id, err := e.db.GetCategoryIDByName(context.Background(), name)
	require.NoError(e.t, err)
	return id
}

func (e *testEnv) createExpense(product string, date time.Time) *models.GroceriesExpense {
	e.t.Helper()
	exp := &models.GroceriesExpense{
		UserID:       e.user.ID,
		CategoryID:   e.categoryID("Milk"),
		Product:      product,
		Quantity:     1,
		UnitPrice:    2,
		TotalPrice:   2,
		PurchaseDate: date,
	}
	require.NoError(e.t, e.db.CreateGroceriesExpense(context.Background(), exp))
	return exp
}

func (e *testEnv) expensesFor(date time.Time) []models.GroceriesExpense {
	e.t.Helper()
	got, err := e.db.GetGroceriesExpensesForMonth(context.Background(), date.Month(), date.Year(), e.user.ID)
	require.NoError(e.t, err)
	return got
}

func itemForm(categoryID int, product, quantity, unitPrice, date string) url.Values {
	return url.Values{
		"category_id":   {strconv.Itoa(categoryID)},
		"product":       {product},
		"quantity":      {quantity},
		"unit_price":    {unitPrice},
		"total_price":   {unitPrice},
		"purchase_date": {date},
	}
}

func TestEditGroceriesKeepsPurchaseDate(t *testing.T) {
	env := newTestEnv(t)
	march := time.Date(2025, time.March, 15, 0, 0, 0, 0, time.UTC)
	exp := env.createExpense("Milk", march)

	w := env.do(http.MethodPut, "/groceries/"+strconv.Itoa(exp.ID), itemForm(env.categoryID("Milk"), "Whole milk", "2", "2.50", "2025-03-15"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := env.db.GetGroceriesExpenseByID(context.Background(), exp.ID, env.user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Whole milk", got.Product)
	assert.Equal(t, "2025-03-15", got.PurchaseDate.Format("2006-01-02"))
}

func TestCreateGroceriesValidation(t *testing.T) {
	env := newTestEnv(t)
	today := time.Now().Format("2006-01-02")
	milk := env.categoryID("Milk")

	tests := []struct {
		name string
		form url.Values
	}{
		{"missing product", itemForm(milk, "", "1", "2", today)},
		{"bad quantity", itemForm(milk, "Milk", "abc", "2", today)},
		{"zero quantity", itemForm(milk, "Milk", "0", "2", today)},
		{"negative price", itemForm(milk, "Milk", "1", "-2", today)},
		{"missing category", itemForm(0, "Milk", "1", "2", today)},
		{"bad date", itemForm(milk, "Milk", "1", "2", "yesterday")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "missing category" {
				tt.form.Del("category_id")
			}
			w := env.do(http.MethodPost, "/groceries", tt.form)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
	assert.Empty(t, env.expensesFor(time.Now()), "invalid input must not be saved")

	t.Run("valid in current month adds a row", func(t *testing.T) {
		w := env.do(http.MethodPost, "/groceries", itemForm(milk, "Milk", "1", "2", today))
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "<tr id=\"grocery-")
		assert.Contains(t, w.Body.String(), `id="total-expense" hx-swap-oob="true"`)
	})

	t.Run("valid in another month saves without a row", func(t *testing.T) {
		w := env.do(http.MethodPost, "/groceries", itemForm(milk, "Old milk", "1", "2", "2025-01-10"))
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		assert.NotContains(t, w.Body.String(), "<tr id=\"grocery-")
		assert.Len(t, env.expensesFor(time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)), 1)
	})
}

func TestConfirmBatchReceipt(t *testing.T) {
	env := newTestEnv(t)
	receiptDay := time.Date(2025, time.February, 20, 0, 0, 0, 0, time.UTC)

	batch := func(secondQuantity string) url.Values {
		return url.Values{
			"purchase_date":             {"2025-02-20"},
			"items[0].product":          {"Bread"},
			"items[0].category_id":      {strconv.Itoa(env.categoryID("Bread"))},
			"items[0].quantity":         {"1"},
			"items[0].unit_price":       {"1.20"},
			"items[0].total_price":      {"1.20"},
			"items[0].supermarket_name": {"Lidl"},
			"items[1].product":          {"Mystery"},
			"items[1].category_id":      {""},
			"items[1].quantity":         {secondQuantity},
			"items[1].unit_price":       {"3"},
			"items[1].total_price":      {"3"},
			"items[1].supermarket_name": {"Lidl"},
		}
	}

	t.Run("an invalid item saves nothing", func(t *testing.T) {
		w := env.do(http.MethodPost, "/groceries/confirm-batch", batch("lots"))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Item 2")
		assert.Empty(t, env.expensesFor(receiptDay))
	})

	t.Run("valid batch uses receipt date and defaults to Other", func(t *testing.T) {
		w := env.do(http.MethodPost, "/groceries/confirm-batch", batch("1"))
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		// February is not the current month, so no rows are added to the table.
		assert.NotContains(t, w.Body.String(), "<tr id=\"grocery-")

		got := env.expensesFor(receiptDay)
		require.Len(t, got, 2)
		categories := map[string]string{}
		for _, e := range got {
			categories[e.Product] = e.CategoryName
			assert.Equal(t, "2025-02-20", e.PurchaseDate.Format("2006-01-02"))
		}
		assert.Equal(t, "Bread", categories["Bread"])
		assert.Equal(t, "Other", categories["Mystery"])
	})
}

func TestGroceriesCurrentMonthOnly(t *testing.T) {
	env := newTestEnv(t)
	env.createExpense("This month milk", time.Now())
	env.createExpense("Last year milk", time.Now().AddDate(-1, 0, 0))

	w := env.do(http.MethodGet, "/groceries/current", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "This month milk")
	assert.NotContains(t, w.Body.String(), "Last year milk")
	assert.Contains(t, w.Body.String(), `id="highest-expense"`)
}

func TestGroceriesSearch(t *testing.T) {
	env := newTestEnv(t)
	env.createExpense("March milk", time.Date(2025, time.March, 3, 0, 0, 0, 0, time.UTC))
	env.createExpense("April milk", time.Date(2025, time.April, 3, 0, 0, 0, 0, time.UTC))

	w := env.do(http.MethodGet, "/groceries/search", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `hx-post="./groceries/search"`)

	w = env.do(http.MethodPost, "/groceries/search", url.Values{"date": {"2025-03"}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "March milk")
	assert.NotContains(t, w.Body.String(), "April milk")
	assert.Contains(t, w.Body.String(), ">2.00</span>")
}

func TestDeleteGroceriesRefreshesCards(t *testing.T) {
	env := newTestEnv(t)
	exp := env.createExpense("Milk", time.Now())

	w := env.do(http.MethodDelete, "/groceries/"+strconv.Itoa(exp.ID), nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "<tr")
	assert.Contains(t, w.Body.String(), `id="total-expense" hx-swap-oob="true"`)
	assert.Empty(t, env.expensesFor(time.Now()))

	w = env.do(http.MethodDelete, "/groceries/"+strconv.Itoa(exp.ID), nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
