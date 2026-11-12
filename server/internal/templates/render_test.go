package templates

import (
	"bytes"
	"expenser/internal/chart"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"html/template"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
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
		parsed, parseErr = utilities.ParseTemplates(filepath.Join(".", "**", "*.html"))
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
	assert.Equal(t, 4, nav.Find(".tracker-nav-button").Length(), "house, car, groceries, bills")
	bills, _ := nav.Find(`.tracker-nav-button[data-path="/bills"]`).Attr("hx-get")
	assert.Equal(t, "/bills", bills)
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
			{Month: "2026-08", MonthNum: time.August}, {Month: "2026-09", MonthNum: time.September, Height: 50, Total: 625},
			{Month: "2026-10", MonthNum: time.October, Height: 100, Total: 1250}, {Month: "2026-11", MonthNum: time.November, Height: 80, Total: 1000, Current: true},
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
	toolbar := doc.Find(".month-toolbar button.upload-btn")
	require.Equal(t, 1, toolbar.Length(), "desktop: next to Add in the toolbar")
	hx, _ := toolbar.Attr("hx-get")
	assert.Equal(t, "/groceries/form", hx)
	assert.Equal(t, 1, toolbar.Find("svg").Length(), "has an icon")
	assert.Equal(t, 1, doc.Find("#add-expense-section button.upload-btn").Length(), "phone: full-width button")

	house := parseHTML(t, render(t, "house-current", monthView("house")))
	assert.Equal(t, 0, house.Find("button.upload-btn").Length(), "only groceries upload receipts")
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

	values := doc.Find(".mini-chart .mini-bar-value")
	require.Equal(t, 6, values.Length(), "one rough value per bar")
	assert.Empty(t, strings.TrimSpace(values.Eq(0).Text()), "empty months stay unlabelled")
	assert.Equal(t, "625", values.Eq(3).Text())
	assert.Equal(t, "1.3k", values.Eq(4).Text())
	assert.Equal(t, "1k", values.Eq(5).Text())
	assert.True(t, bars.Eq(5).HasClass("current"), "current value is styled through its bar")
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
	v.Summary = models.MonthSummary{Total: 183.63}
	v.Breakdown = []models.TypeShare{
		{Type: "Gas", Color: 3, Amount: 85.01, Share: 46.3, Usual: 72, HasUsual: true, VsUsual: 18.1},
		{Type: "Water", Color: 2, Amount: 19.19, Share: 10.4, Usual: 20, HasUsual: true, VsUsual: -4},
		{Type: "TV", Amount: 9.99, Share: 5.4}, // Color 0: unknown type
	}
	v.YearSoFar = models.YearSummary{Total: 1834.5, Months: 11, Average: 166.77, VsAverage: 10.1, HasComparison: true}
	v.BarsTotal = 912.4
	doc := parseHTML(t, render(t, "house-current", v))

	// Every panel has an icon and a title, like the KPI tiles.
	panels := doc.Find(".month-side .side-panel")
	panels.Each(func(i int, p *goquery.Selection) {
		assert.Equal(t, 1, p.Find(".side-panel-head svg").Length(), "panel %d has an icon", i)
		assert.NotEmpty(t, strings.TrimSpace(p.Find(".side-panel-head h3").Text()))
	})

	bd := doc.Find(".breakdown")
	segments := bd.Find(".stacked-bar span")
	require.Equal(t, 3, segments.Length(), "one segment per type")
	style, _ := segments.Eq(0).Attr("style")
	assert.Contains(t, style, "width: 46.3%")
	assert.True(t, segments.Eq(0).HasClass("cat-3"))
	assert.True(t, segments.Eq(1).HasClass("cat-2"))
	assert.True(t, segments.Eq(2).HasClass("cat-0"))

	items := bd.Find(".breakdown-item")
	require.Equal(t, 3, items.Length())
	assert.Equal(t, 1, items.Eq(0).Find(".cat-dot.cat-3").Length(), "dot matches its segment")
	assert.Contains(t, items.Eq(0).Text(), "85.01 €")
	gas := items.Eq(0).Find(".pill")
	assert.Contains(t, gas.Text(), "18%")
	assert.True(t, gas.HasClass("up"), "well above usual")
	title, _ := gas.Attr("title")
	assert.Contains(t, title, "72.00 €", "tooltip explains the usual amount")
	assert.True(t, items.Eq(1).Find(".pill").HasClass("same"), "within 10% reads as usual")
	assert.Contains(t, items.Eq(2).Find(".pill").Text(), "New")

	year := doc.Find(".year-so-far")
	assert.Equal(t, "2026 so far", strings.TrimSpace(year.Find(".side-panel-head h3").Text()))
	assert.Contains(t, year.Find(".side-big").Text(), "1834.50 €")
	assert.Contains(t, year.Text(), "166.77 €")
	assert.Contains(t, year.Find(".pill").Text(), "10%")

	assert.Equal(t, 0, doc.Find(".estimate").Length(), "no estimate unless HasEstimate")
	v.HasEstimate, v.Estimate = true, 250
	doc = parseHTML(t, render(t, "house-current", v))
	est := doc.Find(".estimate")
	assert.Contains(t, est.Find(".side-big").Text(), "250.00 €")
	fill, _ := est.Find(".progress span").Attr("style")
	assert.Contains(t, fill, "width: 73.5%", "spent so far out of the estimate")
	assert.Contains(t, est.Text(), "183.63 €")
	assert.Contains(t, est.Find(".side-foot").Text(), "Bills still due plus your usual pace")

	v.EstimateFromPace = true
	doc = parseHTML(t, render(t, "house-current", v))
	assert.Contains(t, doc.Find(".estimate .side-foot").Text(), "Based on your daily pace so far")

	assert.Contains(t, doc.Find(".mini-chart .side-panel-head").Text(), "912.40 €", "chart header shows the 6-month total")
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

func TestConfirmModalLabel(t *testing.T) {
	data := &models.ModalConfirmContent{Title: "Delete?", Method: "DELETE", Endpoint: "/house/expenses/2", Target: "#exp-2", Message: "Water", Lang: "en"}
	doc := parseHTML(t, render(t, "confirm-modal", data))
	assert.Equal(t, "Ok", strings.TrimSpace(doc.Find("#confirm-btn").Text()), "default label")

	data.ConfirmLabel = "Delete"
	doc = parseHTML(t, render(t, "confirm-modal", data))
	assert.Equal(t, "Delete", strings.TrimSpace(doc.Find("#confirm-btn").Text()))
}

func TestToasts(t *testing.T) {
	cases := map[string]string{"success-modal": "success", "error-modal": "error", "modal": "info"}
	for tmpl, kind := range cases {
		t.Run(tmpl, func(t *testing.T) {
			doc := parseHTML(t, render(t, tmpl, &models.ModalContent{Title: "Saved", Message: "Gas updated", Lang: "en"}))
			toast := doc.Find("dialog#modal .toast")
			require.Equal(t, 1, toast.Length())
			assert.True(t, toast.HasClass(kind))
			assert.Equal(t, 1, toast.Find(".toast-icon svg").Length())
			assert.Equal(t, "Saved", strings.TrimSpace(toast.Find(".toast-title").Text()))
			assert.Equal(t, "Gas updated", strings.TrimSpace(toast.Find(".toast-message").Text()))
			closeBtn := toast.Find("button#confirm-btn.toast-close")
			require.Equal(t, 1, closeBtn.Length(), "keeps the id main.js focuses")
			label, _ := closeBtn.Attr("aria-label")
			assert.Equal(t, "Close", label)
			assert.Equal(t, 1, toast.Find("progress#countdown-progress").Length(), "auto-dismiss countdown")
		})
	}
}

func TestReceiptUploadSheet(t *testing.T) {
	doc := parseHTML(t, render(t, "groceries-receipt-form", map[string]any{"Lang": "en"}))
	require.Equal(t, 1, doc.Find(".sheet .sheet-grab").Length(), "same panel as add/edit")
	assert.Contains(t, doc.Find(".new-expense-heading").Text(), "Upload Receipt")

	form := doc.Find("form.receipt-upload-form")
	target, _ := form.Attr("hx-target")
	assert.Equal(t, "#action-dialog", target, "the review replaces the upload step")
	indicator, _ := form.Attr("hx-indicator")
	assert.Equal(t, "#receipt-reading", indicator)
	assert.Equal(t, 1, doc.Find("#receipt-reading").Length(), "reading state while the AI works")

	drop := doc.Find("label.dropzone")
	require.Equal(t, 1, drop.Length())
	input := drop.Find(`input[type="file"][name="receipt"]`)
	require.Equal(t, 1, input.Length(), "the whole drop zone is the file picker")
	accept, _ := input.Attr("accept")
	assert.Equal(t, "image/*,.pdf", accept)

	assert.Equal(t, 1, doc.Find(".sheet-actions .sheet-save").Length())
	assert.Equal(t, 1, doc.Find(".sheet-actions .sheet-cancel").Length())
}

func TestReceiptReviewSheet(t *testing.T) {
	items := []models.ReceiptItem{
		{Name: "Chicken breast", Quantity: 0.8, UnitPrice: 9.9, TotalPrice: 7.92, TotalDiscount: 1.2, Category: "Meat"},
		{Name: "Milk", Quantity: 2, UnitPrice: 1.45, TotalPrice: 2.9, Category: ""},
	}
	cats := []models.Category{{ID: 1, Name: "Meat"}, {ID: 2, Name: "Dairy"}, {ID: 9, Name: "Other"}}
	doc := parseHTML(t, render(t, "groceries-receipt-edit-form", map[string]any{
		"Store": "Lidl", "Items": items, "Categories": cats, "Subtotal": 10.82, "Discount": 1.2, "Total": 9.62, "Lang": "en",
		"PurchasedAt": time.Date(2026, 11, 10, 14, 5, 0, 0, time.UTC),
	}))

	when, _ := doc.Find(`input[name="purchased_at"]`).Attr("value")
	assert.Equal(t, "2026-11-10T14:05", when, "the date printed on the receipt, editable before saving")
	kind, _ := doc.Find(`input[name="purchased_at"]`).Attr("type")
	assert.Equal(t, "datetime-local", kind)

	head := doc.Find(".new-expense-heading")
	assert.Contains(t, head.Text(), "Lidl")
	assert.Contains(t, doc.Find(".receipt-summary").Text(), "2 items")
	assert.Contains(t, doc.Find(".receipt-summary").Text(), "9.62 €")
	totals := doc.Find(".receipt-totals")
	require.Equal(t, 1, totals.Length(), "totals shown before saving")
	assert.Contains(t, totals.Find(".receipt-subtotal").Text(), "10.82 €")
	assert.Contains(t, totals.Find(".receipt-discount").Text(), "-1.20 €")
	assert.Contains(t, totals.Find(".receipt-total").Text(), "9.62 €")

	noDiscount := parseHTML(t, render(t, "groceries-receipt-edit-form", map[string]any{
		"Items": items, "Categories": cats, "Subtotal": 10.82, "Total": 10.82, "Lang": "en",
	}))
	assert.Equal(t, 0, noDiscount.Find(".receipt-discount").Length(), "no discount row without discounts")

	cards := doc.Find(".receipt-item")
	require.Equal(t, 2, cards.Length(), "one card per item instead of a wide table")
	assert.Equal(t, 0, doc.Find("table").Length())
	product, _ := cards.Eq(0).Find(`input[name="items[0].product"]`).Attr("value")
	assert.Equal(t, "Chicken breast", product)
	sel, _ := cards.Eq(0).Find(`select[name="items[0].category_id"] option[selected]`).Attr("value")
	assert.Equal(t, "1", sel)
	other, _ := cards.Eq(1).Find(`select[name="items[1].category_id"] option[selected]`).Attr("value")
	assert.Equal(t, "9", other, "unknown category falls back to Other")
	for _, f := range []string{"quantity", "unit_price", "total_price", "discount_per_unit", "total_discount", "supermarket_name"} {
		assert.Equal(t, 1, cards.Eq(1).Find(`[name="items[1].`+f+`"]`).Length(), f)
	}

	form := doc.Find("form")
	post, _ := form.Attr("hx-post")
	assert.Equal(t, "/groceries/confirm-batch", post)
	assert.Contains(t, doc.Find(".sheet-save").Text(), "Save 2 items")
	assert.NotContains(t, doc.Text(), "Confirm & Save All", "no hard-coded English")
}

func sampleBill(name string, due time.Time, active bool) models.RecurringPayment {
	amount := 29.9
	typeID := 4
	return models.RecurringPayment{
		ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Name: name, Category: models.SectionHouse,
		UtilityTypeID: &typeID, TypeName: "Internet", DueDate: due, StartDate: due, CycleUnit: models.CycleMonth,
		CycleEvery: 1, ExpectedAmount: &amount, Active: active, PaymentOptions: []string{"cash", "card"},
		UpdatedAt: time.Date(2026, 11, 11, 9, 0, 0, 123456000, time.UTC),
	}
}

func TestBillsView(t *testing.T) {
	overdue := sampleBill("Fiber", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), true)
	soon := sampleBill("", time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), true)
	soon.ID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	soon.CycleEvery = 6
	v := &models.BillsView{
		Overdue:      []models.BillRow{{Payment: overdue, DaysLeft: -71, Missed: true}},
		DueSoon:      []models.BillRow{{Payment: soon, DaysLeft: 3}},
		OverdueCount: 1, DueThisMonth: 59.8, Total: 2, Lang: "en",
	}
	v.Next = &v.DueSoon[0]
	doc := parseHTML(t, render(t, "bills-view", v))

	wrapper := doc.Find("#bills-view")
	trig, _ := wrapper.Attr("hx-trigger")
	assert.Equal(t, "bills-changed from:body", trig)
	dis, _ := wrapper.Attr("hx-disinherit")
	assert.Equal(t, "*", dis)
	add, _ := doc.Find(".month-toolbar .add-btn").Attr("hx-get")
	assert.Equal(t, "/bills/new", add)
	fab, _ := doc.Find("button.fab").Attr("hx-get")
	assert.Equal(t, "/bills/new", fab)

	assert.Contains(t, doc.Find(".kpi-total").Text(), "59.80 €")
	assert.Contains(t, doc.Find(".kpi-overdue").Text(), "1")

	groups := doc.Find(".bill-group")
	require.Equal(t, 2, groups.Length(), "empty groups are not shown")
	assert.Contains(t, groups.Eq(0).Find("h3").Text(), "Overdue")

	row := doc.Find("#bill-11111111-1111-1111-1111-111111111111")
	require.Equal(t, 1, row.Length())
	assert.Contains(t, row.Find(".bill-name").Text(), "Fiber")
	assert.Contains(t, row.Find(".bill-meta").Text(), "Internet")
	assert.Contains(t, row.Find(".bill-meta").Text(), "Monthly")
	assert.Contains(t, row.Find(".bill-due").Text(), "71 days overdue")
	assert.Contains(t, row.Find(".bill-amount").Text(), "29.90 €")
	edit, _ := row.Attr("hx-get")
	assert.Equal(t, "/bills/edit/11111111-1111-1111-1111-111111111111", edit)
	pay, _ := row.Find("button.pay-btn").Attr("hx-get")
	assert.Equal(t, "/bills/pay/11111111-1111-1111-1111-111111111111", pay)
	skip, _ := row.Find("button.skip-btn").Attr("hx-post")
	assert.Equal(t, "/bills/11111111-1111-1111-1111-111111111111/skip", skip)

	soonRow := doc.Find("#bill-22222222-2222-2222-2222-222222222222")
	assert.Contains(t, soonRow.Find(".bill-name").Text(), "Internet", "falls back to the type name")
	assert.Contains(t, soonRow.Find(".bill-meta").Text(), "Every 6 months")
	assert.Contains(t, soonRow.Find(".bill-due").Text(), "in 3 days")
	assert.Equal(t, 0, soonRow.Find("button.skip-btn").Length(), "only bills several cycles behind can be skipped")
}

