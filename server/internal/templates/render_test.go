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
	themes := menu.Find(".theme-option")
	require.Equal(t, 2, themes.Length(), "light and dark options")
	light, _ := themes.Eq(0).Attr("data-theme")
	dark, _ := themes.Eq(1).Attr("data-theme")
	assert.Equal(t, []string{"light", "dark"}, []string{light, dark})
	assert.Contains(t, menu.Text(), "Theme")
	assert.NotContains(t, menu.Text(), "Toggle Theme", "state-less toggle replaced")
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
	assert.Equal(t, 2, doc.Find("details.settings-menu .theme-option").Length(), "theme still reachable")
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

func houseTypes() *[]models.HomeUtilityType {
	return &[]models.HomeUtilityType{{ID: 1, Name: "Electricity"}, {ID: 3, Name: "Gas"}, {ID: 7, Name: "Other"}}
}

func carTypes() *[]models.CarExpenseType {
	return &[]models.CarExpenseType{{ID: 1, Name: "Fuel"}, {ID: 3, Name: "Insurance"}}
}

// assertSheetForm checks the parts every add/edit sheet shares.
func assertSheetForm(t *testing.T, doc *goquery.Document, typeCount int, metaURL string) {
	t.Helper()
	assert.Equal(t, 0, doc.Find(`select[name="typeID"]`).Length(), "no type dropdown")
	chips := doc.Find(`fieldset.type-chips`)
	require.Equal(t, 1, chips.Length())
	assert.Equal(t, typeCount, chips.Find(`input[type="radio"][name="typeID"]`).Length())
	hxGet, _ := chips.Attr("hx-get")
	assert.Equal(t, metaURL, hxGet)
	include, _ := chips.Attr("hx-include")
	assert.Equal(t, "this", include)

	amount := doc.Find(`input[name="amount"]`)
	mode, _ := amount.Attr("inputmode")
	assert.Equal(t, "decimal", mode)
	assert.Equal(t, 0, doc.Find(`[hidden]`).Length(), "no hidden pick-a-type-first step")
	more := doc.Find("details.more-fields")
	require.Equal(t, 1, more.Length())
	assert.Equal(t, 1, more.Find("#metadata-fields").Length())
	assert.Equal(t, 1, more.Find(`[name="discount_amount"]`).Length())
	assert.Equal(t, 1, more.Find(`[name="notes"]`).Length())
}

func TestCreateHouseFormChips(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	doc := parseHTML(t, render(t, "create-house-exp-form", map[string]any{"Types": houseTypes(), "Lang": "en", "Date": today}))
	assertSheetForm(t, doc, 3, "/house/expenses/metadata")
	assert.Equal(t, 0, doc.Find(`input[name="typeID"][checked]`).Length())
	date, _ := doc.Find(`input[name="date"]`).Attr("value")
	assert.Equal(t, today, date)
	_, open := doc.Find("details.more-fields").Attr("open")
	assert.False(t, open, "extras folded when adding")
	assert.Equal(t, 0, doc.Find(".sheet-delete").Length())
}

func TestEditHouseFormDelete(t *testing.T) {
	e := sampleHouseExpense()
	doc := parseHTML(t, render(t, "edit-house-exp-form", &models.EditHouseFormData{Expense: e, Types: houseTypes(), Lang: "en"}))
	assertSheetForm(t, doc, 3, "/house/expenses/metadata")
	checked, _ := doc.Find(`input[name="typeID"][checked]`).Attr("value")
	assert.Equal(t, "3", checked)
	_, open := doc.Find("details.more-fields").Attr("open")
	assert.True(t, open, "extras expanded when editing")
	assert.Equal(t, "Meter read", doc.Find(`textarea[name="notes"]`).Text(), "no stray whitespace")
	form := doc.Find("form.new-expense-form")
	put, _ := form.Attr("hx-put")
	assert.Equal(t, "/house/expenses/42", put)
	del, _ := doc.Find("button.sheet-delete").Attr("hx-get")
	assert.Equal(t, "/house/expenses/delete/42", del)
	swap, _ := doc.Find("button.sheet-delete").Attr("hx-swap")
	assert.Equal(t, "innerHTML", swap, "must not inherit the form's outerHTML swap and replace the dialog")
}

