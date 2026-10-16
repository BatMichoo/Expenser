package handlers

import (
	"errors"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
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

func invalidCredentials(err error) error {
	return &HTTPError{
		Status:     http.StatusUnauthorized,
		TitleKey:   "auth.invalid_credentials",
		MessageKey: "auth.invalid_credentials_message",
		Err:        err,
	}
}

// registrationError turns a binding failure on the registration form into a
// message that names the field to fix.
func registrationError(err error) error {
	messageKey := "error.invalid_form"

	var fieldErrs validator.ValidationErrors
	if errors.As(err, &fieldErrs) && len(fieldErrs) > 0 {
		switch fieldErrs[0].Field() {
		case "Username":
			messageKey = "auth.username_length"
			if fieldErrs[0].Tag() == "required" {
				messageKey = "auth.username_required"
			}
		case "Password":
			messageKey = "auth.password_too_short"
		case "ConfirmPassword":
			messageKey = "auth.passwords_mismatch"
		}
	}

	return badRequest(messageKey, err)
}

func (h *AuthHandler) GetRegister(c *gin.Context) error {
	renderPage(c, utilities.Templates.Pages.Register, gin.H{"Lang": currentLang(c)})
	return nil
}

func (h *AuthHandler) Register(c *gin.Context) error {
	var regData models.UserRegistration
	if err := c.ShouldBind(&regData); err != nil {
		return registrationError(err)
	}

	// Check if user already exists
	existingUser, _ := h.BaseHandler.DB.GetUserByUsername(regData.Username)
	if existingUser != nil {
		return &HTTPError{
			Status:     http.StatusBadRequest,
			TitleKey:   "auth.username_exists",
			MessageKey: "auth.username_exists_message",
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(regData.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user := &models.User{
		Username:     regData.Username,
		PasswordHash: string(hashedPassword),
	}
	if err := h.BaseHandler.DB.CreateUser(user); err != nil {
		return err
	}

	token, err := h.BaseHandler.AS.GenerateToken(user)
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	h.BaseHandler.AS.SetCookie(token, c)
	c.HTML(http.StatusCreated, utilities.Templates.Responses.RegisterSuccess, gin.H{
		"User": user,
		"Lang": currentLang(c),
	})
	return nil
}

func (h *AuthHandler) GetLogin(c *gin.Context) error {
	renderPage(c, utilities.Templates.Pages.Login, gin.H{"Lang": currentLang(c)})
	return nil
}

// Login verifies the credentials, sets the auth cookie and redirects home.
func (h *AuthHandler) Login(c *gin.Context) error {
	var loginData models.UserLogin
	if err := c.ShouldBind(&loginData); err != nil {
		return badRequest("auth.login_required", err)
	}

	user, err := h.BaseHandler.DB.GetUserByUsername(loginData.Username)
	if err != nil {
		return invalidCredentials(err)
	}
	if user == nil {
		return invalidCredentials(errors.New("user not found"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(loginData.Password)); err != nil {
		return invalidCredentials(err)
	}

	token, err := h.BaseHandler.AS.GenerateToken(user)
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	h.BaseHandler.AS.SetCookie(token, c)
	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
	return nil
}

func (h *AuthHandler) Logout(c *gin.Context) error {
	h.BaseHandler.AS.ClearCookie(c)

	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
	return nil
}
