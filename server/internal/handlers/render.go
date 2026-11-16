package handlers

import (
	"expenser/internal/middleware"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"net/http"

	"github.com/BatMichoo/theme"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// currentLang returns the language I18nMiddleware resolved for this request.
func currentLang(c *gin.Context) string {
	return c.GetString(middleware.LanguageCookieKey)
}

// site is where Expenser sits in the home server, for the page header.
type site struct {
	AuthURL   string // the login service: Account, Log out
	LanDomain string // the theme cookie is set for it
}

const siteKey = "expenser.site"

// withSite makes s available to renderLayout for every request.
func withSite(s site) gin.HandlerFunc {
	return func(c *gin.Context) { c.Set(siteKey, s); c.Next() }
}

func siteOf(c *gin.Context) site {
	v, _ := c.Get(siteKey)
	s, _ := v.(site)
	return s
}

// currentUserID returns the logged-in user's ID, or uuid.Nil on routes
// without RequireUser.
func currentUserID(c *gin.Context) uuid.UUID {
	u, _ := middleware.CurrentUser(c)
	return u.ID
}

// isHTMX reports whether the request wants a fragment. A history restore
// after a cache miss (back button) also sends HX-Request but swaps the whole
// body, so it gets the full page.
func isHTMX(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true" && c.GetHeader("HX-History-Restore-Request") != "true"
}

// renderPage renders tmpl as a fragment for HTMX requests, or inside the root
// layout for full page loads.
func renderPage(c *gin.Context, tmpl string, data any) {
	renderView(c, tmpl, tmpl, data)
}

// renderView renders partial for HTMX requests and page inside the root layout
// for full page loads. Use it when a route swaps only part of a page.
func renderView(c *gin.Context, partial, page string, data any) {
	if isHTMX(c) {
		c.HTML(http.StatusOK, partial, data)
		return
	}
	renderLayout(c, http.StatusOK, page, data)
}

// pageData is what the page around a full page load passes to Expenser's
// parts of it (templates/components/layout.html).
type pageData struct {
	Name     string // the content template
	Data     any    // its data
	Lang     string
	HasFlash bool // a flash error waits to be shown as a popup
}

// renderLayout renders tmpl inside the shared theme's page (header,
// footer, sheet and toast containers); a name that is not a template
// shows the home page.
func renderLayout(c *gin.Context, status int, tmpl string, data any) {
	lang := currentLang(c)
	s := siteOf(c)
	_, loggedIn := middleware.CurrentUser(c)
	app := utilities.T(lang, "header.title")

	header := theme.Header{
		AppName: app, HomeURL: "/", Lang: lang, LoggedIn: loggedIn,
		LanguageURL: "/settings/language",
	}
	if loggedIn {
		header.Nav = []theme.NavItem{
			{Label: utilities.T(lang, "nav.house"), URL: "/house", Icon: "house"},
			{Label: utilities.T(lang, "nav.car"), URL: "/car", Icon: "car"},
			{Label: utilities.T(lang, "nav.groceries"), URL: "/groceries", Icon: "groceries"},
			{Label: utilities.T(lang, "nav.bills"), URL: "/bills", Icon: "bills"},
		}
		header.AccountURL = s.AuthURL + "/account"
		header.LogoutURL = s.AuthURL + "/logout"
	}
	asset := func(p string) string { return utilities.StaticPrefix + "/" + p + "?v=" + utilities.AssetVersion() }

	c.HTML(status, "theme/page", theme.Page{
		Lang:         lang,
		Title:        app,
		Mode:         theme.ModeFromRequest(c.Request),
		CookieDomain: s.LanDomain,
		Header:       header,
		Footer:       theme.Footer{Year: time.Now().Year(), AppName: app, Version: utilities.AppVersion(), Lang: lang},
		Content:      "expenser/content",
		BodyEnd:      "expenser/body-end",
		Data:         pageData{Name: tmpl, Data: data, Lang: lang, HasFlash: hasFlash(c)},
		CSS:          []string{asset("css/styles.css"), asset("css/mobile.css"), asset("css/tablet.css")},
		JS:           []string{asset("js/main.js")},
	})
}

// renderSuccess renders the success modal.
func renderSuccess(c *gin.Context, status int, title, message string) {
	c.HTML(status, utilities.Templates.Components.ModalSuccess, &models.ModalContent{
		Title:   title,
		Message: message,
		Lang:    currentLang(c),
	})
}

// renderError shows the error modal as a popup. For HTMX requests the main
// swap is skipped so the request's target keeps its content, and the modal is
// swapped out of band. A full page load has nothing to swap into, so the
// error is stored as a flash and the browser is sent to the home page, which
// pops it up (see flash.go).
func renderError(c *gin.Context, status int, title, message string) {
	if isHTMX(c) {
		c.Header("HX-Reswap", "none")
		c.HTML(status, utilities.Templates.Components.ModalError, &models.ModalContent{
			Title:   title,
			Message: message,
			Lang:    currentLang(c),
		})
		return
	}

	setFlash(c, title, message)

	// Redirecting home from home would loop, so render it in place.
	if c.Request.URL.Path == "/" {
		renderLayout(c, status, utilities.Templates.Pages.Index, nil)
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

// expensesChanged tells the page (via HX-Trigger) that expenses were added,
// edited or deleted, so the month view reloads its list and totals.
func expensesChanged(c *gin.Context) {
	c.Header("HX-Trigger", "expenses-changed")
}

// monthFilter reads the search text and type/category filter of a month view.
func monthFilter(c *gin.Context) services.MonthFilter {
	typeID, _ := strconv.Atoi(c.Query("type"))
	return services.MonthFilter{Query: c.Query("q"), TypeID: typeID}
}
