package templates

import (
	"bytes"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"html/template"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	parseOnce sync.Once
	parsed    *template.Template
	parseErr  error
)

// loadTemplates parses every template the way cmd/main.go does.
func loadTemplates(t *testing.T) *template.Template {
	t.Helper()
	parseOnce.Do(func() {
		if parseErr = utilities.InitI18n(); parseErr != nil {
			return
		}
		parsed, parseErr = template.New("").Funcs(utilities.TemplateFuncs()).
			ParseGlob(filepath.Join(".", "**", "*.html"))
	})
	require.NoError(t, parseErr)
	return parsed
}

// render executes one named template and returns its HTML.
func render(t *testing.T, name string, data any) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, loadTemplates(t).ExecuteTemplate(&buf, name, data))
	return buf.String()
}

// parseHTML turns rendered markup into a queryable document.
func parseHTML(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	return doc
}

func TestAllTemplatesParse(t *testing.T) {
	tmpl := loadTemplates(t)
	require.NotNil(t, tmpl.Lookup("house-exp-row"))
	require.NotNil(t, tmpl.Lookup("root"))
}

func TestHeaderSettingsMenu(t *testing.T) {
	html := render(t, "header", models.HeaderOptions{IsLoggedIn: true, Lang: "en"})
	doc := parseHTML(t, html)

	menu := doc.Find("details.settings-menu")
	require.Equal(t, 1, menu.Length(), "settings menu present")
	assert.Equal(t, 1, menu.Find("#theme-toggle").Length(), "theme toggle inside menu")
	assert.Equal(t, 2, menu.Find(".lang-btn").Length(), "language buttons inside menu")
	assert.Equal(t, 1, menu.Find(`[hx-get="/logout"]`).Length(), "logout inside menu")

	nav := doc.Find("#tracker-navigation")
	assert.Equal(t, 3, nav.Find(".tracker-nav-button").Length(), "house, car, groceries")
	assert.Equal(t, 0, nav.Find(`[hx-get="/logout"]`).Length(), "logout no longer a tab")
}

func TestHeaderLoggedOut(t *testing.T) {
	doc := parseHTML(t, render(t, "header", models.HeaderOptions{Lang: "en"}))
	assert.Equal(t, 0, doc.Find(`[hx-get="/logout"]`).Length())
	assert.Equal(t, 2, doc.Find("#tracker-navigation .tracker-nav-button").Length(), "login, register")
	assert.Equal(t, 1, doc.Find("details.settings-menu #theme-toggle").Length(), "theme still reachable")
}

func sampleHouseExpense() *models.HouseExpense {
	return &models.HouseExpense{
		ID:            42,
		UtilityTypeID: 3,
		UtilityType:   "Gas",
		Amount:        85.01,
		ExpenseDate:   time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC),
		Notes:         "Meter read",
		Metadata:      []byte(`{"quantity": 42.5, "price_per_unit": 2}`),
	}
}

func TestHouseRowCompactMarkup(t *testing.T) {
	doc := parseHTML(t, "<table>"+render(t, "house-exp-row", map[string]any{"Exp": sampleHouseExpense(), "Lang": "en"})+"</table>")
	row := doc.Find("tr#exp-42")

	assert.True(t, row.HasClass("exp-row"))
	hxGet, _ := row.Attr("hx-get")
	assert.Equal(t, "/house/expenses/edit/42", hxGet)
	target, _ := row.Attr("hx-target")
	assert.Equal(t, "#action-dialog", target)
	trigger, _ := row.Attr("hx-trigger")
	assert.Equal(t, "click[!event.target.closest('button')]", trigger)

	assert.Equal(t, "85.01", strings.TrimSpace(row.Find("td.c-amount").Text()))
	assert.True(t, row.Find("td.c-discount").HasClass("is-empty"), "zero discount hidden")
	meta := row.Find("td.c-meta").Text()
	assert.Contains(t, meta, "13.11")
	assert.Contains(t, meta, "Qty: 42.50")
	assert.Contains(t, meta, "Meter read")
}

func TestHouseRowDiscountShown(t *testing.T) {
	e := sampleHouseExpense()
	e.DiscountAmount = 2
	doc := parseHTML(t, "<table>"+render(t, "house-exp-row", map[string]any{"Exp": e, "Lang": "en"})+"</table>")
	cell := doc.Find("td.c-discount")
	assert.False(t, cell.HasClass("is-empty"))
	assert.Contains(t, cell.Text(), "2.00")
}

