package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// AuthDemoHandler handles the JWT authentication demo page
type AuthDemoHandler struct{}

// NewAuthDemoHandler creates a new AuthDemoHandler instance
func NewAuthDemoHandler() *AuthDemoHandler {
	return &AuthDemoHandler{}
}

// GetAuthDemo serves the JWT authentication demo page
func (h *AuthDemoHandler) GetAuthDemo(c *gin.Context) {
	// Serve the static HTML file directly
	authTemplatePath := filepath.Join("internal", "templates", "auth.html")
	c.File(authTemplatePath)
}

// GetAuthDemoInfo provides information about the JWT demo
func (h *AuthDemoHandler) GetAuthDemoInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "JWT Authentication Demo",
		"features": []string{
			"User Registration",
			"User Login",
			"JWT Token Generation",
			"Protected API Endpoints",
			"Real-time API Testing",
		},
		"endpoints": gin.H{
			"register":  "POST /api/register",
			"login":     "POST /api/login",
			"profile":   "GET /api/profile (protected)",
			"protected": "GET /api/protected (protected)",
		},
	})
}
