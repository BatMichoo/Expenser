package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles API endpoints for JWT authentication
type AuthHandler struct {
	DB          *database.DB
	AuthService *services.AuthService
}

// NewAuthHandler creates a new APIHandler instance
func NewAuthHandler(db *database.DB, authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		DB:          db,
		AuthService: authService,
	}
}

func (h *AuthHandler) GetRegister(c *gin.Context) {
	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Pages.Register, gin.H{})
	} else {
		rl := &models.RootLayout{
			TemplateName: utilities.Templates.Pages.Register,
			HeaderOpts:   &models.HeaderOptions{},
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

// APIRegister handles user registration via API
func (h *AuthHandler) Register(c *gin.Context) {
	var regData models.UserRegistration

	if err := c.ShouldBind(&regData); err != nil {
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.Error, gin.H{"Error": "Invalid request data: " + err.Error()})
		return
	}

	// Check if user already exists
	existingUser, _ := h.DB.GetUserByUsername(regData.Username)
	if existingUser != nil {
		c.HTML(http.StatusConflict, utilities.Templates.Components.Error, gin.H{"Error": "Username already exists"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(regData.Password), bcrypt.DefaultCost)
	if err != nil {
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.Error, gin.H{"Error": "Failed to process password"})
		return
	}

	// Create user
	user := &models.User{
		Username:     regData.Username,
		PasswordHash: string(hashedPassword),
	}

	if err := h.DB.CreateUser(user); err != nil {
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.Error, gin.H{"Error": "Failed to create user account"})
		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateToken(user)
	if err != nil {
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.Error, gin.H{"Error": "Failed to generate authentication token"})
		return
	}

	h.AuthService.SetCookie(token, c)
	c.HTML(http.StatusCreated, utilities.Templates.Responses.RegisterSuccess, user)
}

func (h *AuthHandler) GetLogin(c *gin.Context) {
	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Pages.Login, gin.H{})
	} else {
		rl := &models.RootLayout{
			TemplateName: utilities.Templates.Pages.Login,
			HeaderOpts:   &models.HeaderOptions{},
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

// APILogin handles user login via API
func (h *AuthHandler) Login(c *gin.Context) {
	var loginData models.UserLogin

	if err := c.ShouldBind(&loginData); err != nil {
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.Error, gin.H{"Error": "Invalid request data: " + err.Error()})
		return
	}

	// Get user from database
	user, err := h.DB.GetUserByUsername(loginData.Username)
	if err != nil {
		c.HTML(http.StatusUnauthorized, utilities.Templates.Components.Error, gin.H{"Error": "Invalid username or password"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(loginData.Password)); err != nil {
		c.HTML(http.StatusUnauthorized, utilities.Templates.Components.Error, gin.H{"Error": "Invalid username or password"})
		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateToken(user)
	if err != nil {
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.Error, gin.H{"Error": "Failed to generate authentication token"})
		return
	}

	h.AuthService.SetCookie(token, c)
	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie(
		"auth_token",
		"",
		-1,
		"/",
		"",
		false,
		true,
	)

	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
}
