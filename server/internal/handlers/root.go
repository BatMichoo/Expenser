package handlers

import (
	database "expenser/internal/db"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RootHandler struct {
	DB *database.DB
}

type RootLayout struct {
	TemplateName    string
	TemplateContent any
}

func NewRootHandler(db *database.DB) *RootHandler {
	return &RootHandler{
		DB: db,
	}
}

func (h *RootHandler) GetRoot(c *gin.Context) {
	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, "index-content", gin.H{})
	} else {
		rl := &RootLayout{
			TemplateName: "index-content",
		}
		c.HTML(http.StatusOK, "index", rl)
	}
}
