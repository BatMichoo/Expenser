package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/utilities"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RootHandler struct {
	DB *database.DB
}

type HeaderOptions struct {
	IsLoggedIn bool
	IsOOB      bool
}

type RootLayout struct {
	TemplateName    string
	TemplateContent any
	HeaderOpts      *HeaderOptions
}

func NewRootHandler(db *database.DB) *RootHandler {
	return &RootHandler{
		DB: db,
	}
}

func (h *RootHandler) GetRoot(c *gin.Context) {
	authC, _ := c.Cookie("auth_token")
	isLoggedIn := authC != ""
	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Pages.Index, gin.H{})
	} else {
		rl := &RootLayout{
			TemplateName: utilities.Templates.Pages.Index,
			HeaderOpts: &HeaderOptions{
				IsLoggedIn: isLoggedIn,
			},
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}
