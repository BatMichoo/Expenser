package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"expenser/internal/middleware"
	"expenser/internal/templates"
	"expenser/internal/utilities"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// page renders a full page load of tmpl the way the routes do.
func page(t *testing.T, tmpl string, data any, loggedIn bool, cookie string) *goquery.Document {
	t.Helper()
	require.NoError(t, utilities.InitI18n())
	set, err := utilities.ParseTemplates(templates.FS, templates.Patterns...)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.SetHTMLTemplate(set)
	r.Use(middleware.I18nMiddleware(), withSite(site{AuthURL: "https://auth.homeserver.lan", LanDomain: "homeserver.lan", HomeServerURL: "https://homeserver.lan/"}))
	r.GET("/", func(c *gin.Context) {
		if loggedIn {
			middleware.SetUser(c, middleware.User{ID: uuid.New(), Name: "Miro"})
		}
		renderLayout(c, http.StatusOK, tmpl, data)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(w.Body.String()))
	require.NoError(t, err)
	return doc
}

func TestLayoutIsTheSharedTheme(t *testing.T) {
	doc := page(t, "snooze-page", map[string]any{"State": "invalid", "Lang": "en"}, true, "")
	assert.Contains(t, doc.Find("main#content").Text(), "doesn't work", "the page by its name, with its data")
	assert.Equal(t, 1, doc.Find("dialog#sheet").Length(), "the theme's sheet")
	assert.Equal(t, 1, doc.Find("dialog#toast").Length(), "and toast")

	nav := doc.Find("header #nav .nav-button")
	var paths []string
	nav.Each(func(_ int, b *goquery.Selection) { paths = append(paths, b.AttrOr("data-path", "")) })
	assert.Equal(t, []string{"/house", "/car", "/groceries", "/bills"}, paths)
	assert.Equal(t, 4, nav.Find("svg").Length(), "Expenser's icons")
	assert.Equal(t, "https://homeserver.lan/", doc.Find("header a.home-crumb").AttrOr("href", ""), "Home › leads to the dashboard")
	assert.Contains(t, doc.Find("header .app-crumb").Text(), "Expense Tracker")

	menu := doc.Find("details.settings-menu")
	assert.Equal(t, "https://auth.homeserver.lan/account", menu.Find("form.settings-logout-form[method=get]").AttrOr("action", ""))
	assert.Equal(t, 1, menu.Find(`form[method="post"][action="https://auth.homeserver.lan/logout"]`).Length())
	assert.Equal(t, 2, menu.Find(`.lang-btn[hx-post="/settings/language"]`).Length())
	assert.Equal(t, "homeserver.lan", doc.Find("body").AttrOr("data-theme-cookie-domain", ""), "light/dark for every app")

	// The theme's CSS and scripts first, then Expenser's, each with a version.
	var css, js []string
	doc.Find(`link[rel="stylesheet"]`).Each(func(_ int, l *goquery.Selection) { css = append(css, l.AttrOr("href", "")) })
	doc.Find(`script[src]`).Each(func(_ int, s *goquery.Selection) { js = append(js, s.AttrOr("src", "")) })
	require.Len(t, css, 7)
	assert.True(t, strings.HasPrefix(css[0], "/static/theme/css/tokens.css?v="))
	assert.True(t, strings.HasPrefix(css[4], "/static/expenser/css/styles.css?v="))
	require.Len(t, js, 3)
	assert.True(t, strings.HasPrefix(js[0], "/static/theme/js/htmx.min.js?v="), "one htmx: the theme's")
	assert.True(t, strings.HasPrefix(js[2], "/static/expenser/js/main.js?v="))
}

func TestLayoutUnknownNameShowsHome(t *testing.T) {
	doc := page(t, "no-such-page", nil, true, "")
	assert.Equal(t, 1, doc.Find("main#content section.home").Length())
}

func TestLayoutLoggedOut(t *testing.T) {
	// Snooze links open without a login: no navigation, account or log out.
	doc := page(t, "snooze-page", map[string]any{"State": "invalid", "Lang": "en"}, false, "")
	assert.Equal(t, 0, doc.Find("#nav .nav-button").Length())
	assert.Equal(t, 0, doc.Find(`form[action$="/logout"]`).Length())
	assert.Equal(t, 2, doc.Find(".theme-option").Length(), "light/dark still there")
}

func TestLayoutLanguageAndMode(t *testing.T) {
	doc := page(t, "no-such-page", nil, true, "homeserver_lang=bg; homeserver_theme=dark")
	assert.Equal(t, "bg", doc.Find("html").AttrOr("lang", ""))
	assert.True(t, doc.Find("body").HasClass("dark"), "rendered dark, no flash of light")
}
