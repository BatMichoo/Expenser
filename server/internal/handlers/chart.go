package handlers

import (
	"expenser/internal/groceryunits"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ChartHandler serves each section's trends (charts) page.
type ChartHandler struct {
	trends *services.TrendsService
}

// Show renders the trends page of sec: the fragment for HTMX requests, the
// full page (section buttons and all) otherwise.
func (h *ChartHandler) Show(sec models.Section) func(*gin.Context) error {
	return func(c *gin.Context) error {
		now := time.Now()
		q := ParseTrendQuery(c.Request.URL.Query(), sec, now)
		page, err := h.trends.Page(c.Request.Context(), currentUserID(c), sec, q, currentLang(c), now)
		if err != nil {
			return err
		}
		renderView(c, utilities.Templates.Components.Trends, utilities.Templates.Pages.Trends, page)
		return nil
	}
}

// minTrendYear is the oldest year the trends page accepts in its query.
const minTrendYear = 2000

// ParseTrendQuery reads the trends page query string. Every value is
// validated; anything missing or invalid falls back to its default, so a bad
// link never fails the page.
func ParseTrendQuery(v url.Values, sec models.Section, now time.Time) models.TrendQuery {
	validYear := func(y int) bool { return y >= minTrendYear && y <= now.Year()+1 }
	positive := func(key string) int {
		n, err := strconv.Atoi(v.Get(key))
		if err != nil || n < 0 {
			return 0
		}
		return n
	}

	q := models.TrendQuery{Tab: "month", Period: "12", By: "month"}

	switch tab := v.Get("tab"); tab {
	case "year":
		q.Tab = tab
	case "item":
		if sec == models.SectionGroceries {
			q.Tab = tab
		}
	}

	switch p := v.Get("period"); {
	case p == "6" || p == "12" || p == "24":
		q.Period = p
	case p == "all" && q.Tab == "item":
		q.Period = p
	case strings.HasPrefix(p, "y") && q.Tab != "item": // item tab has no calendar-year period
		digits := p[1:]
		if y, err := strconv.Atoi(digits); err == nil && len(digits) == 4 && validYear(y) {
			q.Period = p
		}
	}

	q.TypeID = positive("type")
	q.ItemID = positive("item")

	q.YearA = now.Year()
	if a, err := strconv.Atoi(v.Get("a")); err == nil && validYear(a) {
		q.YearA = a
	}
	q.YearB = q.YearA - 1
	if b, err := strconv.Atoi(v.Get("b")); err == nil && validYear(b) && b != q.YearA {
		q.YearB = b
	}

	if v.Get("by") == "year" {
		q.By = "year"
	}
	if u := v.Get("unit"); groceryunits.Unit(u).Valid() {
		q.Unit = u
	}
	return q
}
