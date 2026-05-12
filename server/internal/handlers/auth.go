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
		c.HTML(http.StatusOK, utilities.Templates.Pages.Register, gin.H{
			"Lang": c.GetString("lang"),
		})
	} else {
		rl := &models.RootLayout{
			TemplateName: utilities.Templates.Pages.Register,
			HeaderOpts: &models.HeaderOptions{
				Lang: c.GetString("lang"),
			},
			Lang: c.GetString("lang"),
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

// APIRegister handles user registration via API
func (h *AuthHandler) Register(c *gin.Context) {
	lang := c.GetString("lang")
	var regData models.UserRegistration

	if err := c.ShouldBind(&regData); err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "400: Invalid request data.",
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	// Check if user already exists
	existingUser, _ := h.DB.GetUserByUsername(regData.Username)
	if existingUser != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "auth.username_exists"),
			Message: "400: Invalid request data.",
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(regData.Password), bcrypt.DefaultCost)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "500: Internal server error.",
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	// Create user
	user := &models.User{
		Username:     regData.Username,
		PasswordHash: string(hashedPassword),
	}

	if err := h.DB.CreateUser(user); err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "500: Internal server error.",
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateToken(user)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "500: Internal server error.",
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	h.AuthService.SetCookie(token, c)
	c.HTML(http.StatusCreated, utilities.Templates.Responses.RegisterSuccess, gin.H{
		"User": user,
		"Lang": lang,
	})
}

func (h *AuthHandler) GetLogin(c *gin.Context) {
	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Pages.Login, gin.H{
			"Lang": c.GetString("lang"),
		})
	} else {
		rl := &models.RootLayout{
			TemplateName: utilities.Templates.Pages.Login,
			HeaderOpts: &models.HeaderOptions{
				Lang: c.GetString("lang"),
			},
			Lang: c.GetString("lang"),
		}
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

// APILogin handles user login via API
func (h *AuthHandler) Login(c *gin.Context) {
	lang := c.GetString("lang")
	var loginData models.UserLogin

	if err := c.ShouldBind(&loginData); err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "400: Invalid request data.",
			Lang:    lang,
		}
		c.HTML(http.StatusBadRequest, utilities.Templates.Components.ModalError, content)
		return
	}

	// Get user from database
	user, err := h.DB.GetUserByUsername(loginData.Username)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "auth.invalid_credentials"),
			Message: "401: Unauthorized.",
			Lang:    lang,
		}
		c.HTML(http.StatusUnauthorized, utilities.Templates.Components.ModalError, content)
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(loginData.Password)); err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "auth.invalid_credentials"),
			Message: "401: Unauthorized.",
			Lang:    lang,
		}
		c.HTML(http.StatusUnauthorized, utilities.Templates.Components.ModalError, content)
		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateToken(user)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "500: Failed to get authentication token.",
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	h.AuthService.SetCookie(token, c)
	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	h.AuthService.ClearCookie(c)

	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
}