func TestBillsViewEmpty(t *testing.T) {
	doc := parseHTML(t, render(t, "bills-view", &models.BillsView{Lang: "en"}))
	assert.Equal(t, 1, doc.Find(".bills-empty").Length())
	assert.Equal(t, 0, doc.Find(".bill-group").Length())
}

func TestBillFormEdit(t *testing.T) {
	b := sampleBill("Fiber", time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), true)
	b.PaymentOptions = []string{"cash", "ePay"}
	doc := parseHTML(t, render(t, "bill-form", map[string]any{
		"Bill": &b, "IsEdit": true, "Category": "house", "TypeID": 4, "DueDate": "2026-12-01",
		"Version": b.UpdatedAt.Format(time.RFC3339Nano),
		"Types":   []models.TypeOption{{ID: 4, Name: "Internet"}, {ID: 5, Name: "TV"}},
		"Options": map[string]bool{"cash": true}, "OtherOption": "ePay",
		"KnownOptions": []string{"cash", "card", "bank_transfer", "online"}, "Lang": "en",
	}))
	form := doc.Find("form.bill-form")
	put, _ := form.Attr("hx-put")
	assert.Equal(t, "/bills/11111111-1111-1111-1111-111111111111", put)
	ver, _ := form.Find(`input[name="version"]`).Attr("value")
	assert.Equal(t, "2026-11-11T09:00:00.123456Z", ver, "stale edits are detected")

	cat, _ := form.Find(`input[name="category"][checked]`).Attr("value")
	assert.Equal(t, "house", cat)
	switcher, _ := form.Find("fieldset.category-switch").Attr("hx-get")
	assert.Equal(t, "/bills/types", switcher)
	typ, _ := form.Find(`#bill-type-chips input[name="typeID"][checked]`).Attr("value")
	assert.Equal(t, "4", typ)
	_, hasMeta := form.Find("#bill-type-chips fieldset").Attr("hx-get")
	assert.False(t, hasMeta, "bills have no per-type detail fields")

	amount, _ := form.Find(`input[name="expected_amount"]`).Attr("value")
	assert.Equal(t, "29.9", amount)
	unit, _ := form.Find(`select[name="cycle_unit"] option[selected]`).Attr("value")
	assert.Equal(t, "month", unit)
	assert.Equal(t, 1, form.Find(`input[name="payment_options"][value="cash"][checked]`).Length())
	assert.Equal(t, 0, form.Find(`input[name="payment_options"][value="card"][checked]`).Length())
	other, _ := form.Find(`input[name="payment_option_other"]`).Attr("value")
	assert.Equal(t, "ePay", other)
	assert.Equal(t, 1, form.Find(`input[name="paused"]`).Length(), "pause/resume when editing")
	del, _ := form.Find(".sheet-delete").Attr("hx-get")
	assert.Equal(t, "/bills/delete/11111111-1111-1111-1111-111111111111", del)
}

