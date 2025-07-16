package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/middleware"
	"expenser/internal/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// APIHandler handles API endpoints for JWT authentication
type APIHandler struct {
	DB          *database.DB
	AuthService *middleware.AuthService
}

// NewAPIHandler creates a new APIHandler instance
func NewAPIHandler(db *database.DB, authService *middleware.AuthService) *APIHandler {
	return &APIHandler{
		DB:          db,
		AuthService: authService,
	}
}

// APIRegister handles user registration via API
func (h *APIHandler) APIRegister(c *gin.Context) {
	var regData models.UserRegistration

	if err := c.ShouldBindJSON(&regData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
		return
	}

	// Check if user already exists
	existingUser, _ := h.DB.GetUserByUsername(regData.Username)
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	existingUser, _ = h.DB.GetUserByEmail(regData.Email)
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(regData.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Create user
	user := &models.User{
		Username:     regData.Username,
		Email:        regData.Email,
		PasswordHash: string(hashedPassword),
	}

	if err := h.DB.CreateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user account"})
		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authentication token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"token":   token,
		"user":    user.ToResponse(),
	})
}

// APILogin handles user login via API
func (h *APIHandler) APILogin(c *gin.Context) {
	var loginData models.UserLogin

	if err := c.ShouldBindJSON(&loginData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
		return
	}

	// Get user from database
	user, err := h.DB.GetUserByUsername(loginData.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(loginData.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authentication token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"token":   token,
		"user":    user.ToResponse(),
	})
}

// APIProfile returns the authenticated user's profile
func (h *APIHandler) APIProfile(c *gin.Context) {
	userID, username, email, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":       userID,
			"username": username,
			"email":    email,
		},
	})
}

// APIProtectedExample demonstrates a protected API endpoint
func (h *APIHandler) APIProtectedExample(c *gin.Context) {
	userID, username, _, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "This is a protected endpoint",
		"user_id":   userID,
		"username":  username,
		"timestamp": time.Now().Unix(),
	})
}
