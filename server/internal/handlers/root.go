package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RootHandler struct {
	DB *database.DB
	AS *services.AuthService
}

type RootLayout struct {
	TemplateName    string
	TemplateContent any
	HeaderOpts      *models.HeaderOptions
}

func NewRootHandler(db *database.DB, as *services.AuthService) *RootHandler {
	return &RootHandler{
		DB: db,
		AS: as,
	}
}

func (h *RootHandler) GetRoot(c *gin.Context) {
	cookie, _ := c.Cookie("auth_token")
	claims, _ := h.AS.ValidateToken(cookie)
	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Pages.Index, gin.H{})
	} else {
		rl := &RootLayout{
			TemplateName: utilities.Templates.Pages.Index,
			HeaderOpts: &models.HeaderOptions{
				IsLoggedIn: claims != nil,
			},
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}
