package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/utilities"

	"github.com/gin-gonic/gin"
)

type CarHandler struct {
	DB *database.DB
}

func NewCarHandler(db *database.DB) *CarHandler {
	return &CarHandler{
		DB: db,
	}
}

func (h *CarHandler) GetHome(c *gin.Context) {
	c.HTML(200, utilities.Templates.Pages.Car, map[string]any{})
}