func TestCreateCarFormChips(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	doc := parseHTML(t, render(t, "create-car-exp-form", map[string]any{"Types": carTypes(), "Lang": "en", "Date": today}))
	assertSheetForm(t, doc, 2, "/car/expenses/metadata")
	date, _ := doc.Find(`input[name="date"]`).Attr("value")
	assert.Equal(t, today, date)
}

func TestEditCarFormDelete(t *testing.T) {
	e := &models.CarExpense{ID: 7, ExpenseTypeID: 1, Type: "Fuel", Amount: 60, Notes: "Shell", Date: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)}
	doc := parseHTML(t, render(t, "edit-car-exp-form", &models.EditCarFormData{Expense: e, Types: carTypes(), Lang: "en"}))
	assertSheetForm(t, doc, 2, "/car/expenses/metadata")
	checked, _ := doc.Find(`input[name="typeID"][checked]`).Attr("value")
	assert.Equal(t, "1", checked)
	assert.Equal(t, "Shell", doc.Find(`textarea[name="notes"]`).Text())
	put, _ := doc.Find("form.new-expense-form").Attr("hx-put")
	assert.Equal(t, "/car/expenses/7", put)
	del, _ := doc.Find("button.sheet-delete").Attr("hx-get")
	assert.Equal(t, "/car/expenses/delete/7", del)
}

func groceryCategories() []map[string]any {
	return []map[string]any{{"ID": 1, "Name": "Meat"}, {"ID": 2, "Name": "Dairy"}}
}

func TestCreateGroceriesFormSheet(t *testing.T) {
	doc := parseHTML(t, render(t, "create-groceries-form", map[string]any{"Categories": groceryCategories(), "Lang": "en"}))
	form := doc.Find("form.new-expense-form")
	post, _ := form.Attr("hx-post")
	assert.Equal(t, "/groceries", post, "submits from the form so Enter works")
	assert.Equal(t, 1, doc.Find(".sheet .sheet-grab").Length())
	mode, _ := doc.Find(`input[name="total_price"]`).Attr("inputmode")
	assert.Equal(t, "decimal", mode)
	assert.Equal(t, 1, doc.Find(".qty-unit").Find(`[name="quantity"]`).Length())
	assert.Equal(t, 1, doc.Find(".qty-unit").Find(`[name="unit_price"]`).Length())
	more := doc.Find("details.more-fields")
	assert.Equal(t, 1, more.Find(`[name="discount_per_unit"]`).Length())
	assert.Equal(t, 1, more.Find(`[name="total_discount"]`).Length())
	assert.Equal(t, 0, doc.Find(".sheet-delete").Length())
	assert.NotContains(t, doc.Find(".new-expense-heading").Text(), "form.", "heading is translated")
}

func TestEditGroceriesFormDelete(t *testing.T) {
	g := &models.GroceriesExpense{ID: 9, CategoryID: 2, Product: "Milk", Quantity: 2, UnitPrice: 1.45, TotalPrice: 2.9}
	doc := parseHTML(t, render(t, "edit-groceries-form", map[string]any{"Expense": g, "Categories": groceryCategories(), "Lang": "en"}))
	form := doc.Find("form.new-expense-form")
	put, _ := form.Attr("hx-put")
	assert.Equal(t, "/groceries/9", put)
	sel, _ := doc.Find(`select[name="category_id"] option[selected]`).Attr("value")
	assert.Equal(t, "2", sel)
	del := doc.Find("button.sheet-delete")
	require.Equal(t, 1, del.Length())
	hxDel, _ := del.Attr("hx-delete")
	assert.Equal(t, "/groceries/9", hxDel)
	target, _ := del.Attr("hx-target")
	assert.Equal(t, "#grocery-9", target)
	swap, _ := del.Attr("hx-swap")
	assert.Equal(t, "outerHTML", swap)
	_, confirm := del.Attr("hx-confirm")
	assert.True(t, confirm)
	_, open := doc.Find("details.more-fields").Attr("open")
	assert.True(t, open)
}