func TestBillFormCreate(t *testing.T) {
	b := models.RecurringPayment{Category: "house", CycleUnit: models.CycleMonth, CycleEvery: 1, RemindDaysBefore: 3, PaymentOptions: []string{"cash"}, Active: true}
	doc := parseHTML(t, render(t, "bill-form", map[string]any{
		"Bill": &b, "IsEdit": false, "Category": "house", "DueDate": "2026-12-11",
		"Types": []models.TypeOption{{ID: 4, Name: "Internet"}}, "Options": map[string]bool{"cash": true},
		"KnownOptions": []string{"cash", "card", "bank_transfer", "online"}, "Lang": "en",
	}))
	post, _ := doc.Find("form.bill-form").Attr("hx-post")
	assert.Equal(t, "/bills", post)
	assert.Equal(t, 0, doc.Find(`input[name="version"]`).Length())
	assert.Equal(t, 0, doc.Find(`input[name="paused"]`).Length())
	assert.Equal(t, 0, doc.Find(".sheet-delete").Length())
	assert.Equal(t, 1, doc.Find(`input[name="payment_options"][value="cash"][checked]`).Length(), "cash pre-selected")
}

func TestBillPayForm(t *testing.T) {
	b := sampleBill("Fiber", time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), true)
	doc := parseHTML(t, render(t, "bill-pay-form", map[string]any{"Bill": &b, "Label": "Fiber", "Today": "2026-11-11", "Lang": "en"}))
	form := doc.Find("form")
	post, _ := form.Attr("hx-post")
	assert.Equal(t, "/bills/11111111-1111-1111-1111-111111111111/paid", post)
	amount, _ := form.Find(`input[name="amount"]`).Attr("value")
	assert.Equal(t, "29.9", amount, "pre-filled with the expected amount")
	min, _ := form.Find(`input[name="amount"]`).Attr("min")
	assert.Equal(t, "0", min, "a free month is allowed")
	paid, _ := form.Find(`input[name="paid_on"]`).Attr("value")
	assert.Equal(t, "2026-11-11", paid)
	assert.Contains(t, doc.Find(".new-expense-heading").Text(), "Fiber")
}

func TestUpcomingBillsPanel(t *testing.T) {
	v := monthView("house")
	b := sampleBill("Fiber", time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), true)
	v.UpcomingBills = []models.UpcomingPayment{{Payment: b, DaysLeft: 3}}
	doc := parseHTML(t, render(t, "house-current", v))
	panel := doc.Find(".month-side .upcoming-bills")
	require.Equal(t, 1, panel.Length())
	assert.Contains(t, panel.Text(), "Fiber")
	assert.Contains(t, panel.Text(), "in 3 days")
	pay, _ := panel.Find("button.pay-btn").Attr("hx-get")
	assert.Equal(t, "/bills/pay/11111111-1111-1111-1111-111111111111", pay)
	all, _ := panel.Find("a.all-bills").Attr("hx-get")
	assert.Equal(t, "/bills", all)

	doc = parseHTML(t, render(t, "house-current", monthView("house")))
	assert.Equal(t, 0, doc.Find(".upcoming-bills").Length(), "hidden when nothing is coming up")
}

func TestBillFormReminderFields(t *testing.T) {
	b := sampleBill("Gas", time.Date(2026, 12, 8, 0, 0, 0, 0, time.UTC), true)
	b.DateKind = models.DateArrives
	b.RemindDaysAfter = 1
	b.RemindDaysBefore = 3
	every, max := 3, 5
	b.RepeatEveryDays, b.MaxReminders = &every, &max
	snooze := time.Date(2026, 12, 12, 0, 0, 0, 0, time.UTC)
	b.SnoozedUntil = &snooze
	doc := parseHTML(t, render(t, "bill-form", map[string]any{
		"Bill": &b, "IsEdit": true, "Category": "house", "TypeID": 3, "DueDate": "2026-12-08",
		"Types": []models.TypeOption{{ID: 3, Name: "Gas"}}, "Options": map[string]bool{},
		"KnownOptions": []string{"cash"}, "Lang": "en",
	}))
	form := doc.Find("form.bill-form")
	kind, _ := form.Find(`input[name="date_kind"][checked]`).Attr("value")
	assert.Equal(t, "arrives", kind)
	assert.Equal(t, 2, form.Find(`input[name="date_kind"]`).Length(), "pay by / arrives around")
	before, _ := form.Find(`input[name="remind_days_before"]`).Attr("value")
	assert.Equal(t, "3", before, "kept for when switching back to pay-by")
	after, _ := form.Find(`input[name="remind_days_after"]`).Attr("value")
	assert.Equal(t, "1", after)
	assert.Equal(t, 1, form.Find(`input[name="keep_reminding"][checked]`).Length())
	repeat, _ := form.Find(`input[name="repeat_every_days"]`).Attr("value")
	assert.Equal(t, "3", repeat)
	cap, _ := form.Find(`input[name="max_reminders"]`).Attr("value")
	assert.Equal(t, "5", cap)
	sn, _ := form.Find(`input[name="snoozed_until"]`).Attr("value")
	assert.Equal(t, "2026-12-12", sn)
}

func TestBillFormNewHasNoSnooze(t *testing.T) {
	every := 3
	b := models.RecurringPayment{Category: "house", DateKind: models.DatePayBy, CycleUnit: models.CycleMonth, CycleEvery: 1, RemindDaysBefore: 3, RepeatEveryDays: &every, Active: true}
	doc := parseHTML(t, render(t, "bill-form", map[string]any{
		"Bill": &b, "IsEdit": false, "Category": "house", "DueDate": "2026-12-11",
		"Types": []models.TypeOption{{ID: 4, Name: "Internet"}}, "Options": map[string]bool{},
		"KnownOptions": []string{"cash"}, "Lang": "en",
	}))
	assert.Equal(t, 0, doc.Find(`input[name="snoozed_until"]`).Length(), "nothing to snooze yet")
	kind, _ := doc.Find(`input[name="date_kind"][checked]`).Attr("value")
	assert.Equal(t, "pay_by", kind)
	assert.Equal(t, 1, doc.Find(`input[name="keep_reminding"][checked]`).Length(), "keeps reminding by default")
	max, _ := doc.Find(`input[name="max_reminders"]`).Attr("value")
	assert.Empty(t, max, "no limit by default")
}

func TestBillsViewWaiting(t *testing.T) {
	gas := sampleBill("Gas", time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC), true)
	gas.DateKind = models.DateArrives
	soon := sampleBill("Gas next", time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), true)
	soon.ID = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	soon.DateKind = models.DateArrives
	v := &models.BillsView{
		Waiting:      []models.BillRow{{Payment: gas, DaysLeft: -3}},
		DueSoon:      []models.BillRow{{Payment: soon, DaysLeft: 3}},
		WaitingCount: 1, Total: 2, Lang: "en",
	}
	doc := parseHTML(t, render(t, "bills-view", v))
	group := doc.Find(".bill-group.waiting")
	require.Equal(t, 1, group.Length())
	assert.Contains(t, group.Find("h3").Text(), "Waiting")
	assert.Contains(t, group.Find(".bill-due").Text(), "waiting 3 days")
	assert.Contains(t, doc.Find("#bill-33333333-3333-3333-3333-333333333333 .bill-due").Text(), "arrives in 3 days")
}

func TestSettingsHasAccount(t *testing.T) {
	doc := parseHTML(t, render(t, "header", models.HeaderOptions{IsLoggedIn: true, Lang: "en"}))
	btn := doc.Find("details.settings-menu .settings-account")
	require.Equal(t, 1, btn.Length())
	hx, _ := btn.Attr("hx-get")
	assert.Equal(t, "/settings/account", hx)
	assert.Contains(t, btn.Text(), "Account")

	out := parseHTML(t, render(t, "header", models.HeaderOptions{Lang: "en"}))
	assert.Equal(t, 0, out.Find(".settings-account").Length(), "only when logged in")
}

func accountUser() *models.User {
	two, five := 2, 5
	return &models.User{
		Username: "dev", DisplayName: "Miro", Email: "me@example.com",
		ReminderDefaults: models.ReminderDefaults{Notify: models.NotifyNone, RemindDaysBefore: 7, RemindDaysAfter: 1, RepeatEveryDays: &two, MaxReminders: &five},
	}
}

