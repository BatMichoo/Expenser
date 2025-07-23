package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"net/http"
	"os"

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
		rl := &RootLayout{
			TemplateName: utilities.Templates.Pages.Register,
			HeaderOpts:   &HeaderOptions{},
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

// APIRegister handles user registration via API
func (h *AuthHandler) Register(c *gin.Context) {
	var regData models.UserRegistration

	if err := c.ShouldBind(&regData); err != nil {
		c.HTML(http.StatusBadRequest, "error", gin.H{"Error": "Invalid request data: " + err.Error()})
		return
	}

	// Check if user already exists
	existingUser, _ := h.DB.GetUserByUsername(regData.Username)
	if existingUser != nil {
		c.HTML(http.StatusConflict, "error", gin.H{"Error": "Username already exists"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(regData.Password), bcrypt.DefaultCost)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"Error": "Failed to process password"})
		return
	}

	// Create user
	user := &models.User{
		Username:     regData.Username,
		PasswordHash: string(hashedPassword),
	}

	if err := h.DB.CreateUser(user); err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"Error": "Failed to create user account"})
		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateToken(user)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"Error": "Failed to generate authentication token"})
		return
	}

	c.Header("Authorization", "Bearer "+token.Value)
	c.HTML(http.StatusCreated, utilities.Templates.Pages.Index, user)
}

func (h *AuthHandler) GetLogin(c *gin.Context) {
	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Pages.Login, gin.H{})
	} else {
		rl := &RootLayout{
			TemplateName: utilities.Templates.Pages.Login,
			HeaderOpts:   &HeaderOptions{},
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

// APILogin handles user login via API
func (h *AuthHandler) Login(c *gin.Context) {
	var loginData models.UserLogin

	if err := c.ShouldBind(&loginData); err != nil {
		c.HTML(http.StatusBadRequest, "error", gin.H{"error": "Invalid request data: " + err.Error()})
		return
	}

	// Get user from database
	user, err := h.DB.GetUserByUsername(loginData.Username)
	if err != nil {
		c.HTML(http.StatusUnauthorized, "error", gin.H{"error": "Invalid username or password"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(loginData.Password)); err != nil {
		c.HTML(http.StatusUnauthorized, "error", gin.H{"error": "Invalid username or password"})
		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateToken(user)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": "Failed to generate authentication token"})
		return
	}

	domain := os.Getenv("LAN_DOMAIN")

	if domain == "" {
		domain = "localhost"
	}

	c.SetCookie("auth_token", token.Value, int(token.Expiration), "/", domain, false, true)
	c.HTML(http.StatusOK, utilities.Templates.Responses.LoginSuccess, &HeaderOptions{
		IsLoggedIn: true,
		IsOOB:      true,
	})
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

	c.HTML(http.StatusOK, utilities.Templates.Responses.LoginSuccess, &HeaderOptions{
		IsOOB: true,
	})
}

// APIProfile returns the authenticated user's profile
func (h *AuthHandler) Profile(c *gin.Context) {
	// userID, username, email, exists := middleware.GetUserFromContext(c)
	// if !exists {
	// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
	// 	return
	// }
	//
	// c.JSON(http.StatusOK, gin.H{
	// 	"user": gin.H{
	// 		"id":       userID,
	// 		"username": username,
	// 		"email":    email,
	// 	},
	// })
}
