package handlers

import (
	"expenser/internal/middleware"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RootHandler serves the home page, the flash popup and the language
// switch.
type RootHandler struct {
	users UserStore
	homes *services.HomeService
	auth  *services.AuthService
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
	user, err := h.users.GetUserByID(c.Request.Context(), currentUserID(c))
	if err != nil {
		return nil, err
	}
	name := user.DisplayName
	if name == "" {
		name = user.Username
	}
	return h.homes.Build(c.Request.Context(), user.ID, name, currentLang(c), time.Now())
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

	middleware.SetLanguageCookie(c, h.auth.Domain, lang)

	// Logged in: remember the choice on the account too, and reissue the
	// token so its claims carry the new language. The cookie alone already
	// switches this browser, so a failure here is only logged.
	if claims := services.ClaimsFromContext(c); claims != nil {
		if err := h.saveLanguage(c, claims.UserID, lang); err != nil {
			log.Printf("save language for %s: %v", claims.UserID, err)
		}
	}

	c.Header("HX-Refresh", "true")
	c.Status(http.StatusOK)
	return nil
}

// saveLanguage stores the user's language and refreshes their login token.
func (h *RootHandler) saveLanguage(c *gin.Context, userID uuid.UUID, lang string) error {
	ctx := c.Request.Context()
	if err := h.users.UpdateUserLanguage(ctx, userID, lang); err != nil {
		return err
	}
	user, err := h.users.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	token, err := h.auth.GenerateToken(user)
	if err != nil {
		return err
	}
	h.auth.SetCookie(token, c)
	return nil
}