func TestAccountForm(t *testing.T) {
	doc := parseHTML(t, render(t, "account-form", map[string]any{"User": accountUser(), "Lang": "en"}))
	form := doc.Find("form.account-form")
	require.Equal(t, 1, form.Length())
	post, _ := form.Attr("hx-post")
	assert.Equal(t, "/settings/account", post)

	val := func(sel string) string { v, _ := form.Find(sel).Attr("value"); return v }
	assert.Equal(t, "Miro", val(`input[name="name"]`))
	ph, _ := form.Find(`input[name="name"]`).Attr("placeholder")
	assert.Contains(t, ph, "dev", "empty name falls back to the username")
	_, required := form.Find(`input[name="name"]`).Attr("required")
	assert.False(t, required)
	ml, _ := form.Find(`input[name="name"]`).Attr("maxlength")
	assert.Equal(t, "60", ml)
	assert.Equal(t, "me@example.com", val(`input[type="email"][name="email"]`))

	assert.Equal(t, "none", val(`input[name="notify"][checked]`))
	assert.Equal(t, "7", val(`input[name="remind_days_before"]`))
	assert.Equal(t, "1", val(`input[name="remind_days_after"]`))
	assert.Equal(t, 1, form.Find(`input[name="keep_reminding"][checked]`).Length())
	assert.Equal(t, "2", val(`input[name="repeat_every_days"]`))
	assert.Equal(t, "5", val(`input[name="max_reminders"]`))
	assert.Equal(t, 0, form.Find(`input[name="date_kind"]`).Length(), "defaults cover both kinds; no switch")
	assert.Contains(t, form.Text(), "Defaults for new bills")
}

func TestAccountFormEmptyDefaults(t *testing.T) {
	u := &models.User{Username: "dev", ReminderDefaults: models.DefaultReminderDefaults()}
	u.ReminderDefaults.RepeatEveryDays = nil
	doc := parseHTML(t, render(t, "account-form", map[string]any{"User": u, "Lang": "en"}))
	assert.Equal(t, 0, doc.Find(`input[name="keep_reminding"][checked]`).Length())
	v, _ := doc.Find(`input[name="repeat_every_days"]`).Attr("value")
	assert.Equal(t, "3", v, "a sensible number ready when switched on")
	v, _ = doc.Find(`input[name="max_reminders"]`).Attr("value")
	assert.Empty(t, v, "no limit")
	assert.Equal(t, "email", func() string { v, _ := doc.Find(`input[name="notify"][checked]`).Attr("value"); return v }())
}

func TestAccountFormPasswordSection(t *testing.T) {
	doc := parseHTML(t, render(t, "account-form", map[string]any{"User": accountUser(), "Lang": "en"}))
	sec := doc.Find("details.password-section")
	require.Equal(t, 1, sec.Length())
	_, open := sec.Attr("open")
	assert.False(t, open, "folded by default")

	for name, auto := range map[string]string{"current_password": "current-password", "new_password": "new-password", "confirm_password": "new-password"} {
		in := sec.Find(`input[type="password"][name="` + name + `"]`)
		require.Equal(t, 1, in.Length(), name)
		ac, _ := in.Attr("autocomplete")
		assert.Equal(t, auto, ac, name)
		_, req := in.Attr("required")
		assert.False(t, req, "must not block saving the rest of the panel")
	}
	btn := sec.Find(`button[hx-post="/settings/account/password"]`)
	require.Equal(t, 1, btn.Length())
	typ, _ := btn.Attr("type")
	assert.Equal(t, "button", typ, "its own action, not the panel's Save")
	inc, _ := btn.Attr("hx-include")
	assert.Equal(t, ".password-section input", inc)
}

func TestAccountFormTestButtonOnlyInDevMode(t *testing.T) {
	doc := parseHTML(t, render(t, "account-form", map[string]any{"User": accountUser(), "Lang": "en"}))
	assert.Equal(t, 0, doc.Find(`[hx-post="/settings/account/test-email"]`).Length(), "hidden outside dev mode")

	doc = parseHTML(t, render(t, "account-form", map[string]any{"User": accountUser(), "Lang": "en", "DevMode": true}))
	btn := doc.Find(`button[hx-post="/settings/account/test-email"]`)
	require.Equal(t, 1, btn.Length())
	typ, _ := btn.Attr("type")
	assert.Equal(t, "button", typ, "must not submit the save form")
	inc, _ := btn.Attr("hx-include")
	assert.Equal(t, "#account-email", inc, "tests the address typed, saved or not")
}

func TestBillFormDelivery(t *testing.T) {
	b := sampleBill("Fiber", time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), true)
	b.Notify = models.NotifyEmail
	b.NotifyEmail = "partner@example.com"
	doc := parseHTML(t, render(t, "bill-form", map[string]any{
		"Bill": &b, "IsEdit": true, "Category": "house", "TypeID": 4, "DueDate": "2026-12-01",
		"Types": []models.TypeOption{{ID: 4, Name: "Internet"}}, "Options": map[string]bool{},
		"KnownOptions": []string{"cash"}, "UserEmail": "me@example.com", "Lang": "en",
	}))
	notify, _ := doc.Find(`input[name="notify"][checked]`).Attr("value")
	assert.Equal(t, "email", notify)
	addr := doc.Find(`input[name="notify_email"]`)
	val, _ := addr.Attr("value")
	assert.Equal(t, "partner@example.com", val)
	ph, _ := addr.Attr("placeholder")
	assert.Equal(t, "me@example.com", ph, "shows where it goes by default")
}

func TestBillRowNoEmailHint(t *testing.T) {
	b := sampleBill("Fiber", time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), true)
	b.Notify = models.NotifyEmail
	v := &models.BillsView{DueSoon: []models.BillRow{{Payment: b, DaysLeft: 3}}, Total: 1, Lang: "en"}
	doc := parseHTML(t, render(t, "bills-view", v))
	assert.Contains(t, doc.Find(".bill-row .no-email").Text(), "No email", "no address: no reminders")

	b.UserEmail = "me@example.com"
	v.DueSoon[0].Payment = b
	doc = parseHTML(t, render(t, "bills-view", v))
	assert.Equal(t, 0, doc.Find(".no-email").Length())
}

func TestLoginPageSplitLayout(t *testing.T) {
	doc := parseHTML(t, render(t, "login-page", map[string]any{"Lang": "en"}))
	split := doc.Find("section.auth-split")
	require.Equal(t, 1, split.Length())
	assert.Equal(t, 1, split.Find(".auth-brand").Length(), "brand panel")
	assert.GreaterOrEqual(t, split.Find(".auth-brand li").Length(), 3, "what the app does")

	form := split.Find(".auth-card form")
	post, _ := form.Attr("hx-post")
	assert.Equal(t, "/login", post)
	user := form.Find(`input[name="username"]`)
	ac, _ := user.Attr("autocomplete")
	assert.Equal(t, "username", ac)
	pw := form.Find(`input[type="password"][name="password"]`)
	ac, _ = pw.Attr("autocomplete")
	assert.Equal(t, "current-password", ac)
	assert.Equal(t, 1, form.Find(`button[type="submit"].auth-submit`).Length())
	link, _ := split.Find(".auth-switch a").Attr("hx-get")
	assert.Equal(t, "/register", link)
}

func TestRegisterPageSplitLayout(t *testing.T) {
	doc := parseHTML(t, render(t, "register-page", map[string]any{"Lang": "en"}))
	form := doc.Find("section.auth-split .auth-card form")
	post, _ := form.Attr("hx-post")
	assert.Equal(t, "/register", post)
	for name, auto := range map[string]string{"username": "username", "password": "new-password", "confirm_password": "new-password"} {
		ac, _ := form.Find(`input[name="` + name + `"]`).Attr("autocomplete")
		assert.Equal(t, auto, ac, name)
	}
	hint := form.Find("#password-match-feedback")
	require.Equal(t, 1, hint.Length())
	assert.True(t, hint.HasClass("field-error"))
	_, styled := hint.Attr("style")
	assert.False(t, styled, "no inline red text")
	link, _ := doc.Find(".auth-switch a").Attr("hx-get")
	assert.Equal(t, "/login", link)
	assert.NotContains(t, render(t, "register-page", map[string]any{"Lang": "en"}), "DOMContentLoaded",
		"the check must also work when the page is swapped in by HTMX")
}

func TestRegisterSuccessCard(t *testing.T) {
	doc := parseHTML(t, render(t, "register-success", map[string]any{"Lang": "en", "User": &models.User{Username: "newbie"}}))
	card := doc.Find(".auth-card.auth-done")
	require.Equal(t, 1, card.Length())
	assert.Contains(t, card.Text(), "Account created")
	href, _ := card.Find("a.auth-submit").Attr("href")
	assert.Equal(t, "/", href, "continue button")
	assert.Contains(t, card.Text(), "newbie")
}

func TestHomeLoggedOut(t *testing.T) {
	doc := parseHTML(t, render(t, "index-page", map[string]any{"Lang": "en"}))
	assert.Equal(t, 1, doc.Find("section.auth-split .auth-brand").Length(), "same brand panel as login")
	login, _ := doc.Find(`.home-actions [hx-get="/login"]`).Attr("hx-push-url")
	assert.Equal(t, "true", login)
	assert.Equal(t, 1, doc.Find(`.home-actions [hx-get="/register"]`).Length())
	assert.Equal(t, 0, doc.Find(".home-grid").Length())
	assert.NotContains(t, doc.Text(), "top left corner", "old hints are gone")
}

