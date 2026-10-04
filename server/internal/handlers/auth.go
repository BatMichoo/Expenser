package handlers

import (
	"errors"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles API endpoints for JWT authentication
type AuthHandler struct {
	BaseHandler
}

// NewAuthHandler creates a new APIHandler instance
func NewAuthHandler(rh *RootHandler) *AuthHandler {
	return &AuthHandler{
		BaseHandler: rh.BaseHandler,
	}
}

func (h *AuthHandler) GetRegister(c *gin.Context) {
	RenderPage(c, false, utilities.Templates.Pages.Register, nil)
}

// APIRegister handles user registration via API
func (h *AuthHandler) Register(c *gin.Context) {
	lang := c.GetString("lang")
	var regData models.UserRegistration

	if err := c.ShouldBind(&regData); err != nil {
		error := errors.New("Invalid request data.")
		RenderErrorModal(c, error, http.StatusBadRequest)
		return
	}

	// Check if user already exists
	existingUser, _ := h.BaseHandler.DB.GetUserByUsername(regData.Username)
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

	if err := h.BaseHandler.DB.CreateUser(user); err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "500: Internal server error.",
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	// Generate JWT token
	token, err := h.BaseHandler.AS.GenerateToken(user)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "500: Internal server error.",
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	h.BaseHandler.AS.SetCookie(token, c)
	c.HTML(http.StatusCreated, utilities.Templates.Responses.RegisterSuccess, gin.H{
		"User": user,
		"Lang": lang,
	})
}

func (h *AuthHandler) GetLogin(c *gin.Context) {
	isHtmxRequest := c.Request.Header.Get("HX-Request") == "true"
	lang := c.GetString("lang")

	if isHtmxRequest {
		c.HTML(http.StatusOK, utilities.Templates.Pages.Login, gin.H{
			"Lang": lang,
		})
	} else {
		rl := &models.RootLayout{
			TemplateName: utilities.Templates.Pages.Login,
			HeaderOpts: &models.HeaderOptions{
				Lang: lang,
			},
			Lang: lang,
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
	user, err := h.BaseHandler.DB.GetUserByUsername(loginData.Username)
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
	token, err := h.BaseHandler.AS.GenerateToken(user)
	if err != nil {
		content := &models.ModalContent{
			Title:   utilities.T(lang, "modal.error_title"),
			Message: "500: Failed to generate authentication token.",
			Lang:    lang,
		}
		c.HTML(http.StatusInternalServerError, utilities.Templates.Components.ModalError, content)
		return
	}

	h.BaseHandler.AS.SetCookie(token, c)
	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	h.BaseHandler.AS.ClearCookie(c)

	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
}
