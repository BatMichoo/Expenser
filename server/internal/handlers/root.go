package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/middleware"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type BaseHandler struct {
	DB *database.DB
	AS *services.AuthService
}

func NewBaseHandler(db *database.DB, as *services.AuthService) BaseHandler {
	return BaseHandler{
		DB: db,
		AS: as,
	}
}

type RootHandler struct {
	BaseHandler
}

func NewRootHandler(db *database.DB, as *services.AuthService) *RootHandler {
	return &RootHandler{
		BaseHandler: NewBaseHandler(db, as),
	}
}

func (h *RootHandler) GetRoot(c *gin.Context) error {
	data := gin.H{
		"Lang":       currentLang(c),
		"IsLoggedIn": isLoggedIn(c),
		"Year":       time.Now().Year(),
	}
	if isLoggedIn(c) {
		// The home page must always load: without its data it just greets.
		home, err := h.home(c)
		if err != nil {
			log.Printf("home page data: %v", err)
		} else {
			data["Home"] = home
		}
	}
	renderPage(c, utilities.Templates.Pages.Index, data)
	return nil
}

// home builds the logged-in home page's cards.
func (h *RootHandler) home(c *gin.Context) (*models.HomeView, error) {
	user, err := h.DB.GetUserByID(currentUserID(c))
	if err != nil {
		return nil, err
	}
	name := user.DisplayName
	if name == "" {
		name = user.Username
	}
	return services.NewHomeService(h.DB).Build(c.Request.Context(), user.ID, name, currentLang(c), time.Now())
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

func (h *RootHandler) ChangeLanguage(c *gin.Context) error {
	lang := c.PostForm("lang")
	if !middleware.SupportedLanguage(lang) {
		c.Status(http.StatusBadRequest)
		return nil
	}

	middleware.SetLanguageCookie(c, h.AS.Domain, lang)

	if claims := services.ClaimsFromContext(c); claims != nil {
		userID := claims.UserID
		_ = h.DB.UpdateUserLanguage(userID, lang)

		// If logged in, we should also regenerate the token to include the new language preference
		// to avoid stale claims in the middleware.
		user, err := h.DB.GetUserByID(userID)
		if err == nil {
			token, err := h.AS.GenerateToken(user)
			if err == nil {
				h.AS.SetCookie(token, c)
			}
		}
	}

	c.Header("HX-Refresh", "true")
	c.Status(http.StatusOK)
	return nil
}