func TestCarRowCompactMarkup(t *testing.T) {
	e := &models.CarExpense{ID: 7, Type: "Fuel", Amount: 60, Date: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)}
	doc := parseHTML(t, "<table>"+render(t, "car-exp-row", map[string]any{"Exp": e, "Lang": "en"})+"</table>")
	row := doc.Find("tr#exp-7")
	assert.True(t, row.HasClass("exp-row"))
	hxGet, _ := row.Attr("hx-get")
	assert.Equal(t, "/car/expenses/edit/7", hxGet)
	assert.Equal(t, "60.00", strings.TrimSpace(row.Find("td.c-amount").Text()))
	assert.True(t, row.Find("td.c-notes").HasClass("is-empty"))
}

func TestGroceryRowCompactMarkup(t *testing.T) {
	g := &models.GroceriesExpense{
		ID: 9, CategoryName: "Meat", Product: "Chicken breast", Quantity: 0.8, UnitPrice: 9.9,
		TotalPrice: 7.92, TotalDiscount: 1.2, SupermarketName: "Lidl",
		PurchaseDate: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC),
	}
	doc := parseHTML(t, "<table>"+render(t, "grocery-row", map[string]any{"G": g, "Lang": "en"})+"</table>")
	row := doc.Find("tr#grocery-9")

	assert.True(t, row.HasClass("exp-row"))
	hxGet, _ := row.Attr("hx-get")
	assert.Equal(t, "/groceries/edit/9", hxGet)
	assert.Equal(t, "Chicken breast", strings.TrimSpace(row.Find("td.c-product").Text()))
	assert.Equal(t, "7.92", strings.TrimSpace(row.Find("td.c-amount").Text()))
	assert.False(t, row.Find("td.c-discount").HasClass("is-empty"))
	meta := row.Find("td.c-meta").Text()
	assert.Contains(t, meta, "0.8 × 9.90")
	assert.Contains(t, meta, "Lidl")
	assert.Contains(t, meta, "04.11")
}

func TestEmptyListRow(t *testing.T) {
	data := &models.HouseData{
		MonthlyExpense: &models.MonthlyExpense{}, HighestExpense: &models.HighestExpense{},
		RecentExpenses: &[]models.HouseExpense{}, Lang: "en",
	}
	doc := parseHTML(t, render(t, "house-current", data))
	empty := doc.Find("tr.empty-row")
	require.Equal(t, 1, empty.Length())
	assert.Contains(t, empty.Text(), "No")
}

func TestGroceriesCurrentUsesGroceryRow(t *testing.T) {
	rows := []models.GroceriesExpense{{ID: 1, Product: "Milk", CategoryName: "Dairy", PurchaseDate: time.Now()}}
	data := &models.GroceriesData{
		MonthlyExpense: &models.MonthlyExpense{}, HighestExpense: &models.HighestExpense{},
		RecentExpenses: &rows, Lang: "en",
	}
	doc := parseHTML(t, render(t, "groceries-current", data))
	assert.Equal(t, 1, doc.Find("tr#grocery-1.exp-row").Length())
}

func TestCurrentViewHasFab(t *testing.T) {
	cases := []struct {
		tmpl string
		data any
		url  string
	}{
		{"house-current", &models.HouseData{MonthlyExpense: &models.MonthlyExpense{}, HighestExpense: &models.HighestExpense{}, RecentExpenses: &[]models.HouseExpense{}, Lang: "en"}, "/house/expenses/new"},
		{"car-current", &models.CarData{MonthlyExpense: &models.MonthlyExpense{}, HighestExpense: &models.HighestExpense{}, RecentExpenses: &[]models.CarExpense{}, Lang: "en"}, "/car/expenses/new"},
		{"groceries-current", &models.GroceriesData{MonthlyExpense: &models.MonthlyExpense{}, HighestExpense: &models.HighestExpense{}, RecentExpenses: &[]models.GroceriesExpense{}, Lang: "en"}, "/groceries/create"},
	}
	for _, tc := range cases {
		t.Run(tc.tmpl, func(t *testing.T) {
			fab := parseHTML(t, render(t, tc.tmpl, tc.data)).Find("button.fab")
			require.Equal(t, 1, fab.Length())
			hxGet, _ := fab.Attr("hx-get")
			assert.Equal(t, tc.url, hxGet)
			target, _ := fab.Attr("hx-target")
			assert.Equal(t, "#action-dialog", target)
			label, _ := fab.Attr("aria-label")
			assert.NotEmpty(t, label)
		})
	}
}
