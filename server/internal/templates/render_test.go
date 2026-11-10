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

	assert.Equal(t, "85.01 €", strings.TrimSpace(row.Find("td.c-amount").Text()))
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
	assert.Equal(t, "60.00 €", strings.TrimSpace(row.Find("td.c-amount").Text()))
	assert.True(t, row.Find("td.c-details").HasClass("is-empty"), "no details and no notes")
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
	assert.Equal(t, "7.92 €", strings.TrimSpace(row.Find("td.c-amount").Text()))
	assert.False(t, row.Find("td.c-discount").HasClass("is-empty"))
	meta := row.Find("td.c-meta").Text()
	assert.Contains(t, meta, "0.8 × 9.90")
	assert.Contains(t, meta, "Lidl")
	assert.Contains(t, meta, "04.11")
}

func monthView(section string) *models.MonthView {
	return &models.MonthView{
		Section: section, Month: "2026-11", MonthNum: time.November, Year: 2026,
		PrevMonth: "2026-10", NextMonth: "2026-12", IsCurrent: true, Lang: "en",
		TypePrefix: "type.",
		Bars: []models.MonthBar{
			{Month: "2026-06", MonthNum: time.June}, {Month: "2026-07", MonthNum: time.July},
			{Month: "2026-08", MonthNum: time.August}, {Month: "2026-09", MonthNum: time.September, Height: 50},
			{Month: "2026-10", MonthNum: time.October, Height: 100}, {Month: "2026-11", MonthNum: time.November, Height: 80, Current: true},
		},
	}
}

func TestEmptyListRow(t *testing.T) {
	doc := parseHTML(t, render(t, "house-current", monthView("house")))
	empty := doc.Find("tr.empty-row")
	require.Equal(t, 1, empty.Length())
	assert.Contains(t, empty.Text(), "No recent expenses")

	filtered := monthView("house")
	filtered.Filtered = true
	doc = parseHTML(t, render(t, "house-current", filtered))
	assert.Contains(t, doc.Find("tr.empty-row").Text(), "match your search", "different message when filtering")
}

func TestGroceriesCurrentUsesGroceryRow(t *testing.T) {
	v := monthView("groceries")
	v.TypePrefix = "category."
	v.Groceries = []models.GroceriesExpense{{ID: 1, Product: "Milk", CategoryName: "Dairy", PurchaseDate: time.Now()}}
	doc := parseHTML(t, render(t, "groceries-current", v))
	assert.Equal(t, 1, doc.Find("tr#grocery-1.exp-row").Length())
	assert.Equal(t, 1, doc.Find("button.upload-btn").Length(), "receipt upload stays on groceries")
}

func TestCurrentViewHasFab(t *testing.T) {
	for section, url := range map[string]string{"house": "/house/expenses/new", "car": "/car/expenses/new", "groceries": "/groceries/create"} {
		t.Run(section, func(t *testing.T) {
			doc := parseHTML(t, render(t, section+"-current", monthView(section)))
			fab := doc.Find("button.fab")
			require.Equal(t, 1, fab.Length())
			hxGet, _ := fab.Attr("hx-get")
			assert.Equal(t, url, hxGet)
			label, _ := fab.Attr("aria-label")
			assert.NotEmpty(t, label)
			add, _ := doc.Find(".month-toolbar button.add-btn").Attr("hx-get")
			assert.Equal(t, url, add, "toolbar add button")
		})
	}
}

func TestMonthToolbar(t *testing.T) {
	v := monthView("car")
	v.Query = "fuel"
	v.TypeID = 3
	v.Types = []models.TypeOption{{ID: 1, Name: "Fuel"}, {ID: 3, Name: "Insurance"}}
	doc := parseHTML(t, render(t, "car-current", v))

	prev := doc.Find(`.month-switcher a[aria-label="Previous month"]`)
	href, _ := prev.Attr("href")
	assert.Equal(t, "/car?month=2026-10", href, "works without JavaScript")
	hxGet, _ := prev.Attr("hx-get")
	assert.Equal(t, "/car/current?month=2026-10", hxGet)
	push, _ := prev.Attr("hx-push-url")
	assert.Equal(t, "/car?month=2026-10", push)
	next, _ := doc.Find(`.month-switcher a[aria-label="Next month"]`).Attr("hx-get")
	assert.Equal(t, "/car/current?month=2026-12", next)
	assert.Contains(t, doc.Find(".month-label").Text(), "November 2026")
	assert.Equal(t, 0, doc.Find(".month-today").Length(), "already on the current month")

	form := doc.Find("form#month-filters")
	trigger, _ := form.Attr("hx-trigger")
	assert.Contains(t, trigger, "input delay:300ms")
	sel, _ := form.Attr("hx-select")
	assert.Equal(t, "#recent-expenses", sel)
	q, _ := form.Find(`input[name="q"]`).Attr("value")
	assert.Equal(t, "fuel", q)
	month, _ := form.Find(`input[name="month"]`).Attr("value")
	assert.Equal(t, "2026-11", month)
	selected, _ := form.Find(`select[name="type"] option[selected]`).Attr("value")
	assert.Equal(t, "3", selected)

	wrapper := doc.Find("#month-view")
	refresh, _ := wrapper.Attr("hx-trigger")
	assert.Equal(t, "expenses-changed from:body", refresh)
	include, _ := wrapper.Attr("hx-include")
	assert.Equal(t, "#month-filters", include, "refresh keeps the search")
	disinherit, _ := wrapper.Attr("hx-disinherit")
	assert.Equal(t, "*", disinherit, "children must not inherit the wrapper's outerHTML swap")

	v.IsCurrent = false
	doc = parseHTML(t, render(t, "car-current", v))
	assert.Equal(t, 1, doc.Find(".month-today").Length(), "way back to this month")
}

