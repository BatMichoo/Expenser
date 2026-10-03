package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func (h *RootHandler) GetRoot(c *gin.Context) {
	claims, _ := h.AS.ValidateToken(c)
	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	lang := c.GetString("lang")

	year := time.Now().Year()
	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Pages.Index, gin.H{"Lang": lang, "IsLoggedIn": claims != nil, "Year": year})
	} else {
		rl := &models.RootLayout{
			TemplateName: utilities.Templates.Pages.Index,
			HeaderOpts: &models.HeaderOptions{
				IsLoggedIn: claims != nil,
				Lang:       lang,
			},
			Lang:       lang,
			Year:       year,
			IsLoggedIn: claims != nil,
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

func (h *RootHandler) NotFound(c *gin.Context) {
	claims, _ := h.AS.ValidateToken(c)
	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"
	lang := c.GetString("lang")

	if isHtmxRequest {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "404: Page not found!",
			Lang:    lang,
		}
		c.HTML(http.StatusNotFound, utilities.Templates.Components.ModalError, content)
	} else {
		rl := &models.RootLayout{
			TemplateName: utilities.Templates.Pages.Index,
			HeaderOpts: &models.HeaderOptions{
				IsLoggedIn: claims != nil,
				Lang:       lang,
			},
			Lang:       lang,
			IsLoggedIn: claims != nil,
		}
		c.HTML(http.StatusNotFound, utilities.Templates.Root, rl)
	}
}

func (h *RootHandler) ChangeLanguage(c *gin.Context) {
	lang := c.PostForm("lang")
	if lang == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	c.SetCookie("lang", lang, 3600*24*30, "/", "", false, true)

	userIDstr, exists := c.Get("user_id")
	if exists {
		userID, ok := userIDstr.(uuid.UUID)
		if ok {
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
	}

	c.Header("HX-Refresh", "true")
	c.Status(http.StatusOK)
}
