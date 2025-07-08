package handlers

import (
	database "expenser/internal/db"

	"github.com/gin-gonic/gin"
)

type RootHandler struct {
	DB *database.DB
}

func NewRootHandler(db *database.DB) *RootHandler {
	return &RootHandler{
		DB: db,
	}
}

func (h *RootHandler) GetRoot(c *gin.Context) {
	c.HTML(200, "index", map[string]any{})
}
