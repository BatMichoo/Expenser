package handlers

import (
	"expenser/internal/middleware"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// currentLang returns the language I18nMiddleware resolved for this request.
func currentLang(c *gin.Context) string {
	return c.GetString(middleware.LanguageCookieKey)
}

// currentUserID returns the logged-in user's ID, or uuid.Nil when anonymous.
func currentUserID(c *gin.Context) uuid.UUID {
	if claims := services.ClaimsFromContext(c); claims != nil {
		return claims.UserID
	}
	return uuid.Nil
}

func isLoggedIn(c *gin.Context) bool {
	return services.ClaimsFromContext(c) != nil
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

// renderLayout renders tmpl inside the root layout, which includes it by
// name; a name that is not a template shows the home page.
func renderLayout(c *gin.Context, status int, tmpl string, data any) {
	lang := currentLang(c)
	loggedIn := isLoggedIn(c)

	c.HTML(status, utilities.Templates.Root, &models.RootLayout{
		TemplateName:    tmpl,
		TemplateContent: data,
		HeaderOpts: &models.HeaderOptions{
			IsLoggedIn: loggedIn,
			Lang:       lang,
		},
		Lang:       lang,
		Year:       time.Now().Year(),
		IsLoggedIn: loggedIn,
		HasFlash:   hasFlash(c),
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