func homeView() *models.HomeView {
	bars := make([]models.MonthBar, 12)
	for i := range bars {
		bars[i] = models.MonthBar{MonthNum: time.Month((i+11)%12 + 1), Total: float64(i * 10), Height: float64(i * 8)}
	}
	bars[11].Current = true
	amount := 64.0
	elec := models.RecurringPayment{Name: "Electricity", TypeName: "Electricity", ExpectedAmount: &amount, Active: true}
	gas := models.RecurringPayment{TypeName: "Gas", DateKind: models.DateArrives, Active: true}
	return &models.HomeView{
		Name: "Miro", Lang: "en",
		House:     models.HomeCategory{Section: "house", YearTotal: 2840, LastYearTotal: 2535, HasLastYear: true, ChangePct: 12, ThisMonth: 212, TopName: "Electricity", TopPct: 41, Bars: bars},
		Car:       models.HomeCategory{Section: "car", YearTotal: 1190, LastYearTotal: 1295, HasLastYear: true, ChangePct: -8.1, ThisMonth: 96, TopName: "Fuel", TopPct: 63, Bars: bars},
		Groceries: models.HomeCategory{Section: "groceries", Empty: true, Bars: bars},
		Bills: []models.HomeBillRow{
			{Payment: elec, DaysLeft: 3, LastYear: 58, HasLastYear: true, LastYearMonth: time.November},
			{Payment: gas, DaysLeft: -4},
		},
		MoreBills: 3,
	}
}

func TestHomeLoggedIn(t *testing.T) {
	doc := parseHTML(t, render(t, "index-page", map[string]any{"Lang": "en", "IsLoggedIn": true, "Home": homeView()}))
	assert.Contains(t, doc.Find("h2").Text(), "Hi Miro")
	cards := doc.Find(".home-grid .home-card")
	require.Equal(t, 4, cards.Length())
	var paths []string
	cards.Each(func(_ int, s *goquery.Selection) { p, _ := s.Attr("hx-get"); paths = append(paths, p) })
	assert.Equal(t, []string{"/house", "/car", "/groceries", "/bills"}, paths, "each card opens its tab")
	assert.Equal(t, 0, doc.Find(".auth-split").Length())

	house := doc.Find(`.home-card[hx-get="/house"]`)
	assert.Contains(t, house.Find(".home-big").Text(), "2840.00")
	cmp := house.Find(".home-cmp")
	assert.True(t, cmp.HasClass("up"), "more than last year")
	assert.Contains(t, cmp.Text(), "12%")
	assert.Contains(t, cmp.Text(), "2535.00")
	assert.Contains(t, house.Text(), "212.00", "this month")
	assert.Contains(t, house.Text(), "Electricity · 41%")
	assert.Equal(t, 12, house.Find(".home-bars span").Length())
	assert.Equal(t, 1, house.Find(".home-bars span.current").Length())

	car := doc.Find(`.home-card[hx-get="/car"]`)
	assert.True(t, car.Find(".home-cmp").HasClass("down"))
	assert.Contains(t, car.Find(".home-cmp").Text(), "8%")

	groceries := doc.Find(`.home-card[hx-get="/groceries"]`)
	assert.Contains(t, groceries.Text(), "Nothing spent yet this year")
	assert.Equal(t, 0, groceries.Find(".home-bars").Length(), "no extras when empty")

	bills := doc.Find(`.home-card[hx-get="/bills"] .home-bill`)
	require.Equal(t, 2, bills.Length())
	first := bills.Eq(0).Text()
	assert.Contains(t, first, "Electricity")
	assert.Contains(t, first, "in 3 days")
	assert.Contains(t, first, "64.00")
	assert.Contains(t, first, "last Nov: 58.00")
	second := bills.Eq(1).Text()
	assert.Contains(t, second, "Gas", "no name: the type")
	assert.Contains(t, second, "waiting 4 days")
	assert.NotContains(t, second, "last ", "no last-year figure when there was none")
	assert.False(t, bills.Eq(1).HasClass("late"), "waiting is not overdue")
	assert.Contains(t, doc.Find(`.home-card[hx-get="/bills"]`).Text(), "+3 more")
	assert.Equal(t, "Upcoming bills", strings.TrimSpace(doc.Find(`.home-card[hx-get="/bills"] .home-card-title`).Text()), "the title says it; no extra label line")
}

func TestHomeLoggedInWithoutData(t *testing.T) {
	plain := parseHTML(t, render(t, "index-page", map[string]any{"Lang": "en", "IsLoggedIn": true}))
	assert.Equal(t, "Welcome back", strings.TrimSpace(plain.Find("h2").Text()), "no data: no empty name")
	assert.Equal(t, 0, plain.Find(".home-card").Length())
}

func TestHomeNoBills(t *testing.T) {
	v := homeView()
	v.Bills, v.MoreBills = nil, 0
	doc := parseHTML(t, render(t, "index-page", map[string]any{"Lang": "en", "IsLoggedIn": true, "Home": v}))
	card := doc.Find(`.home-card[hx-get="/bills"]`)
	assert.Contains(t, card.Text(), "No bills yet")
	assert.NotContains(t, card.Text(), "more")
}

func TestAddFormsHaveBackfill(t *testing.T) {
	for _, form := range []string{"create-house-exp-form", "create-car-exp-form"} {
		doc := parseHTML(t, render(t, form, map[string]any{"Lang": "en", "Date": "2026-11-11", "Month": "2026-11"}))
		require.Equal(t, 1, doc.Find(`.backfill input[type="checkbox"][name="backfill"]`).Length(), form)
		from := doc.Find(`input[type="month"][name="backfill_from"]`)
		max, _ := from.Attr("max")
		assert.Equal(t, "2026-11", max, form+": no future months")
		to, _ := doc.Find(`input[type="month"][name="backfill_to"]`).Attr("value")
		assert.Equal(t, "2026-11", to, form+": To starts at this month")
		after, _ := doc.Find("form").Attr("hx-on::after-request")
		assert.Contains(t, after, "HX-Retarget", form+": the preview keeps the panel open")
	}
}

func TestBackfillPreview(t *testing.T) {
	dates := []time.Time{time.Date(2024, 1, 20, 0, 0, 0, 0, time.Local), time.Date(2024, 2, 20, 0, 0, 0, 0, time.Local), time.Date(2024, 4, 20, 0, 0, 0, 0, time.Local)}
	plan := &services.BackfillPlan{
		Request: services.BackfillRequest{Day: 20, Expense: models.BackfillExpense{Amount: 15, Discount: 1}},
		Dates:   dates, Skipped: []services.YearMonth{{Year: 2024, Month: time.March}}, Months: 4,
	}
	values := url.Values{"typeID": {"4"}, "amount": {"15"}, "discount_amount": {"1"}, "notes": {"fiber"}, "date": {"2026-11-20"}, "backfill_from": {"2024-01"}, "backfill_to": {"2024-04"}}
	doc := parseHTML(t, render(t, "backfill-preview", map[string]any{
		"Plan": plan, "First": dates[0], "Last": dates[2], "TypeName": "Internet", "Section": "house", "Values": values, "Lang": "en",
	}))
	form := doc.Find("form.backfill-preview")
	post, _ := form.Attr("hx-post")
	assert.Equal(t, "/house/expenses/backfill", post)
	for k := range values {
		v, _ := form.Find(`input[type="hidden"][name="` + k + `"]`).Attr("value")
		assert.Equal(t, values.Get(k), v, k)
	}
	text := form.Text()
	assert.Contains(t, text, "Adds 3 payments")
	assert.Contains(t, text, "Internet · 15.00")
	assert.Contains(t, text, "Discount 1.00")
	assert.Contains(t, text, "Jan 2024 – Apr 2024, on day 20 of each month")
	assert.Contains(t, text, "Skipping 1 already recorded")
	assert.Contains(t, form.Find(".backfill-skipped").Text(), "Mar 2024")
	assert.Contains(t, form.Find("button[type=submit]").Text(), "Add 3 payments")

	empty := &services.BackfillPlan{Request: plan.Request, Skipped: plan.Skipped, Months: 1}
	none := parseHTML(t, render(t, "backfill-preview", map[string]any{"Plan": empty, "TypeName": "Internet", "Section": "car", "Values": values, "Lang": "en"}))
	assert.Contains(t, none.Text(), "Nothing to add")
	assert.Equal(t, 0, none.Find("button[type=submit]").Length(), "nothing to confirm")
}

func TestLoginKeepsNext(t *testing.T) {
	doc := parseHTML(t, render(t, "login-page", map[string]any{"Lang": "en", "Next": "/bills"}))
	v, _ := doc.Find(`form input[type="hidden"][name="next"]`).Attr("value")
	assert.Equal(t, "/bills", v)
	plain := parseHTML(t, render(t, "login-page", map[string]any{"Lang": "en"}))
	assert.Equal(t, 0, plain.Find(`input[name="next"]`).Length())
}