func TestKPITiles(t *testing.T) {
	v := monthView("house")
	v.Summary = models.MonthSummary{Total: 183.63, Count: 5, Average: 36.73, TopType: "Gas", TopAmount: 85.01}
	v.HasChange, v.ChangePct = true, -12.4
	doc := parseHTML(t, render(t, "house-current", v))
	assert.Contains(t, doc.Find(".kpi-total .kpi-value").Text(), "183.63 €")
	assert.Equal(t, "-12%", strings.TrimSpace(doc.Find(".kpi-change").Text()))
	assert.True(t, doc.Find(".kpi-change").HasClass("down"))
	assert.Contains(t, doc.Find(".kpi-highest").Text(), "Gas")
	assert.Contains(t, doc.Find(".kpi-count").Text(), "36.73 €")

	v.HasChange = false
	doc = parseHTML(t, render(t, "house-current", v))
	assert.Equal(t, 0, doc.Find(".kpi-change").Length(), "no percentage without last month")
}

func TestMiniChart(t *testing.T) {
	doc := parseHTML(t, render(t, "house-current", monthView("house")))
	bars := doc.Find(".mini-chart .mini-bar")
	assert.Equal(t, 6, bars.Length())
	assert.Equal(t, 1, doc.Find(".mini-chart .mini-bar.current").Length())
	style, _ := bars.Eq(4).Attr("style")
	assert.Contains(t, style, "height: 100%")
	link, _ := doc.Find(".mini-chart-bars").Attr("hx-get")
	assert.Equal(t, "/house/chart", link)
	assert.Contains(t, doc.Find(".mini-chart-labels").Text(), "Jun")
}

func TestRowActionsAreLabelledIcons(t *testing.T) {
	doc := parseHTML(t, "<table>"+render(t, "house-exp-row", map[string]any{"Exp": sampleHouseExpense(), "Lang": "en"})+"</table>")
	btns := doc.Find(".row-actions button.icon-btn")
	require.Equal(t, 2, btns.Length())
	edit, _ := btns.Eq(0).Attr("aria-label")
	del, _ := btns.Eq(1).Attr("aria-label")
	assert.Equal(t, []string{"Edit", "Delete"}, []string{edit, del})
	hx, _ := btns.Eq(1).Attr("hx-get")
	assert.Equal(t, "/house/expenses/delete/42", hx)
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

func TestInsightPanels(t *testing.T) {
	v := monthView("house")
	v.Breakdown = []models.TypeShare{
		{Type: "Gas", Amount: 85.01, Share: 46.3, Usual: 72, HasUsual: true, VsUsual: 18.1},
		{Type: "TV", Amount: 9.99, Share: 5.4},
	}
	v.YearSoFar = models.YearSummary{Total: 1834.5, Months: 11, Average: 166.77, VsAverage: 10.1, HasComparison: true}
	v.Usual = 170
	doc := parseHTML(t, render(t, "house-current", v))

	items := doc.Find(".breakdown .breakdown-item")
	require.Equal(t, 2, items.Length())
	assert.Contains(t, items.Eq(0).Text(), "Gas")
	assert.Contains(t, items.Eq(0).Text(), "85.01 €")
	assert.Contains(t, items.Eq(0).Find(".vs-usual").Text(), "+18%")
	assert.True(t, items.Eq(0).Find(".vs-usual").HasClass("up"), "well above usual is flagged")
	style, _ := items.Eq(0).Find(".breakdown-bar span").Attr("style")
	assert.Contains(t, style, "width: 46%")
	assert.Contains(t, items.Eq(1).Find(".vs-usual").Text(), "New", "no history")

	year := doc.Find(".year-so-far")
	assert.Contains(t, year.Text(), "2026")
	assert.Contains(t, year.Text(), "1834.50 €")
	assert.Contains(t, year.Text(), "166.77 €")
	assert.Contains(t, year.Find(".vs-usual").Text(), "+10%")

	assert.Equal(t, 0, doc.Find(".estimate").Length(), "no estimate unless HasEstimate")
	v.HasEstimate, v.Estimate = true, 212.4
	doc = parseHTML(t, render(t, "house-current", v))
	assert.Contains(t, doc.Find(".estimate").Text(), "212.40 €")
	assert.Contains(t, doc.Find(".estimate").Text(), "170.00 €", "explains the usual month it is based on")
}

func TestInsightPanelsEmptyMonth(t *testing.T) {
	doc := parseHTML(t, render(t, "house-current", monthView("house")))
	assert.Equal(t, 0, doc.Find(".breakdown-item").Length())
	assert.Equal(t, 1, doc.Find(".breakdown .side-empty").Length())
}

func TestListTabKeepsViewedMonth(t *testing.T) {
	for _, section := range []string{"house", "car", "groceries"} {
		t.Run(section, func(t *testing.T) {
			doc := parseHTML(t, render(t, section+"-section-buttons", map[string]any{"Lang": "en"}))
			list := doc.Find(`.section-button[hx-get="/` + section + `/current"]`)
			require.Equal(t, 1, list.Length())
			vals, _ := list.Attr("hx-vals")
			assert.Contains(t, vals, "js:", "month is read when clicked")
			assert.Contains(t, vals, `get("month")`, "from the page URL")
		})
	}
}
