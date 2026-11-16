package handlers

import (
	"context"
	"expenser/internal/middleware"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RootHandler serves the home page, the flash popup and the language
// switch.
type RootHandler struct {
	homes     *services.HomeService
	lanDomain string
}

func (h *RootHandler) GetRoot(c *gin.Context) error {
	data := gin.H{
		"Lang": currentLang(c),
		"Year": time.Now().Year(),
	}
	// The home page must always load: without its data it just greets.
	home, err := h.home(c)
	if err != nil {
		log.Printf("home page data: %v", err)
	} else {
		data["Home"] = home
	}
	renderPage(c, utilities.Templates.Pages.Index, data)
	return nil
}

// home builds the logged-in home page's cards.
func (h *RootHandler) home(c *gin.Context) (*models.HomeView, error) {
	u, _ := middleware.CurrentUser(c)
	return h.homes.Build(c.Request.Context(), u.ID, u.Name, currentLang(c), time.Now())
}

// GetFlash pops the pending flash error up as the error modal.
func (h *RootHandler) GetFlash(c *gin.Context) error {
	f, ok := popFlash(c)
	if !ok {
		c.Status(http.StatusNoContent)
		return nil
	}

	c.HTML(http.StatusOK, utilities.Templates.Components.ModalError, &models.ModalContent{
		Title:   f.Title,
		Message: f.Message,
		Lang:    currentLang(c),
	})
	return nil
}

func (h *RootHandler) NotFound(c *gin.Context) error {
	return notFound("error.page_not_found")
}

// ChangeLanguage sets the language for every app of the home server (the
// header's EN/BG). RequireUser saves it on the user on their next request,
// for their reminder emails.
func (h *RootHandler) ChangeLanguage(c *gin.Context) error {
	lang := c.PostForm("lang")
	if !middleware.SupportedLanguage(lang) {
		c.Status(http.StatusBadRequest)
		return nil
	}
	middleware.SetLanguageCookie(c, h.lanDomain, lang)
	c.Header("HX-Refresh", "true")
	c.Status(http.StatusOK)
	return nil
}

// health answers "ok" while the database answers, else 503.
func health(store interface{ Ping(context.Context) error }) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			log.Printf("health: %v", err)
			c.String(http.StatusServiceUnavailable, "database unavailable")
			return
		}
		c.String(http.StatusOK, "ok")
	}
}