func TestSnoozePage(t *testing.T) {
	until := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	confirm := parseHTML(t, render(t, "snooze-page", map[string]any{"State": "confirm", "Bill": "Internet", "Until": until, "Token": "tok.en", "Lang": "en"}))
	form := confirm.Find("form")
	post, _ := form.Attr("hx-post")
	assert.Equal(t, "/snooze/tok.en", post)
	assert.Contains(t, confirm.Text(), "Internet")
	assert.Contains(t, confirm.Text(), "15.11.2026")
	assert.Equal(t, 1, form.Find("button[type=submit]").Length(), "a click on the email link changes nothing by itself")

	done := parseHTML(t, render(t, "snooze-page", map[string]any{"State": "done", "Bill": "Internet", "Until": until, "Lang": "en"}))
	assert.Contains(t, done.Text(), "Snoozed")
	assert.Equal(t, 0, done.Find("form").Length())

	for state, want := range map[string]string{"stale": "out of date", "expired": "expired", "invalid": "doesn't work"} {
		doc := parseHTML(t, render(t, "snooze-page", map[string]any{"State": state, "Lang": "en"}))
		assert.Contains(t, doc.Text(), want, state)
		assert.Equal(t, 0, doc.Find("form").Length(), state)
		assert.Equal(t, 1, doc.Find(`a[href="/bills"]`).Length(), state+": a way into the app")
	}
}

func TestLandingPage(t *testing.T) {
	doc := parseHTML(t, render(t, "index-page", map[string]any{"Lang": "en"}))
	land := doc.Find("section.landing")
	require.Equal(t, 1, land.Length(), "below the login panel for visitors")
	assert.Equal(t, 6, land.Find(".landing-feature").Length())
	assert.Equal(t, 3, land.Find(".landing-step").Length())
	assert.GreaterOrEqual(t, land.Find(".landing-stat").Length(), 3)
	preview := land.Find(".landing-preview")
	require.Equal(t, 1, preview.Length())
	_, hidden := preview.Find(".landing-preview-cards").Attr("aria-hidden")
	assert.True(t, hidden, "the sample dashboard is decoration")
	assert.Equal(t, 0, preview.Find("a, button").Length(), "nothing in the sample is clickable")
	cta := land.Find(".landing-cta")
	reg, _ := cta.Find(`[hx-get="/register"]`).Attr("hx-push-url")
	assert.Equal(t, "true", reg)
	assert.Equal(t, 1, cta.Find(`[hx-get="/login"]`).Length())

	logged := parseHTML(t, render(t, "index-page", map[string]any{"Lang": "en", "IsLoggedIn": true, "Home": homeView()}))
	assert.Equal(t, 0, logged.Find(".landing").Length(), "not for logged-in users")
}

func TestLandingPageTranslated(t *testing.T) {
	for _, lang := range []string{"en", "bg"} {
		out := render(t, "index-page", map[string]any{"Lang": lang})
		assert.NotContains(t, out, "landing.", lang+": every text has a translation")
	}
}

func TestRootLayoutRendersPagesByName(t *testing.T) {
	page := func(name string, content any) *goquery.Document {
		return parseHTML(t, render(t, "root", &models.RootLayout{
			TemplateName: name, TemplateContent: content, Lang: "en",
			HeaderOpts: &models.HeaderOptions{Lang: "en"},
		}))
	}
	login := page("login-page", map[string]any{"Lang": "en", "Next": "/bills"})
	assert.Equal(t, 1, login.Find(`#tracker-content form[hx-post="/login"]`).Length(), "any page by its name")
	v, _ := login.Find(`input[name="next"]`).Attr("value")
	assert.Equal(t, "/bills", v, "with its own data")

	snooze := page("snooze-page", map[string]any{"State": "invalid", "Lang": "en"})
	assert.Contains(t, snooze.Find("#tracker-content").Text(), "doesn't work")

	unknown := page("no-such-page", nil)
	assert.Equal(t, 1, unknown.Find("#tracker-content section.auth-split").Length(), "unknown names show the home page")
}

func TestEveryIconNameDraws(t *testing.T) {
	// Names used by the templates: navigation, home cards, side panels,
	// landing features, toasts and confirmation cards.
	for _, name := range []string{"house", "car", "groceries", "bills", "login", "register", "check", "alert", "moon",
		"receipt", "chart", "pie", "calendar", "target", "trend", "bell", "calendar-days", "history", "phone"} {
		doc := parseHTML(t, render(t, "icon", map[string]any{"Name": name}))
		svg := doc.Find("svg")
		require.Equal(t, 1, svg.Length(), name)
		assert.Greater(t, svg.Children().Length(), 0, name+": draws something")
		w, _ := svg.Attr("width")
		assert.Equal(t, "18", w, "default size")
	}
	big := parseHTML(t, render(t, "icon", map[string]any{"Name": "check", "Size": 28, "Stroke": "2.5"}))
	w, _ := big.Find("svg").Attr("width")
	sw, _ := big.Find("svg").Attr("stroke-width")
	assert.Equal(t, "28", w)
	assert.Equal(t, "2.5", sw)

	land := parseHTML(t, render(t, "index-page", map[string]any{"Lang": "en"}))
	land.Find(".landing-feature-icon svg").Each(func(i int, s *goquery.Selection) {
		assert.Greater(t, s.Children().Length(), 0, "landing feature %d has its icon", i)
	})
}

func TestReceiptJobPanel(t *testing.T) {
	reading := parseHTML(t, render(t, "groceries-receipt-job", map[string]any{"State": "reading", "ID": "abc123", "Lang": "en"}))
	poll := reading.Find(`[hx-get="/groceries/upload/abc123"]`)
	require.Equal(t, 1, poll.Length(), "checks back on its job")
	trigger, _ := poll.Attr("hx-trigger")
	assert.Equal(t, "every 2s", trigger)
	target, _ := poll.Attr("hx-target")
	assert.Equal(t, "#action-dialog", target, "the review form replaces the panel when ready")
	assert.Contains(t, reading.Text(), "close this panel", "explains it keeps going in the background")

	failed := parseHTML(t, render(t, "groceries-receipt-job", map[string]any{"State": "failed", "Message": "The AI said: “too small”", "Lang": "en"}))
	assert.Equal(t, 0, failed.Find("[hx-trigger]").Length(), "stops checking")
	assert.Contains(t, failed.Text(), "too small")
	again, _ := failed.Find(`button[hx-get^="/groceries/form"]`).Attr("hx-get")
	assert.Equal(t, "/groceries/form?new=1", again, "try again starts a fresh upload")
	assert.Equal(t, 1, failed.Find(`button[hx-get="/groceries/create"]`).Length(), "or add by hand")

	gone := parseHTML(t, render(t, "groceries-receipt-job", map[string]any{"State": "expired", "Lang": "en"}))
	assert.Contains(t, gone.Text(), "no longer available")
	assert.Equal(t, 0, gone.Find("[hx-trigger]").Length())
}

func TestSvgBarsRenders(t *testing.T) {
	b := chart.Stacked(chart.DefaultFrame, []string{"Jan", "Feb"}, []chart.Series{
		{Name: "Food", Color: 1, Values: []float64{100, 0}},
		{Name: "Fuel", Color: 2, Values: []float64{50, 80}},
	})
	b.Columns[1].Current = true
	b.SetAverage(90)
	html := render(t, "svg-bars", map[string]any{"Chart": b, "Lang": "en", "Unit": ""})
	doc := parseHTML(t, html)

	assert.Contains(t, html, `viewBox="0 0 640 260"`)
	assert.Equal(t, 3, doc.Find("rect.fill-cat-1, rect.fill-cat-2").Length())
	assert.Equal(t, 2, doc.Find("g.hit").Length())
	first := doc.Find("g.hit").First().Find("title").Text()
	assert.Contains(t, first, "Food")
	assert.Contains(t, first, "100.00")
	assert.Equal(t, 2, doc.Find("text.col-label").Length())
	assert.Equal(t, 1, doc.Find("text.col-label.current").Length())
	assert.Equal(t, len(b.Ticks), doc.Find("text.tick").Length())
	assert.Equal(t, 1, doc.Find("line.avg-line").Length())
	assert.Equal(t, 0, doc.Find(".trend-empty").Length())
}

func TestSvgBarsNoNaN(t *testing.T) {
	b := chart.Stacked(chart.DefaultFrame, []string{"Jan"}, nil)
	html := render(t, "svg-bars", map[string]any{"Chart": b, "Lang": "en", "Unit": ""})
	assert.NotContains(t, html, "NaN")
	assert.Contains(t, parseHTML(t, html).Find(".trend-empty").Text(), "Nothing to show")
}

func TestSvgPriceRenders(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	p := chart.Price(chart.DefaultFrame, []string{"Jan", "Feb", "Mar"},
		[]*float64{f(2), f(3), f(2.5)}, []*float64{f(1.5), f(2), f(2)},
		[]*float64{f(2.5), f(3.5), f(3)}, []*float64{f(2), nil, nil}, []float64{4, 0, 6})
	html := render(t, "svg-price", map[string]any{"Chart": p, "Lang": "en", "Unit": "kg"})
	doc := parseHTML(t, html)

	assert.Equal(t, 1, doc.Find("polyline.price-line").Length())
	assert.Equal(t, 1, doc.Find("polygon.price-band").Length())
	assert.Equal(t, 2, doc.Find("rect.qty-bar.fill-cat-3").Length())
	assert.Equal(t, 1, doc.Find("circle.low-marker").Length())
	assert.NotEmpty(t, doc.Find("circle.low-marker").Parent().Find("title").Text())
	assert.Contains(t, doc.Find("g.hit").First().Find("title").Text(), "/ kg")
	assert.NotContains(t, html, "NaN")
}

func TestSvgPriceEmpty(t *testing.T) {
	p := chart.Price(chart.DefaultFrame, []string{"Jan"}, nil, nil, nil, nil, nil)
	html := render(t, "svg-price", map[string]any{"Chart": p, "Lang": "en", "Unit": "kg"})
	assert.Equal(t, 1, parseHTML(t, html).Find(".trend-empty").Length())
	assert.NotContains(t, html, "NaN")
}

func TestTrendLegend(t *testing.T) {
	html := render(t, "trend-legend", map[string]any{"Lang": "en", "Items": []models.LegendItem{
		{Name: "Food", Color: 1}, {Name: "Average", Dashed: true},
	}})
	doc := parseHTML(t, html)
	assert.Equal(t, 2, doc.Find("li").Length())
	assert.Equal(t, 1, doc.Find(".legend-swatch.cat-1").Length())
	assert.Equal(t, 1, doc.Find(".legend-swatch.dashed").Length())
}

func TestSvgA11yAndHitTargets(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	p := chart.Price(chart.DefaultFrame, []string{"Jan", "Feb", "Mar"},
		[]*float64{f(2), f(3), f(2.5)}, []*float64{f(1.5), f(2), f(2)},
		[]*float64{f(2.5), f(3.5), f(3)}, []*float64{f(1.8), nil, f(2.2)}, []float64{4, 0, 6})
	doc := parseHTML(t, render(t, "svg-price", map[string]any{"Chart": p, "Lang": "en", "Unit": "kg"}))
	assert.Equal(t, 0, doc.Find(`svg[role="img"]`).Length())
	hits := doc.Find("g.hit")
	require.Equal(t, 3, hits.Length())
	hits.Each(func(_ int, g *goquery.Selection) {
		v, _ := g.Attr("tabindex")
		assert.Equal(t, "0", v)
		assert.Equal(t, 1, g.Find("circle.hit-area").Length())
	})
	assert.Contains(t, hits.Eq(0).Find("title").Text(), "(shelf: 1.80)")
	assert.NotContains(t, hits.Eq(1).Find("title").Text(), "shelf")
	assert.Contains(t, hits.Eq(2).Find("title").Text(), "(shelf: 2.20)")

	b := chart.Stacked(chart.DefaultFrame, []string{"Jan"}, []chart.Series{{Name: "Food", Color: 1, Values: []float64{5}}})
	bd := parseHTML(t, render(t, "svg-bars", map[string]any{"Chart": b, "Lang": "en", "Unit": ""}))
	assert.Equal(t, 0, bd.Find(`svg[role="img"]`).Length())
	v, _ := bd.Find("g.hit").Attr("tabindex")
	assert.Equal(t, "0", v)
}

func trendsMonthPage(section, prefix string) *models.TrendsPage {
	b := chart.Stacked(chart.DefaultFrame, []string{"Oct", "Nov"}, []chart.Series{
		{Name: "Gas", Color: 3, Values: []float64{40, 60}},
		{Name: "Everything else", Color: 0, Values: []float64{10, 0}},
	})
	b.Columns[1].Current = true
	b.SetAverage(55)
	return &models.TrendsPage{
		Section: section, Lang: "en", TypePrefix: prefix,
		Query: models.TrendQuery{Tab: "month", Period: "12", YearA: 2026, YearB: 2025, By: "month"},
		Types: []models.TypeOption{{ID: 3, Name: "Gas"}}, Years: []int{2026, 2024},
		Month: &models.MonthTrend{
			Total: 110, PrevTotal: 100, ChangePct: 10, HasChange: true, Median: 55,
			TopMonth: "Nov", TopMonthAmount: 60, TopMonthType: "Gas", Chart: b,
			Legend: []models.LegendItem{{Name: "Gas", Color: 3}, {Name: "Everything else", Color: 0}, {Name: "Monthly average", Dashed: true}},
		},
	}
}

func TestTrendsMonthTab(t *testing.T) {
	doc := parseHTML(t, render(t, "trends", trendsMonthPage("groceries", "category.")))
	tabs := doc.Find(".trend-tabs a")
	require.Equal(t, 3, tabs.Length(), "groceries: month, year, item")
	active := doc.Find(`.trend-tabs a[aria-current="page"]`)
	require.Equal(t, 1, active.Length())
	href, _ := active.Attr("hx-get")
	assert.Equal(t, "/groceries/chart?tab=month", href)
	for _, attr := range []string{"hx-target", "hx-push-url", "href"} {
		v, ok := tabs.Eq(1).Attr(attr)
		assert.True(t, ok, attr)
		assert.NotEmpty(t, v, attr)
	}
	target, _ := tabs.Eq(1).Attr("hx-target")
	assert.Equal(t, "#section-content", target)

	form := doc.Find("form#trends-filters")
	require.Equal(t, 1, form.Length())
	hx, _ := form.Attr("hx-get")
	assert.Equal(t, "/groceries/chart", hx)
	trigger, _ := form.Attr("hx-trigger")
	assert.Equal(t, "change", trigger)
	push, _ := form.Attr("hx-push-url")
	assert.Equal(t, "true", push)
	tab, _ := form.Find(`input[name="tab"]`).Attr("value")
	assert.Equal(t, "month", tab)
	sel, _ := form.Find(`select[name="period"] option[selected]`).Attr("value")
	assert.Equal(t, "12", sel)
	assert.Equal(t, 1, form.Find(`select[name="period"] option[value="y2024"]`).Length(), "calendar years with data")
	assert.Equal(t, 2, form.Find(`select[name="type"] option`).Length())
	form.Find("select").Each(func(_ int, sel *goquery.Selection) {
		assert.False(t, sel.HasClass("month-type"), "month-type is hidden on phones")
		assert.True(t, sel.HasClass("trend-select"))
	})

	tiles := doc.Find(".kpi")
	assert.Equal(t, 3, tiles.Length())
	assert.Contains(t, tiles.Eq(0).Text(), "110.00")
	assert.True(t, tiles.Eq(0).Find(".kpi-change").HasClass("up"))
	assert.Contains(t, tiles.Eq(1).Text(), "55.00")
	assert.Contains(t, tiles.Eq(2).Text(), "Nov")

	assert.Equal(t, 1, doc.Find("svg.trend-svg").Length())
	legend := doc.Find(".trend-legend li")
	assert.Equal(t, 3, legend.Length())
	assert.Equal(t, 1, doc.Find(".trend-legend .legend-swatch.dashed").Length(), "monthly average")

	car := parseHTML(t, render(t, "trends", trendsMonthPage("car", "type.")))
	assert.Equal(t, 2, car.Find(".trend-tabs a").Length(), "car: month, year")
}

func TestTrendNamesTranslated(t *testing.T) {
	p := trendsMonthPage("house", "type.")
	p.Lang = "bg"
	doc := parseHTML(t, render(t, "trends", p))
	gas := utilities.T("bg", "type.Gas")
	require.NotEqual(t, "type.Gas", gas)
	assert.Equal(t, gas, strings.TrimSpace(doc.Find(".trend-legend li").Eq(0).Text()))
	assert.Equal(t, "Everything else", strings.TrimSpace(doc.Find(".trend-legend li").Eq(1).Text()), "colour 0 is already translated")
	assert.Contains(t, doc.Find("g.hit title").First().Text(), gas)
	assert.Contains(t, doc.Find(".kpi").Eq(2).Text(), gas, "top type")
	assert.Contains(t, doc.Find(`select[name="type"]`).Text(), gas)
}

func trendsYearPage(section string) *models.TrendsPage {
	b := chart.Grouped(chart.DefaultFrame, []string{"Jan", "Feb"}, []chart.Series{
		{Name: "2025", Color: 0, Values: []float64{10, 20}},
		{Name: "2026", Color: 1, Values: []float64{15, 25}},
	})
	return &models.TrendsPage{
		Section: section, Lang: "en", TypePrefix: "type.",
		Query: models.TrendQuery{Tab: "year", Period: "12", YearA: 2026, YearB: 2025, By: "month"},
		Types: []models.TypeOption{{ID: 3, Name: "Gas"}}, Years: []int{2026, 2025, 2023},
		Year: &models.YearTrend{
			YearA: 2026, YearB: 2025, SoFarA: 40, SoFarB: 30, SoFarPct: 33.3, HasSoFar: true, SoFarLabel: "Jan–Nov",
			Riser: "Gas", RiserAmount: 12, HasRiser: true, SavedA: 5, SavedB: 3, Chart: b,
			Legend:     []models.LegendItem{{Name: "2025", Color: 0}, {Name: "2026", Color: 1}},
			YearTotals: []models.YearTotal{{Year: 2026, Total: 40}, {Year: 2025, Total: 30}, {Year: 2023, Total: 7}},
		},
	}
}

func TestTrendsYearTab(t *testing.T) {
	doc := parseHTML(t, render(t, "trends", trendsYearPage("house")))
	active, _ := doc.Find(`.trend-tabs a[aria-current="page"]`).Attr("hx-get")
	assert.Equal(t, "/house/chart?tab=year", active)

	form := doc.Find("form#trends-filters")
	tab, _ := form.Find(`input[name="tab"]`).Attr("value")
	assert.Equal(t, "year", tab)
	a, _ := form.Find(`select[name="a"] option[selected]`).Attr("value")
	b, _ := form.Find(`select[name="b"] option[selected]`).Attr("value")
	assert.Equal(t, []string{"2026", "2025"}, []string{a, b})
	assert.Equal(t, 0, form.Find(`select[name="period"]`).Length())
	assert.Equal(t, 3, form.Find("select.trend-select").Length(), "a, b, type")
	assert.Equal(t, 0, form.Find("select.month-type").Length())

	tiles := doc.Find(".kpi")
	assert.Equal(t, 2, tiles.Length(), "house: so far and riser")
	assert.Contains(t, tiles.Eq(0).Text(), "40.00")
	assert.Contains(t, tiles.Eq(0).Text(), "Jan–Nov")
	assert.Equal(t, "+33%", strings.TrimSpace(tiles.Eq(0).Find(".kpi-change").Text()))
	assert.Contains(t, tiles.Eq(1).Text(), "Gas")
	assert.Equal(t, 1, doc.Find("svg.trend-svg").Length())
	assert.Equal(t, "2025", strings.TrimSpace(doc.Find(".trend-legend li").First().Text()), "years are not translated")

	totals := doc.Find(".year-totals li")
	assert.Equal(t, 3, totals.Length())
	assert.Contains(t, totals.Eq(2).Text(), "2023")
	assert.Contains(t, doc.Find(".year-totals-card").Text(), "All types", "list ignores the type filter")

	g := parseHTML(t, render(t, "trends", trendsYearPage("groceries")))
	assert.Equal(t, 3, g.Find(".kpi").Length(), "groceries add discounts saved")
	assert.Contains(t, g.Find(".kpi").Eq(2).Text(), "5.00")
}

func TestTrendsEmptyState(t *testing.T) {
	p := trendsMonthPage("house", "type.")
	p.Month = &models.MonthTrend{Empty: true, Chart: chart.Stacked(chart.DefaultFrame, []string{"Nov"}, nil)}
	html := render(t, "trend-month", p)
	assert.NotContains(t, html, "<svg")
	assert.Contains(t, parseHTML(t, html).Find(".trend-empty").Text(), "No expenses in this period")

	y := trendsYearPage("car")
	y.Year = &models.YearTrend{YearA: 2026, YearB: 2025, Empty: true}
	html = render(t, "trend-year", y)
	assert.NotContains(t, html, "<svg")
	assert.Contains(t, parseHTML(t, html).Find(".trend-empty").Text(), "2025")
}

func TestTrendsPageHasSectionButtons(t *testing.T) {
	doc := parseHTML(t, render(t, "trends-page", trendsMonthPage("car", "type.")))
	assert.Equal(t, 1, doc.Find("#section-list").Length())
	assert.Equal(t, 1, doc.Find("#section-content .trend-tabs").Length())
	chartBtn := doc.Find(`#section-list button[hx-get="/car/chart"]`)
	push, _ := chartBtn.Attr("hx-push-url")
	assert.Equal(t, "true", push)
	list, _ := doc.Find(`#section-list button[hx-get="/car/current"]`).Attr("hx-push-url")
	assert.Equal(t, "/car", list)
}

func TestNoChartJS(t *testing.T) {
	html := render(t, "root", &models.RootLayout{Lang: "en", HeaderOpts: &models.HeaderOptions{Lang: "en"}})
	lib := strings.Join([]string{"chart", "js"}, ".") // spelled out so a grep for it finds no leftovers
	assert.False(t, strings.Contains(strings.ToLower(html), lib), "no Chart.js script")
	assert.Nil(t, loadTemplates(t).Lookup("exp-chart"), "old chart template removed")
}

func trendsItemPage() *models.TrendsPage {
	f := func(v float64) *float64 { return &v }
	pc := chart.Price(chart.DefaultFrame, []string{"Sep", "Oct"}, []*float64{f(2), f(2.5)}, []*float64{f(1.8), f(2.2)}, []*float64{f(2.2), f(2.8)}, nil, []float64{1, 2})
	d := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	var disc []models.PriceLine
	for i := 0; i < 5; i++ {
		disc = append(disc, models.PriceLine{Date: d, Store: "Lidl", Product: "Milk", PerUnit: 2, DiscountPerUnit: 0.5})
	}
	return &models.TrendsPage{
		Section: "groceries", Lang: "en", TypePrefix: "category.",
		Query: models.TrendQuery{Tab: "item", Period: "12", ItemID: 7, By: "month", Unit: "l"},
		Items: []models.Category{{ID: 7, Name: "Milk"}, {ID: 8, Name: "Bread"}},
		Item: &models.ItemTrend{
			ItemID: 7, Item: "Milk", Unit: "l", Units: []string{"l"}, By: "month",
			PriceNow: 2.5, HasPriceNow: true, PriceChangePct: 12, HasPriceChange: true,
			QtyNow: 3, QtyUsual: 2, HasQtyUsual: true,
			Cheapest:  &models.PriceLine{Date: d, Store: "Kaufland", Product: "Fresh", PerUnit: 1.9},
			Discounts: disc, TotalSaved: 7.5, Chart: pc,
			Items: []models.ItemRow{
				{ItemID: 7, Item: "Milk", Unit: "l", Now: 2.5, YearAgo: 2.2, ChangePct: 13, HasChange: true, Bought12: 20},
				{ItemID: 8, Item: "Bread", Unit: "pc", Now: 1.5, Bought12: 5},
			},
		},
	}
}

func TestTrendItemTab(t *testing.T) {
	p := trendsItemPage()
	doc := parseHTML(t, render(t, "trends", p))
	assert.Equal(t, 3, doc.Find(".kpi-tiles .kpi").Length())
	assert.Equal(t, 1, doc.Find("svg.trend-svg").Length())
	assert.Equal(t, 5, doc.Find("table.item-discounts tbody tr").Length())
	rows := doc.Find("table.item-all tbody tr")
	assert.Equal(t, 2, rows.Length())
	link, _ := rows.First().Find("a").Attr("hx-get")
	assert.Equal(t, "/groceries/chart?tab=item&item=7", link)
	href, _ := rows.First().Find("a").Attr("href")
	assert.Equal(t, "/groceries/chart?tab=item&item=7", href)
	assert.NotContains(t, doc.Find("table.item-all").Text(), "Other")
	assert.Contains(t, doc.Text(), "2.50")
	assert.Equal(t, 0, doc.Find(".unit-switch").Length(), "one unit: no switch")
	assert.Equal(t, 2, doc.Find(`select[name="item"] option`).Length())
	assert.Equal(t, 0, doc.Find(".month-type").Length())

	p.Item.Units = []string{"kg", "l"}
	doc = parseHTML(t, render(t, "trends", p))
	assert.Equal(t, 1, doc.Find(".unit-switch").Length())
}

func TestTrendItemExcludedNote(t *testing.T) {
	p := trendsItemPage()
	p.Item.Excluded = 3
	doc := parseHTML(t, render(t, "trend-item", p))
	note := doc.Find(".item-excluded")
	require.Equal(t, 1, note.Length())
	assert.Contains(t, note.Text(), "3")
	link, _ := note.Find("a").Attr("hx-get")
	assert.Equal(t, "/groceries/sizes", link)

	p.Item.Excluded = 0
	assert.Equal(t, 0, parseHTML(t, render(t, "trend-item", p)).Find(".item-excluded").Length())
}

func TestTrendItemEmpty(t *testing.T) {
	p := trendsItemPage()
	p.Item = &models.ItemTrend{ItemID: 7, Item: "Milk", Unit: "l", Empty: true}
	doc := parseHTML(t, render(t, "trend-item", p))
	assert.Contains(t, doc.Find(".trend-empty").Text(), "Milk")
	assert.Equal(t, 0, doc.Find("svg").Length())
}

func TestTrendItemBought12AndWrappers(t *testing.T) {
	doc := parseHTML(t, render(t, "trend-item", trendsItemPage()))
	assert.Contains(t, doc.Find("table.item-all thead").Text(), "Bought, 12 mo")
	rows := doc.Find("table.item-all tbody tr")
	assert.Contains(t, rows.First().Text(), "20.0 l")
	assert.Contains(t, rows.Last().Text(), "5 pcs")
	assert.Equal(t, 5, doc.Find("table.item-discounts thead th").Length())
	assert.Equal(t, 1, doc.Find(".trend-table-wrap > table.item-discounts").Length())
	assert.Equal(t, 1, doc.Find(".trend-table-wrap > table.item-all").Length())
}
